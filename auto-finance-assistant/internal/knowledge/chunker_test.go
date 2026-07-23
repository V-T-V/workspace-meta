package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/auto-finance-assistant/internal/parser"
)

// TestChunker_Basic 验证分块基本逻辑。
func TestChunker_Basic(t *testing.T) {
	parsed := &parser.ParsedDocument{Blocks: []parser.Block{
		{Type: "heading", Level: 1, Title: "第一章", Section: "第一章"},
		{Type: "paragraph", Content: "这是第一段内容，介绍产品基本信息。"},
		{Type: "heading", Level: 2, Title: "1.1 申请材料", Section: "第一章 > 1.1 申请材料"},
		{Type: "paragraph", Content: "申请需要身份证、收入证明、居住证明。"},
		{Type: "list_item", Content: "身份证复印件"},
		{Type: "list_item", Content: "收入证明"},
	}}
	c := NewChunker(300, 800, 80)
	chunks := c.Chunk("doc1", parsed)
	if len(chunks) == 0 {
		t.Fatal("应产生至少 1 个 chunk")
	}
	for _, ch := range chunks {
		if ch.DocumentID != "doc1" {
			t.Errorf("documentId 应为 doc1")
		}
		if ch.Content == "" {
			t.Error("chunk content 不应为空")
		}
	}
	// 验证 sequence 连续
	for i, ch := range chunks {
		if ch.Sequence != i+1 {
			t.Errorf("chunk[%d].sequence = %d, want %d", i, ch.Sequence, i+1)
		}
	}
}

// TestChunker_LongSplit 验证超长段落切分。
func TestChunker_LongSplit(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "这是一个比较长的句子。"
	}
	parsed := &parser.ParsedDocument{Blocks: []parser.Block{
		{Type: "paragraph", Content: long},
	}}
	c := NewChunker(100, 300, 50)
	chunks := c.Chunk("doc1", parsed)
	if len(chunks) <= 1 {
		t.Errorf("超长段落应切分为多个 chunk，实际 %d", len(chunks))
	}
}

// TestParser_TXT 验证 TXT 解析。
func TestParser_TXT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "标题行\n\n第一段内容。\n\n第二段内容。"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry()
	doc, err := reg.Parse(nil, path)
	if err != nil {
		t.Fatalf("解析 TXT 失败: %v", err)
	}
	if len(doc.Blocks) == 0 {
		t.Fatal("应解析出 block")
	}
}

// TestParser_MD 验证 Markdown 解析（标题层级）。
func TestParser_MD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := "# 大标题\n\n段落一。\n\n## 小标题\n\n- 列表项一\n- 列表项二\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry()
	doc, err := reg.Parse(nil, path)
	if err != nil {
		t.Fatalf("解析 MD 失败: %v", err)
	}
	headings := 0
	for _, b := range doc.Blocks {
		if b.Type == "heading" {
			headings++
		}
	}
	if headings < 2 {
		t.Errorf("应解析出至少 2 个标题，实际 %d", headings)
	}
}

// TestParser_Unsupported 验证不支持的格式。
func TestParser_Unsupported(t *testing.T) {
	reg := parser.NewRegistry()
	_, err := reg.Parse(nil, "file.xyz")
	if err == nil {
		t.Fatal("不支持的格式应返回错误")
	}
}

// TestSplitLong 验证切分逻辑。
func TestSplitLong(t *testing.T) {
	text := ""
	for i := 0; i < 100; i++ {
		text += "句子。"
	}
	pieces := splitLong(text, 30, 5)
	if len(pieces) < 2 {
		t.Errorf("应切分为多段，实际 %d", len(pieces))
	}
}
