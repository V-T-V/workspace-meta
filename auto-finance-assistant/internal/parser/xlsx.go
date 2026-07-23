package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XLSXParser 处理 .xlsx（ZIP 包，遍历工作表 + 共享字符串）。
// 纯标准库实现。对应原计划 14.4：每行转为自描述文本。
type XLSXParser struct{}

func (p *XLSXParser) Supports(ext string) bool { return ext == ".xlsx" }

func (p *XLSXParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("[parser] 打开 xlsx 失败: %w", err)
	}
	defer zr.Close()

	// 1. 加载共享字符串表
	sharedStrings, err := loadSharedStrings(&zr.Reader)
	if err != nil {
		return nil, err
	}

	// 2. 读取 workbook.xml 获取工作表名 → 文件路径映射
	sheetFiles, err := loadSheetNames(&zr.Reader)
	if err != nil {
		return nil, err
	}

	doc := &ParsedDocument{}
	// 3. 逐表解析
	for _, sf := range sheetFiles {
		rows, err := parseSheet(&zr.Reader, sf.Path, sharedStrings)
		if err != nil {
			return nil, fmt.Errorf("[parser] 解析工作表 %s 失败: %w", sf.Name, err)
		}
		if len(rows) == 0 {
			continue
		}
		doc.Blocks = append(doc.Blocks, Block{
			Type:  "heading",
			Level: 2,
			Title: "工作表：" + sf.Name,
		})
		// 第一行作表头
		header := rows[0]
		for i := 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) == 0 {
				continue
			}
			content := describeRow(header, row, i+1)
			if strings.TrimSpace(content) != "" {
				doc.Blocks = append(doc.Blocks, Block{
					Type:    "table_row",
					Content: content,
					Section: sf.Name,
				})
			}
		}
	}
	return doc, nil
}

// describeRow 把一行数据转为自描述文本。
// 例："第8行：产品名称为'新车金融A'，最低首付比例为'20%'"
func describeRow(header, row []string, rowNum int) string {
	var parts []string
	for i, val := range row {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		key := ""
		if i < len(header) {
			key = strings.TrimSpace(header[i])
		}
		if key != "" {
			parts = append(parts, fmt.Sprintf("%s为\"%s\"", key, val))
		} else {
			parts = append(parts, val)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("第%d行：%s。", rowNum, strings.Join(parts, "，"))
}

// --- XLSX 内部解析 ---

// loadSharedStrings 解析 xl/sharedStrings.xml。
func loadSharedStrings(zr *zip.Reader) ([]string, error) {
	var f *zip.File
	for _, file := range zr.File {
		if file.Name == "xl/sharedStrings.xml" {
			f = file
			break
		}
	}
	if f == nil {
		return nil, nil // 无共享字符串
	}
	rc, err := safeZipEntryReader(f)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var strs []string
	dec := xml.NewDecoder(rc)
	var inSI, inT bool
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "si" {
				inSI = true
				buf.Reset()
			} else if el.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			if el.Name.Local == "si" {
				inSI = false
				strs = append(strs, buf.String())
				if len(strs) > MaxSharedStrings {
					return strs, nil // 超上限截断，防内存爆炸
				}
			} else if el.Name.Local == "t" {
				inT = false
			}
		case xml.CharData:
			if inSI && inT {
				buf.Write(el)
			}
		}
	}
	return strs, nil
}

// sheetFile 工作表名与文件路径。
type sheetFile struct {
	Name string
	Path string
}

// loadSheetNames 从 workbook.xml + workbook.xml.rels 解析工作表顺序。
func loadSheetNames(zr *zip.Reader) ([]sheetFile, error) {
	// 简化：直接找 xl/worksheets/sheetN.xml，按编号排序
	var files []sheetFile
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			files = append(files, sheetFile{
				Name: strings.TrimSuffix(strings.TrimPrefix(f.Name, "xl/worksheets/"), ".xml"),
				Path: f.Name,
			})
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("[parser] xlsx 内未找到工作表")
	}
	return files, nil
}

// parseSheet 解析单个工作表，返回二维数组。
func parseSheet(zr *zip.Reader, path string, shared []string) ([][]string, error) {
	var f *zip.File
	for _, file := range zr.File {
		if file.Name == path {
			f = file
			break
		}
	}
	if f == nil {
		return nil, fmt.Errorf("工作表文件 %s 不存在", path)
	}
	rc, err := safeZipEntryReader(f)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var rows [][]string
	dec := xml.NewDecoder(rc)
	var inRow, inCell, inV bool
	var currentRow []string
	var cellType string
	var valBuf strings.Builder

	flushCell := func() {
		val := strings.TrimSpace(valBuf.String())
		if cellType == "s" && val != "" {
			// 共享字符串索引
			var idx int
			if _, err := fmt.Sscanf(val, "%d", &idx); err == nil && idx < len(shared) {
				val = shared[idx]
			}
		}
		currentRow = append(currentRow, val)
		valBuf.Reset()
		cellType = ""
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "row":
				inRow = true
				currentRow = nil
			case "c":
				inCell = true
				for _, attr := range el.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
			case "v":
				inV = true
			}
		case xml.EndElement:
			switch el.Name.Local {
			case "row":
				if inRow {
					rows = append(rows, currentRow)
					inRow = false
					if len(rows) > MaxSheetRows {
						return rows, nil // 超上限截断
					}
				}
			case "c":
				if inCell {
					flushCell()
					inCell = false
				}
			case "v":
				inV = false
			}
		case xml.CharData:
			if inV && inCell {
				valBuf.Write(el)
			}
		}
	}
	return rows, nil
}
