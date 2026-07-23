// Package knowledge 实现文档导入管线：解析 → 清洗 → 分块 → 发布。
// 对应原计划第十四节（分块）与文档导入链路。
package knowledge

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/parser"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// Chunker 把 ParsedDocument 切分为知识片段。
// 对应原计划 14.6：按标题/段落/列表优先，最小 300 字，最大 800 字，重叠 80 字。
type Chunker struct {
	MinChars    int
	MaxChars    int
	OverlapChars int
}

// NewChunker 用配置构造。
func NewChunker(min, max, overlap int) *Chunker {
	if min <= 0 {
		min = 300
	}
	if max <= 0 {
		max = 800
	}
	if overlap < 0 {
		overlap = 80
	}
	// 防 splitLong 死循环：overlap 必须 < maxChars/2
	if overlap >= max/2 {
		overlap = max / 4
	}
	return &Chunker{MinChars: min, MaxChars: max, OverlapChars: overlap}
}

// Chunk 把解析结果切成 storage.Chunk。
// 策略：尽量保持标题段落完整；超长段落按句号切分；短段落累积到 minChars。
func (c *Chunker) Chunk(docID string, parsed *parser.ParsedDocument) []*storage.Chunk {
	var chunks []*storage.Chunk
	seq := 0

	// 累积缓冲：合并连续短段落，达到 minChars 或遇到标题时 flush
	var bufTitle, bufSection string
	var bufContent strings.Builder
	var bufPage int

	flushBuffer := func() {
		text := strings.TrimSpace(bufContent.String())
		if text == "" {
			bufContent.Reset()
			return
		}
		// 超长则再切分
		if runeLen(text) > c.MaxChars {
			for _, piece := range splitLong(text, c.MaxChars, c.OverlapChars) {
				seq++
				chunks = append(chunks, &storage.Chunk{
					ID:         uuid.NewString(),
					DocumentID: docID,
					Sequence:   seq,
					Title:      bufTitle,
					Section:    bufSection,
					Content:    piece,
					PageNumber: bufPage,
					TokenCount: estimateTokens(piece),
					Metadata:   "{}",
				})
			}
		} else {
			seq++
			chunks = append(chunks, &storage.Chunk{
				ID:         uuid.NewString(),
				DocumentID: docID,
				Sequence:   seq,
				Title:      bufTitle,
				Section:    bufSection,
				Content:    text,
				PageNumber: bufPage,
				TokenCount: estimateTokens(text),
				Metadata:   "{}",
			})
		}
		bufContent.Reset()
	}

	for _, block := range parsed.Blocks {
		switch block.Type {
		case "heading":
			// 标题前 flush 缓冲
			flushBuffer()
			bufTitle = block.Title
			bufSection = block.Section
			if bufSection == "" {
				bufSection = block.Title
			}
		case "paragraph", "list_item", "table_row":
			content := strings.TrimSpace(block.Content)
			if content == "" {
				continue
			}
			// 单段已超 max：单独成块
			if runeLen(content) >= c.MaxChars {
				flushBuffer()
				for _, piece := range splitLong(content, c.MaxChars, c.OverlapChars) {
					seq++
					chunks = append(chunks, &storage.Chunk{
						ID:         uuid.NewString(),
						DocumentID: docID,
						Sequence:   seq,
						Title:      bufTitle,
						Section:    bufSection,
						Content:    piece,
						PageNumber: block.Page,
						TokenCount: estimateTokens(piece),
						Metadata:   "{}",
					})
				}
				continue
			}
			// 累积到缓冲
			if bufContent.Len() > 0 {
				bufContent.WriteString("\n")
			}
			bufContent.WriteString(content)
			if block.Page > 0 {
				bufPage = block.Page
			}
			// 达到 min 即 flush
			if runeLen(bufContent.String()) >= c.MinChars {
				flushBuffer()
			}
		}
	}
	flushBuffer()
	return chunks
}

// runeLen 返回 rune 数（中文按字计）。
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// splitLong 把超长文本按 maxChars 切分，带 overlap 重叠。
// 优先在句号/换行处切。
func splitLong(text string, maxChars, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return []string{text}
	}
	var pieces []string
	start := 0
	for start < len(runes) {
		end := start + maxChars
		if end >= len(runes) {
			pieces = append(pieces, string(runes[start:]))
			break
		}
		// 在 [start+maxChars/2, end] 范围找最近的句号/换行
		cutAt := findCutPoint(runes, start+maxChars/2, end)
		pieces = append(pieces, string(runes[start:cutAt]))
		start = cutAt - overlap
		if start < 0 {
			start = 0
		}
	}
	return pieces
}

// findCutPoint 在 [from, to] 找最佳切分点（句号优先，其次换行，其次空格）。
func findCutPoint(runes []rune, from, to int) int {
	if to > len(runes) {
		to = len(runes)
	}
	// 优先找句号类
	for i := to - 1; i >= from; i-- {
		r := runes[i]
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' {
			return i + 1
		}
	}
	// 其次换行
	for i := to - 1; i >= from; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	// 其次空格
	for i := to - 1; i >= from; i-- {
		if runes[i] == ' ' {
			return i + 1
		}
	}
	return to
}

// estimateTokens 粗估 token 数（中文≈字数，英文≈词数×1.3）。
func estimateTokens(text string) int {
	var cjk, other int
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk++
		} else if r > ' ' {
			other++
		}
	}
	return cjk + other/4
}
