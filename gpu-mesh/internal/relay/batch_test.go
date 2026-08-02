package relay

import "testing"

func TestShardItems(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g"}

	// 正常分片
	got := shardItems(items, 3)
	if len(got) != 3 {
		t.Fatalf("7 项按 3 分片应得 3 片，得到 %d", len(got))
	}
	if len(got[0]) != 3 || len(got[1]) != 3 || len(got[2]) != 1 {
		t.Errorf("分片大小应为 3,3,1，得到 %d,%d,%d", len(got[0]), len(got[1]), len(got[2]))
	}
	// 顺序保持
	flat := []string{}
	for _, s := range got {
		flat = append(flat, s...)
	}
	for i, v := range flat {
		if v != items[i] {
			t.Errorf("顺序错乱 idx=%d: 期望 %s 得到 %s", i, items[i], v)
		}
	}
}

func TestShardItems_ExactDivision(t *testing.T) {
	got := shardItems([]string{"a", "b", "c", "d"}, 2)
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 2 {
		t.Errorf("4 项按 2 分片应得 2x2，得到 %v", got)
	}
}

func TestShardItems_SizeLargerThanInput(t *testing.T) {
	// 分片大小 > 输入数量
	got := shardItems([]string{"a", "b"}, 10)
	if len(got) != 1 || len(got[0]) != 2 {
		t.Errorf("应得 1 片含 2 项，得到 %v", got)
	}
}

func TestShardItems_DefaultSize(t *testing.T) {
	// size<=0 用默认 20
	got := shardItems(make([]string, 45), 0)
	if len(got) != 3 {
		t.Errorf("45 项按默认 20 分片应得 3 片，得到 %d", len(got))
	}
}

func TestShardItems_Empty(t *testing.T) {
	got := shardItems([]string{}, 10)
	if len(got) != 0 {
		t.Errorf("空输入应得 0 片，得到 %d", len(got))
	}
}
