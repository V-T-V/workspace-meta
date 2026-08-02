package relay

import (
	"testing"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

func TestStore_ListTasksByStatus(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// 存 3 个任务：2 个 pending，1 个 completed
	s.SaveTask(StoredTask{Request: proto.TaskRequest{TaskID: "p1"}, Status: StatusPending, CreatedAt: time.Now().UnixMilli()})
	s.SaveTask(StoredTask{Request: proto.TaskRequest{TaskID: "p2"}, Status: StatusPending, CreatedAt: time.Now().UnixMilli()})
	s.SaveTask(StoredTask{Request: proto.TaskRequest{TaskID: "c1"}, Status: StatusCompleted, CreatedAt: time.Now().UnixMilli()})

	pending, err := s.ListTasksByStatus(StatusPending)
	if err != nil {
		t.Fatalf("ListTasksByStatus 失败: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("期望 2 个 pending，得到 %d", len(pending))
	}

	completed, _ := s.ListTasksByStatus(StatusCompleted)
	if len(completed) != 1 {
		t.Errorf("期望 1 个 completed，得到 %d", len(completed))
	}
}

func TestStore_UpdateTaskStatus(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.SaveTask(StoredTask{Request: proto.TaskRequest{TaskID: "t1"}, Status: StatusPending, CreatedAt: time.Now().UnixMilli()})

	if err := s.UpdateTaskStatus("t1", StatusDispatched); err != nil {
		t.Fatalf("UpdateTaskStatus 失败: %v", err)
	}
	t2, _ := s.GetTask("t1")
	if t2.Status != StatusDispatched {
		t.Errorf("状态应为 dispatched，得到 %s", t2.Status)
	}
}

func TestStore_CleanOldTasks(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// 2 小时前的完成态任务（应被清理，cutoff=1h）
	old := StoredTask{
		Request:   proto.TaskRequest{TaskID: "old"},
		Status:    StatusCompleted,
		CreatedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
		UpdatedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
	}
	s.SaveTask(old)

	// 刚刚的完成态任务（应保留）
	recent := StoredTask{
		Request:   proto.TaskRequest{TaskID: "recent"},
		Status:    StatusCompleted,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	s.SaveTask(recent)

	// 2 小时前的 pending 任务（不应清理——非终态）
	oldPending := StoredTask{
		Request:   proto.TaskRequest{TaskID: "oldpending"},
		Status:    StatusPending,
		CreatedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
		UpdatedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
	}
	s.SaveTask(oldPending)

	n, err := s.CleanOldTasks(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanOldTasks 失败: %v", err)
	}
	if n != 1 {
		t.Errorf("期望清理 1 条，得到 %d", n)
	}
	// old 应被删
	if _, err := s.GetTask("old"); err == nil {
		t.Error("old 应被清理")
	}
	// recent 应保留
	if _, err := s.GetTask("recent"); err != nil {
		t.Error("recent 不应被清理")
	}
	// oldPending 应保留（非终态）
	if _, err := s.GetTask("oldpending"); err != nil {
		t.Error("oldPending 非终态不应被清理")
	}
}

func TestStore_DeleteTask(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	s.SaveTask(StoredTask{Request: proto.TaskRequest{TaskID: "d1"}, Status: StatusPending})

	if err := s.DeleteTask("d1"); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}
	if _, err := s.GetTask("d1"); err == nil {
		t.Error("d1 应已删除")
	}
}

// newTestStore 构造临时 store。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir + "/test.db")
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	return s
}
