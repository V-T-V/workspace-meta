// Package parser 实现多格式文档文本提取。
// 对应原计划第十四节。M3 支持 TXT/MD/HTML/DOCX/XLSX/文本PDF。
// DocumentParser 接口供 knowledge.Importer 按 extension 路由。
package parser

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ParsedDocument 是解析结果：结构化段落序列，保留标题层级与页码。
type ParsedDocument struct {
	Blocks []Block // 按文档顺序
}

// Block 是文档的一个结构单元。
type Block struct {
	Type    string // heading | paragraph | list_item | table_row
	Level   int    // heading 层级 1-6
	Title   string // heading 文本（分块时作为标题）
	Section string // 当前所属章节路径（如 "4.2 申请材料"）
	Content string // 正文
	Page    int    // 页码（PDF 有；其他 0）
}

// DocumentParser 按扩展名提取文本为结构化 Block。
// 对应原计划 25.2 接口预留。
type DocumentParser interface {
	Supports(ext string) bool
	Parse(ctx context.Context, filePath string) (*ParsedDocument, error)
}

// Registry 按 extension 分派解析器。
type Registry struct {
	parsers []DocumentParser
}

// NewRegistry 注册全部内置解析器。
func NewRegistry() *Registry {
	return &Registry{parsers: []DocumentParser{
		&TextParser{}, &MarkdownParser{}, &HTMLParser{},
		&DOCXParser{}, &XLSXParser{}, &PDFParser{},
	}}
}

// Parse 按文件扩展名选择解析器并解析。
func (r *Registry) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, p := range r.parsers {
		if p.Supports(ext) {
			return p.Parse(ctx, filePath)
		}
	}
	return nil, fmt.Errorf("[parser] 不支持的格式 %s（支持 .txt/.md/.html/.docx/.xlsx/.pdf）", ext)
}

// SupportsAny 是否有解析器支持某扩展名。
func (r *Registry) SupportsAny(ext string) bool {
	for _, p := range r.parsers {
		if p.Supports(ext) {
			return true
		}
	}
	return false
}

// readFile 读取 UTF-8 文本（处理 BOM）。
func readFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	// 去 UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	return data, nil
}

// 安全限制常量（防 zip-bomb / 实体扩展）。
const (
	// MaxDecompressedSize 单个 ZIP entry 解压上限（50MB）。
	MaxDecompressedSize = 50 * 1024 * 1024
	// MaxTotalDecompressed 单文件解压总上限（200MB）。
	MaxTotalDecompressed = 200 * 1024 * 1024
	// MaxSheetRows XLSX 单表行数上限。
	MaxSheetRows = 100000
	// MaxSharedStrings XLSX 共享字符串上限。
	MaxSharedStrings = 100000
)

// limitedReader 包裹 reader，超过 limit 返回错误（防解压炸弹）。
func limitedReader(r io.Reader, limit int64) io.Reader {
	return io.LimitReader(r, limit)
}

// safeZipEntryReader 打开 ZIP entry 并返回带大小限制的 reader。
// 拒绝未压缩大小异常的 entry。
func safeZipEntryReader(f *zip.File) (io.ReadCloser, error) {
	if f.UncompressedSize64 > MaxDecompressedSize {
		return nil, fmt.Errorf("[parser] ZIP entry %s 解压后过大（%d > %d）", f.Name, f.UncompressedSize64, MaxDecompressedSize)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	return &limitedReadCloser{rc: rc, r: io.LimitReader(rc, MaxDecompressedSize+1)}, nil
}

type limitedReadCloser struct {
	rc io.ReadCloser
	r  io.Reader
}

func (l *limitedReadCloser) Read(p []byte) (int, error) { return l.r.Read(p) }
func (l *limitedReadCloser) Close() error                { return l.rc.Close() }
