package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DOCXParser 处理 .docx（ZIP 包，解析 word/document.xml）。
// 纯标准库实现，无外部依赖。对应原计划 14.3。
type DOCXParser struct{}

func (p *DOCXParser) Supports(ext string) bool { return ext == ".docx" }

func (p *DOCXParser) Parse(ctx context.Context, filePath string) (*ParsedDocument, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("[parser] 打开 docx 失败（非有效 ZIP）: %w", err)
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := safeZipEntryReader(f)
			if err != nil {
				return nil, fmt.Errorf("[parser] 打开 document.xml 失败: %w", err)
			}
			docXML = rc
			break
		}
	}
	if docXML == nil {
		return nil, fmt.Errorf("[parser] docx 内未找到 word/document.xml")
	}
	defer docXML.Close()

	return parseDocxXML(docXML)
}

// docxToken 是 XML token 的简化模型。
type docxToken struct {
	// paragraph 当前段落的文本与样式
}

// parseDocxXML 流式解析 OOXML document.xml。
// 关注 <w:p>（段落）内的 <w:t>（文本）与 <w:pStyle>（样式，识别标题）。
func parseDocxXML(r io.Reader) (*ParsedDocument, error) {
	dec := xml.NewDecoder(r)
	doc := &ParsedDocument{}

	var inParagraph bool
	var inText bool
	var paraStyle string
	var paraText strings.Builder
	var section []string

	flushPara := func() {
		text := strings.TrimSpace(paraText.String())
		if text == "" {
			return
		}
		// Heading 样式：Heading1~6
		level := headingLevel(paraStyle)
		if level > 0 {
			for len(section) >= level {
				section = section[:len(section)-1]
			}
			section = append(section, text)
			doc.Blocks = append(doc.Blocks, Block{
				Type: "heading", Level: level, Title: text, Section: joinSection(section),
			})
		} else {
			doc.Blocks = append(doc.Blocks, Block{
				Type: "paragraph", Content: text, Section: joinSection(section),
			})
		}
		paraText.Reset()
		paraStyle = ""
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("[parser] 解析 docx XML 失败: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "p":
				if el.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
					inParagraph = true
					paraText.Reset()
					paraStyle = ""
				}
			case "pStyle":
				for _, attr := range el.Attr {
					if attr.Name.Local == "val" {
						paraStyle = attr.Value
					}
				}
			case "t":
				inText = true
			case "br", "tab":
				if inParagraph {
					paraText.WriteString(" ")
				}
			}
		case xml.EndElement:
			switch el.Name.Local {
			case "p":
				if inParagraph {
					flushPara()
					inParagraph = false
				}
			case "t":
				inText = false
			}
		case xml.CharData:
			if inText && inParagraph {
				paraText.Write(el)
			}
		}
	}
	return doc, nil
}

// headingLevel 从 OOXML 样式名识别标题层级。
func headingLevel(style string) int {
	style = strings.ToLower(style)
	if strings.HasPrefix(style, "heading") {
		var n int
		fmt.Sscanf(style, "heading%d", &n)
		if n >= 1 && n <= 6 {
			return n
		}
	}
	if strings.HasPrefix(style, "标题") {
		var n int
		fmt.Sscanf(style, "标题%d", &n)
		if n >= 1 && n <= 6 {
			return n
		}
	}
	// HeadingN（数字直接在后面）
	for i := 1; i <= 6; i++ {
		if style == fmt.Sprintf("heading%d", i) {
			return i
		}
	}
	return 0
}
