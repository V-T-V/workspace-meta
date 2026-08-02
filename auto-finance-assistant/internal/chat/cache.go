package chat

import (
	"container/list"
	"sync"
	"time"
)

// answerCache 是一个带 TTL 的 LRU 缓存，用于命中常见问题，跳过模型调用。
// 命中时 <1ms 返回（vs 模型 5-15s），对高频客服场景提速巨大。
type answerCache struct {
	mu    sync.Mutex
	items map[string]*list.Element
	ll    *list.List
	cap   int
	ttl   time.Duration
}

type cacheEntry struct {
	key       string
	answer    string
	intent    string
	expiresAt time.Time
}

func newAnswerCache(cap int, ttl time.Duration) *answerCache {
	return &answerCache{
		items: make(map[string]*list.Element, cap),
		ll:    list.New(),
		cap:   cap,
		ttl:   ttl,
	}
}

// normalizeKey 复用 FAQ 匹配器的 Normalize，确保标点/全角/空格变体归一。
// 例如 "新车首付多少？" 和 "新车首付多少" 命中同一缓存条目。
func normalizeKey(q string) string {
	return Normalize(q)
}

// get 返回缓存中的答案，若过期或不存在返回 ""。
func (c *answerCache) get(q string) (answer, intent string, ok bool) {
	key := normalizeKey(q)
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, exists := c.items[key]; exists {
		entry := el.Value.(*cacheEntry)
		if time.Now().After(entry.expiresAt) {
			c.ll.Remove(el)
			delete(c.items, key)
			return "", "", false
		}
		// 命中：移到队首（LRU）
		c.ll.MoveToFront(el)
		return entry.answer, entry.intent, true
	}
	return "", "", false
}

// set 写入缓存。超过容量时淘汰最久未用的。
func (c *answerCache) set(q, answer, intent string) {
	key := normalizeKey(q)
	c.mu.Lock()
	defer c.mu.Unlock()

	// 已存在 → 更新
	if el, exists := c.items[key]; exists {
		entry := el.Value.(*cacheEntry)
		entry.answer = answer
		entry.intent = intent
		entry.expiresAt = time.Now().Add(c.ttl)
		c.ll.MoveToFront(el)
		return
	}

	// 新增
	entry := &cacheEntry{
		key:       key,
		answer:    answer,
		intent:    intent,
		expiresAt: time.Now().Add(c.ttl),
	}
	el := c.ll.PushFront(entry)
	c.items[key] = el

	// 淘汰
	for c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}
}

// size 返回当前缓存条目数（供指标展示）。
func (c *answerCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
