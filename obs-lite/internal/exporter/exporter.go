// Package exporter 把 metrics 和 trace 格式化为可读输出（文本 + Prometheus 兼容格式）。
package exporter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/types"
)

// FormatMetricsText 把 registry 的所有 metric 格式化为可读文本。
func FormatMetricsText(reg *metrics.Registry) string {
	var b strings.Builder
	points := reg.AllPoints()
	sortPoints(points) // 确定性输出（按 name+labels 排序）
	for _, p := range points {
		b.WriteString(fmt.Sprintf("%s%s%s %s\n",
			p.Name, formatLabels(p.Labels), formatKindTag(p.Kind), formatValue(p.Value)))
	}
	hists := reg.AllHistograms()
	sortHistograms(hists)
	for _, h := range hists {
		b.WriteString(fmt.Sprintf("%s%s\n", h.Name, formatLabels(h.Labels)))
		for _, bk := range h.Buckets {
			b.WriteString(fmt.Sprintf("  bucket{le=%s} = %d\n", formatBound(bk.UpperBound), bk.Count))
		}
		b.WriteString(fmt.Sprintf("  sum = %s  count = %d\n", formatValue(h.Sum), h.Count))
	}
	return b.String()
}

// FormatTraceText 把 spans 格式化成树形（缩进表示父子关系）。
func FormatTraceText(spans []*types.Span) string {
	if len(spans) == 0 {
		return "(无 span)"
	}
	// 按 TraceID 分组
	byTrace := map[string][]*types.Span{}
	for _, s := range spans {
		byTrace[s.TraceID] = append(byTrace[s.TraceID], s)
	}
	var b strings.Builder
	for traceID, traceSpans := range byTrace {
		b.WriteString(fmt.Sprintf("=== Trace %s (%d spans) ===\n", traceID, len(traceSpans)))
		// 构建父子关系并递归打印
		byParent := map[string][]*types.Span{}
		var roots []*types.Span
		for _, s := range traceSpans {
			if s.ParentID == "" {
				roots = append(roots, s)
			} else {
				byParent[s.ParentID] = append(byParent[s.ParentID], s)
			}
		}
		for _, root := range roots {
			printSpanTree(&b, root, byParent, 0)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func printSpanTree(b *strings.Builder, s *types.Span, byParent map[string][]*types.Span, depth int) {
	indent := strings.Repeat("  ", depth)
	dur := s.Duration()
	b.WriteString(fmt.Sprintf("%s├─ %s [%s] %s", indent, s.Name, s.Status, dur.Round(1000)))
	if len(s.Attributes) > 0 {
		attrs := []string{}
		for k, v := range s.Attributes {
			attrs = append(attrs, k+"="+v)
		}
		b.WriteString(" {" + strings.Join(attrs, ", ") + "}")
	}
	b.WriteString("\n")
	for _, child := range byParent[s.SpanID] {
		printSpanTree(b, child, byParent, depth+1)
	}
}

// FormatMetricsPrometheus 输出 Prometheus 文本格式（可被 prometheus 抓取）。
// 这是简化版（无 HELP/TYPE 头，M2 补全）。
func FormatMetricsPrometheus(reg *metrics.Registry) string {
	var b strings.Builder
	points := reg.AllPoints()
	sortPoints(points) // 确定性输出
	seen := map[string]bool{}
	for _, p := range points {
		if !seen[p.Name] {
			b.WriteString(fmt.Sprintf("# TYPE %s %s\n", p.Name, p.Kind))
			seen[p.Name] = true
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", p.Name, formatLabels(p.Labels), formatValue(p.Value)))
	}
	return b.String()
}

// ===== 辅助 =====

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, k+"=\""+v+"\"")
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func formatKindTag(k types.MetricKind) string {
	switch k {
	case types.MetricCounter:
		return " (counter)"
	case types.MetricGauge:
		return " (gauge)"
	}
	return ""
}

func formatValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}

func formatBound(b float64) string {
	if b > 1e99 {
		return "+Inf"
	}
	// 用 %g 保留原始精度，避免 0.005/0.025 被 %.2f 渲染成同值（导致 le 标签冲突）。
	return fmt.Sprintf("%g", b)
}

// sortPoints 按 name + labels 排序，保证输出确定性（map 迭代无序）。
func sortPoints(points []types.MetricPoint) {
	sort.Slice(points, func(i, j int) bool {
		if points[i].Name != points[j].Name {
			return points[i].Name < points[j].Name
		}
		return labelsKey(points[i].Labels) < labelsKey(points[j].Labels)
	})
}

// sortHistograms 按 name + labels 排序。
func sortHistograms(hists []types.HistogramData) {
	sort.Slice(hists, func(i, j int) bool {
		if hists[i].Name != hists[j].Name {
			return hists[i].Name < hists[j].Name
		}
		return labelsKey(hists[i].Labels) < labelsKey(hists[j].Labels)
	})
}

// labelsKey 内部用（与 metrics 包的 labelsKey 同逻辑，按 key 排序序列化）。
func labelsKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for _, k := range keys {
		out += k + "=" + labels[k] + ","
	}
	return out
}
