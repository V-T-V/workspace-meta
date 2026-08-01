package alignment

import (
	"testing"

	"github.com/QiuShichang/ai-safety-atlas/internal/detector"
)

func TestEvaluateRuns(t *testing.T) {
	det := detector.New()
	m := Evaluate(det)
	if m.Total == 0 {
		t.Fatal("应有用例")
	}
	t.Log("\n" + Format(m))
}

func TestPrecisionNoFalsePositives(t *testing.T) {
	// 良性用例不应被误标（precision 应高）
	det := detector.New()
	m := Evaluate(det)
	if m.FalsePos > 0 {
		t.Errorf("误报 %d 个良性用例（应 0）", m.FalsePos)
	}
	if m.Precision < 0.99 {
		t.Errorf("精确率 %.3f 过低（应 ~1.0）", m.Precision)
	}
}

func TestRecallThreshold(t *testing.T) {
	det := detector.New()
	m := Evaluate(det)
	if m.Recall < 0.6 {
		t.Errorf("召回率 %.3f 过低（规则检测器应 >= 0.6）", m.Recall)
	}
}

func TestFormatOutput(t *testing.T) {
	det := detector.New()
	m := Evaluate(det)
	out := Format(m)
	if out == "" {
		t.Error("Format 不应为空")
	}
}

func TestByCategory(t *testing.T) {
	det := detector.New()
	m := Evaluate(det)
	if len(m.ByCategory) == 0 {
		t.Error("应有类别细分")
	}
	// role_override 应有检测
	if cm, ok := m.ByCategory["role_override"]; ok {
		if cm.Total == 0 {
			t.Error("role_override 应有用例")
		}
	}
}
