package rag

import (
	"fmt"
	"strings"
)

// BuildContext 把检索到的片段构造为模型上下文文本。
// 对应原计划 11.8：最多 5 个片段，每片标注来源元数据。
func BuildContext(results []SearchResult, maxChunks int) string {
	if len(results) == 0 {
		return ""
	}
	if maxChunks <= 0 {
		maxChunks = 5
	}
	if len(results) > maxChunks {
		results = results[:maxChunks]
	}
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "[资料%d]\n", i+1)
		if r.DocumentName != "" {
			fmt.Fprintf(&b, "文档：%s\n", r.DocumentName)
		}
		if r.Version != "" {
			fmt.Fprintf(&b, "版本：%s\n", r.Version)
		}
		if r.Section != "" {
			fmt.Fprintf(&b, "章节：%s\n", r.Section)
		}
		if r.EffectiveDate != "" {
			fmt.Fprintf(&b, "生效日期：%s\n", r.EffectiveDate)
		}
		b.WriteString("正文：\n")
		b.WriteString(r.Content)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}
