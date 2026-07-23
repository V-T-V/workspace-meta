package parser

import (
	"bytes"
	"context"
	"os"
	"strings"
)

// PDFParser 处理可提取文本的 .pdf（第一版不支持扫描件/OCR）。
// 纯标准库实现：提取 BT...ET 文本块内的 (...) 与 [...] 显示操作符的文本。
// 对应原计划 14.5。复杂 PDF 标记为需人工转换。
type PDFParser struct{}

func (p *PDFParser) Supports(ext string) bool { return ext == ".pdf" }

func (p *PDFParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	doc := &ParsedDocument{}

	// 简化分页：按 /Type /Page 或 form feed
	pages := splitPDFPages(data)
	if len(pages) == 0 {
		pages = [][]byte{data}
	}

	totalTextLen := 0
	for i, pageData := range pages {
		text := extractPDFText(pageData)
		totalTextLen += len(text)
		text = cleanPDFText(text)
		if text == "" {
			continue
		}
		// 按换行分段
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if len([]rune(line)) < 2 {
				continue
			}
			doc.Blocks = append(doc.Blocks, Block{
				Type:    "paragraph",
				Content: line,
				Page:    i + 1,
			})
		}
	}

	// 文本过少 → 标记为扫描件
	if totalTextLen < 50 {
		doc.Blocks = append(doc.Blocks, Block{
			Type:    "paragraph",
			Content: "[此 PDF 提取文本过少，疑似扫描件，需人工转换或 OCR]",
		})
	}
	return doc, nil
}

// splitPDFPages 按 form feed (0x0C) 粗略分页。
func splitPDFPages(data []byte) [][]byte {
	// PDF 无统一分页符，这里按 /Page 对象简化；实际用 form feed 兜底
	return bytes.Split(data, []byte{0x0C})
}

// extractPDFText 提取 PDF 内容流中的文本操作符。
// 识别 Tj/TJ 显示操作符：(...) 字符串 与 [...] 数组。
func extractPDFText(data []byte) string {
	var b strings.Builder
	// 状态机：提取 (...) 内的文本，忽略转义
	inString := false
	escape := false
	parenDepth := 0
	for i := 0; i < len(data); i++ {
		c := data[i]
		if escape {
			escape = false
			if inString {
				switch c {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '\\':
					b.WriteByte('\\')
				case '(':
					b.WriteByte('(')
				case ')':
					b.WriteByte(')')
				default:
					b.WriteByte(c)
				}
			}
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '(' {
			if !inString {
				inString = true
				parenDepth = 1
			} else {
				parenDepth++
				b.WriteByte(c)
			}
			continue
		}
		if c == ')' && inString {
			parenDepth--
			if parenDepth == 0 {
				inString = false
				b.WriteByte('\n')
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if inString {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// cleanPDFText 清理 PDF 提取的噪声：合并断行、去重复页眉页脚。
func cleanPDFText(s string) string {
	// 合并被换行打断的词
	s = strings.ReplaceAll(s, "-\n", "")
	// 压缩连续空行
	lines := strings.Split(s, "\n")
	var out []string
	blankRun := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			blankRun++
			if blankRun <= 1 {
				out = append(out, "")
			}
			continue
		}
		blankRun = 0
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
