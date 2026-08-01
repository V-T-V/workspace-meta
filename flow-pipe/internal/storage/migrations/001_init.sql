-- flow-pipe 初始 schema：管道定义 + 运行历史。

-- pipelines 存管道定义（name 唯一，按名查找/覆盖）。
CREATE TABLE pipelines (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  definition_yaml TEXT NOT NULL,
  created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- runs 存运行历史（每次执行一条；steps_json 序列化 pipeline.RunResult.Steps）。
CREATE TABLE runs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  pipeline_id   INTEGER,
  pipeline_name TEXT NOT NULL,
  started_at    TEXT,
  finished_at   TEXT,
  status        TEXT,
  steps_json    TEXT,
  error         TEXT,
  FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE SET NULL
);

CREATE INDEX idx_runs_pipeline ON runs(pipeline_name);
CREATE INDEX idx_runs_started  ON runs(started_at);
