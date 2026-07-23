package parser

import (
	"context"
	"regexp"
	"strings"
)

// HTMLParser 处理 .html，剥离 script/style，提取标题/段落/列表。
type HTMLParser struct{}

func (p *HTMLParser) Supports(ext string) bool { return ext == ".html" || ext == ".htm" }

var (
	htmlScriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStyleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	htmlEntityRe = regexp.MustCompile(`&[a-zA-Z#0-9]+;`)
	htmlHeadingRe = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	htmlParaRe    = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	htmlLiRe      = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
)

func (p *HTMLParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	data, err := readFile(filePath)
	if err != nil {
		return nil, err
	}
	html := string(data)
	// 剥离 script/style
	html = htmlScriptRe.ReplaceAllString(html, "")
	html = htmlStyleRe.ReplaceAllString(html, "")

	doc := &ParsedDocument{}

	// 提取标题
	for _, m := range htmlHeadingRe.FindAllStringSubmatch(html, -1) {
		level := int(m[1][0] - '0')
		title := cleanHTMLText(m[2])
		if title != "" {
			doc.Blocks = append(doc.Blocks, Block{Type: "heading", Level: level, Title: title})
		}
	}
	// 提取段落
	for _, m := range htmlParaRe.FindAllStringSubmatch(html, -1) {
		content := cleanHTMLText(m[1])
		if content != "" {
			doc.Blocks = append(doc.Blocks, Block{Type: "paragraph", Content: content})
		}
	}
	// 提取列表项
	for _, m := range htmlLiRe.FindAllStringSubmatch(html, -1) {
		content := cleanHTMLText(m[1])
		if content != "" {
			doc.Blocks = append(doc.Blocks, Block{Type: "list_item", Content: content})
		}
	}
	return doc, nil
}

// cleanHTMLText 去标签 + 解码常见实体 + 压缩空白。
func cleanHTMLText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	// 常见实体解码
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = htmlEntityRe.ReplaceAllString(s, "")
	// 压缩空白
	return strings.Join(strings.Fields(s), " ")
}
