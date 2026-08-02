package password

import (
	"context"
	"strings"
	"testing"
)

func TestWeakCommonPassword(t *testing.T) {
	// 命中黑名单的口令：分数必须为 0、级别 weak、IsCommon=true
	for _, pw := range []string{"123456", "password", "qwerty", "abc123"} {
		r := CheckStrength(pw)
		if !r.IsCommon {
			t.Errorf("%q 应被识别为常见弱口令", pw)
		}
		if r.Score != 0 {
			t.Errorf("%q 命中黑名单后分数应为 0，实际 %d", pw, r.Score)
		}
		if r.Level != LevelWeak {
			t.Errorf("%q 级别应为 weak，实际 %s", pw, r.Level)
		}
	}
}

func TestStrongPassword(t *testing.T) {
	// 长度 + 四类齐全 → strong，分数 >= 80
	pw := "xK9#mQ$vL2@pN7!"
	r := CheckStrength(pw)
	if r.Level != LevelStrong {
		t.Errorf("%q 应为 strong，实际 %s（分数 %d）", pw, r.Level, r.Score)
	}
	if r.Score < 80 {
		t.Errorf("%q 分数应 >= 80，实际 %d", pw, r.Score)
	}
	if !(r.HasLower && r.HasUpper && r.HasDigit && r.HasSpecial) {
		t.Errorf("%q 应同时含大小写/数字/特殊字符", pw)
	}
}

func TestMediumPassword(t *testing.T) {
	// 仅小写+数字、长度中等，既不命中黑名单也不够四类 → medium（或至少非 strong）
	// 注意分数取决于具体长度，这里只验证落在 weak/medium 而非 strong，
	// 且不命中黑名单。
	pw := "hello2024"
	r := CheckStrength(pw)
	if r.IsCommon {
		t.Errorf("%q 不应在黑名单中", pw)
	}
	if r.Level == LevelStrong {
		t.Errorf("%q 缺大写/特殊，不应为 strong（分数 %d）", pw, r.Score)
	}
	if r.Score < 40 || r.Score >= 80 {
		t.Errorf("%q 期望 medium 区间 [40,80)，实际分数 %d", pw, r.Score)
	}
}

func TestScoreBoundsAndLevelMapping(t *testing.T) {
	// 任何口令分数都应在 [0,100]
	for _, pw := range []string{"", "a", "123456", "abcdef", "Abc123!@#XyZ9"} {
		r := CheckStrength(pw)
		if r.Score < 0 || r.Score > 100 {
			t.Errorf("%q 分数 %d 越界 [0,100]", pw, r.Score)
		}
		// 级别与分数一致性（黑名单例外：强制 weak）
		if r.IsCommon {
			if r.Level != LevelWeak {
				t.Errorf("%q 命中黑名单级别应为 weak", pw)
			}
			continue
		}
		want := levelFromScore(r.Score)
		if r.Level != want {
			t.Errorf("%q 级别 %s 与分数 %d 不一致（期望 %s）", pw, r.Level, r.Score, want)
		}
	}
}

func TestEmptyPassword(t *testing.T) {
	// 空口令：长度 0、分数 0、weak
	r := CheckStrength("")
	if r.Length != 0 {
		t.Errorf("空口令长度应为 0，实际 %d", r.Length)
	}
	if r.Score != 0 {
		t.Errorf("空口令分数应为 0，实际 %d", r.Score)
	}
	if r.Level != LevelWeak {
		t.Errorf("空口令级别应为 weak，实际 %s", r.Level)
	}
}

func TestRuneLengthForUnicode(t *testing.T) {
	// 非 ASCII 字符按 rune 计数，3 个中文应记长度 3
	r := CheckStrength("密码啊")
	if r.Length != 3 {
		t.Errorf("中文口令长度应为 3（按 rune），实际 %d", r.Length)
	}
}

func TestSpecialCharDetection(t *testing.T) {
	// 空格、标点、符号都算特殊字符
	for _, pw := range []string{"ab cd", "a!b@c", "a-b_c"} {
		r := CheckStrength(pw)
		if !r.HasSpecial {
			t.Errorf("%q 应被识别为含特殊字符", pw)
		}
	}
}

func TestReasonsExplainScore(t *testing.T) {
	// Reasons 应解释得分依据：命中黑名单时应有相应说明
	r := CheckStrength("123456")
	found := false
	for _, reason := range r.Reasons {
		if strings.Contains(reason, "黑名单") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("命中黑名单的口令应在 Reasons 中说明，实际 %v", r.Reasons)
	}

	// 非黑名单口令的 Reasons 至少应提到长度
	r2 := CheckStrength("Abc123!@#")
	gotLenReason := false
	for _, reason := range r2.Reasons {
		if strings.Contains(reason, "长度") {
			gotLenReason = true
			break
		}
	}
	if !gotLenReason {
		t.Errorf("非黑名单口令应在 Reasons 中提到长度，实际 %v", r2.Reasons)
	}
}

func TestLevelFromScoreThresholds(t *testing.T) {
	// 阈值边界：< 40 weak，40..79 medium，>= 80 strong
	cases := []struct {
		score int
		level Level
	}{
		{0, LevelWeak}, {39, LevelWeak},
		{40, LevelMedium}, {79, LevelMedium},
		{80, LevelStrong}, {100, LevelStrong},
	}
	for _, c := range cases {
		if got := levelFromScore(c.score); got != c.level {
			t.Errorf("levelFromScore(%d) = %s，期望 %s", c.score, got, c.level)
		}
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) < 3 {
		t.Fatal("demo 应至少演示 3 组口令")
	}
	// demo 应同时包含 weak（黑名单）和 strong 样本
	var hasWeak, hasStrong bool
	for _, it := range r.Items {
		if it.Level == LevelWeak {
			hasWeak = true
		}
		if it.Level == LevelStrong {
			hasStrong = true
		}
	}
	if !hasWeak {
		t.Error("demo 应包含 weak 样本")
	}
	if !hasStrong {
		t.Error("demo 应包含 strong 样本")
	}
}
