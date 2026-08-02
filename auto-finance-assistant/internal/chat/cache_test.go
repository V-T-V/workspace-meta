package chat

import (
	"testing"
	"time"
)

func TestAnswerCacheBasic(t *testing.T) {
	c := newAnswerCache(3, 1*time.Minute)

	// 不存在
	if _, _, ok := c.get("hello"); ok {
		t.Error("expected miss on empty cache")
	}

	// 写入 + 命中
	c.set("hello", "你好，我是客服", "guard_shortcut")
	ans, intent, ok := c.get("hello")
	if !ok {
		t.Fatal("expected hit after set")
	}
	if ans != "你好，我是客服" || intent != "guard_shortcut" {
		t.Errorf("got ans=%q intent=%q", ans, intent)
	}
}

func TestAnswerCacheLRUEviction(t *testing.T) {
	c := newAnswerCache(2, 1*time.Minute)
	c.set("a", "ans-a", "model")
	c.set("b", "ans-b", "model")
	c.set("c", "ans-c", "model") // 应淘汰 a（最久未用）

	if _, _, ok := c.get("a"); ok {
		t.Error("a should have been evicted")
	}
	if _, _, ok := c.get("b"); !ok {
		t.Error("b should still exist")
	}
	if _, _, ok := c.get("c"); !ok {
		t.Error("c should still exist")
	}
}

func TestAnswerCacheTTLExpiry(t *testing.T) {
	c := newAnswerCache(10, 50*time.Millisecond)
	c.set("temp", "temporary", "model")

	// 立即命中
	if _, _, ok := c.get("temp"); !ok {
		t.Fatal("expected hit before expiry")
	}

	time.Sleep(60 * time.Millisecond)
	// 过期后应 miss
	if _, _, ok := c.get("temp"); ok {
		t.Error("expected miss after TTL expiry")
	}
}

func TestAnswerCacheNormalizeKey(t *testing.T) {
	c := newAnswerCache(10, 1*time.Minute)
	c.set("  Hello  ", "ans", "model")

	// 不同大小写 + 首尾空格应命中
	if _, _, ok := c.get("hello"); !ok {
		t.Error("case-insensitive + trim match expected")
	}
	if _, _, ok := c.get("  HELLO "); !ok {
		t.Error("trim + uppercase match expected")
	}
}

func TestAnswerCacheUpdateExisting(t *testing.T) {
	c := newAnswerCache(10, 1*time.Minute)
	c.set("q", "old-answer", "model")
	c.set("q", "new-answer", "model")

	ans, _, ok := c.get("q")
	if !ok || ans != "new-answer" {
		t.Errorf("expected new-answer, got %q ok=%v", ans, ok)
	}
}
