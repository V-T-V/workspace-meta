package parser

import (
	"context"
	"strings"
)

// TextParser 处理 .txt 纯文本。
type TextParser struct{}

func (p *TextParser) Supports(ext string) bool { return ext == ".txt" }

func (p *TextParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	data, err := readFile(filePath)
	if err != nil {
		return nil, err
	}
	// 按空行分段，每段一个 paragraph block
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	paragraphs := strings.Split(text, "\n\n")
	doc := &ParsedDocument{}
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// 检测简单标题：单行且较短
		if !strings.Contains(para, "\n") && len([]rune(para)) < 40 {
			doc.Blocks = append(doc.Blocks, Block{Type: "heading", Level: 1, Title: para})
		} else {
			doc.Blocks = append(doc.Blocks, Block{Type: "paragraph", Content: strings.ReplaceAll(para, "\n", " ")})
		}
	}
	return doc, nil
}
