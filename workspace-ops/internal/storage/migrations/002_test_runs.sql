-- 测试运行结果表（M2 testrunner 采集）
-- 每次 scan 时可选实跑各项目测试，结果记录于此。

CREATE TABLE test_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  scan_id INTEGER NOT NULL,
  status TEXT NOT NULL,           -- pass / fail / skipped / timeout / error
  command TEXT,                   -- 实际跑的命令
  duration_ms INTEGER,            -- 耗时毫秒
  output_tail TEXT,               -- 输出末尾（失败诊断用，截断）
  ran_at TEXT NOT NULL,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

CREATE INDEX idx_test_runs_project ON test_runs(project_id);
CREATE INDEX idx_test_runs_scan ON test_runs(scan_id);
