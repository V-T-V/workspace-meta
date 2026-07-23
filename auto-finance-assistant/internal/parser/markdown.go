package parser

import (
	"context"
	"regexp"
	"strings"
)

// MarkdownParser 处理 .md，保留标题层级。
type MarkdownParser struct{}

var mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func (p *MarkdownParser) Supports(ext string) bool { return ext == ".md" || ext == ".markdown" }

func (p *MarkdownParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	data, err := readFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	doc := &ParsedDocument{}
	var section []string // 章节路径栈
	var paraBuf strings.Builder

	flushPara := func() {
		if paraBuf.Len() > 0 {
			content := strings.TrimSpace(paraBuf.String())
			if content != "" {
				doc.Blocks = append(doc.Blocks, Block{
					Type:    "paragraph",
					Content: content,
					Section: joinSection(section),
				})
			}
			paraBuf.Reset()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := mdHeadingRe.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			// 更新章节栈：保留 <= level 的层级
			section = section[:min(level-1, len(section))]
			for len(section) < level-1 {
				section = append(section, "")
			}
			if len(section) <= level-1 {
				section = append(section, title)
			} else {
				section[level-1] = title
			}
			doc.Blocks = append(doc.Blocks, Block{
				Type:    "heading",
				Level:   level,
				Title:   title,
				Section: joinSection(section),
			})
		} else if trimmed == "" {
			flushPara()
		} else {
			// 列表项
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				flushPara()
				doc.Blocks = append(doc.Blocks, Block{
					Type:    "list_item",
					Content: strings.TrimSpace(trimmed[2:]),
					Section: joinSection(section),
				})
			} else {
				if paraBuf.Len() > 0 {
					paraBuf.WriteString(" ")
				}
				paraBuf.WriteString(trimmed)
			}
		}
	}
	flushPara()
	return doc, nil
}

func joinSection(parts []string) string {
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, " > ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
