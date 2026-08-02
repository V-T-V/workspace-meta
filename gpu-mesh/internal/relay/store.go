package relay

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// 任务状态常量。
const (
	StatusPending    = "pending"
	StatusDispatched = "dispatched"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// bbolt buckets。
var (
	bucketTasks   = []byte("tasks")
	bucketResults = []byte("results")
)

// StoredTask 持久化的任务记录。
type StoredTask struct {
	Request   proto.TaskRequest `json:"request"`
	Status    string            `json:"status"`
	Attempt   int               `json:"attempt"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
}

// Store bbolt 持久化（任务 + 结果）。
//
// Phase 1 基本占位：任务状态/结果持久化。Phase 3 调度器会用它做 pending 重投。
type Store struct {
	db *bolt.DB
}

// NewStore 打开 bbolt 文件。path 为空时返回错误（避免静默写工作目录）。
func NewStore(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store 路径不能为空")
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketTasks, bucketResults} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭。
func (s *Store) Close() error { return s.db.Close() }

// ListTasksByStatus 按状态筛选任务（重启恢复 + TTL 清理用）。
func (s *Store) ListTasksByStatus(status string) ([]StoredTask, error) {
	var out []StoredTask
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketTasks).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var t StoredTask
			if err := json.Unmarshal(v, &t); err == nil && t.Status == status {
				out = append(out, t)
			}
		}
		return nil
	})
	return out, err
}

// DeleteTask 删除任务记录（TTL 清理用）。
func (s *Store) DeleteTask(taskID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketTasks).Delete([]byte(taskID))
	})
}

// CleanOldTasks 清理超过 maxAge 的终态任务（completed/failed/cancelled）。
// 防止 store.db 无限增长。返回清理的条数。
func (s *Store) CleanOldTasks(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).UnixMilli()
	terminal := map[string]bool{StatusCompleted: true, StatusFailed: true, "cancelled": true}
	deleted := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketTasks)
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var t StoredTask
			if err := json.Unmarshal(v, &t); err != nil {
				continue
			}
			// 只清理终态且超期的
			if terminal[t.Status] && t.UpdatedAt < cutoff {
				bucket.Delete(k)
				deleted++
			}
		}
		return nil
	})
	return deleted, err
}

// SaveTask 持久化任务。
func (s *Store) SaveTask(t StoredTask) error {
	if t.UpdatedAt == 0 {
		t.UpdatedAt = time.Now().UnixMilli()
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketTasks).Put([]byte(t.Request.TaskID), data)
	})
}

// GetTask 读取任务。
func (s *Store) GetTask(taskID string) (*StoredTask, error) {
	var t StoredTask
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketTasks).Get([]byte(taskID))
		if data == nil {
			return fmt.Errorf("任务不存在")
		}
		return json.Unmarshal(data, &t)
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTasks 列出任务（按创建时间最新优先，limit 条）。
//
// 注意：bbolt 的 key 是 taskID（UUID，无时间序），不能靠 cursor 顺序。
// 此处全量读出后按 CreatedAt 倒序排序再截断。百级任务量级可接受；
// 若任务量达到万级以上，应引入单独的时间索引 bucket。
func (s *Store) ListTasks(limit int) ([]StoredTask, error) {
	var all []StoredTask
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketTasks).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var t StoredTask
			if err := json.Unmarshal(v, &t); err == nil {
				all = append(all, t)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 按 CreatedAt 倒序
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt > all[j].CreatedAt
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// SaveResult 持久化任务结果。
func (s *Store) SaveResult(taskID string, result proto.TaskResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		// 同时更新任务状态
		if t, _ := s.GetTask(taskID); t != nil {
			if result.Success {
				t.Status = StatusCompleted
			} else {
				t.Status = StatusFailed
			}
			t.UpdatedAt = time.Now().UnixMilli()
			td, _ := json.Marshal(t)
			_ = tx.Bucket(bucketTasks).Put([]byte(taskID), td)
		}
		return tx.Bucket(bucketResults).Put([]byte(taskID), data)
	})
}

// UpdateTaskStatus 更新任务状态。
func (s *Store) UpdateTaskStatus(taskID, status string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketTasks).Get([]byte(taskID))
		if data == nil {
			return fmt.Errorf("任务不存在")
		}
		var t StoredTask
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		t.Status = status
		t.UpdatedAt = time.Now().UnixMilli()
		nd, err := json.Marshal(t)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketTasks).Put([]byte(taskID), nd)
	})
}
