-- workspace-ops 初始 schema
-- 三张表：scans（扫描记录）/ projects（项目）/ project_facts（项目检查结果 KV）

CREATE TABLE scans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  project_count INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'running'  -- running / done / failed
);

CREATE TABLE projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  path TEXT NOT NULL,
  stack_primary TEXT,
  has_agents_md INTEGER NOT NULL DEFAULT 0,  -- 0/1
  git_branch TEXT,
  git_dirty INTEGER NOT NULL DEFAULT 0,
  last_scan_id INTEGER,
  last_scan_at TEXT,
  FOREIGN KEY (last_scan_id) REFERENCES scans(id)
);

CREATE INDEX idx_projects_slug ON projects(slug);

-- project_facts: 灵活 KV 表，存 inspector 产出的所有检查项
-- （go_version / test_count / npm_dep_count / build_artifacts 等）
CREATE TABLE project_facts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  scan_id INTEGER NOT NULL,
  fact_key TEXT NOT NULL,
  fact_value TEXT,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE,
  UNIQUE (project_id, scan_id, fact_key)
);

CREATE INDEX idx_facts_project ON project_facts(project_id);
CREATE INDEX idx_facts_scan ON project_facts(scan_id);
