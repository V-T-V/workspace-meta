package pipeline

import (
	"strings"
	"testing"
)

// TestExpandEnv：${VAR} 应被替换为 os.Getenv("VAR") 的值。
// 用 t.Setenv 设置变量（测试结束自动恢复，避免污染其他测试）。
func TestExpandEnv(t *testing.T) {
	t.Setenv("TEST_DATA_PATH", "/tmp/test")
	expanded := expandEnv([]byte("path: ${TEST_DATA_PATH}/data"))
	if got := string(expanded); got != "path: /tmp/test/data" {
		t.Errorf("expandEnv 应替换 ${TEST_DATA_PATH}，实际 %q", got)
	}
}

// TestExpandEnvUnset：未设置的环境变量替换为空字符串（${...} 整段消失）。
func TestExpandEnvUnset(t *testing.T) {
	// 确保该变量确实未设置（t.Setenv 会在测试后恢复）。
	t.Setenv("DEFINITELY_NOT_SET_VAR", "")
	expanded := expandEnv([]byte("x: ${DEFINITELY_NOT_SET_VAR}"))
	if got := string(expanded); got != "x: " {
		t.Errorf("未设置变量应替换为空，实际 %q", got)
	}
}

// TestExpandEnvNoVar：无 ${} 的内容应原样返回。
func TestExpandEnvNoVar(t *testing.T) {
	expanded := expandEnv([]byte("name: test"))
	if got := string(expanded); got != "name: test" {
		t.Errorf("无 ${} 的内容应不变，实际 %q", got)
	}
}

// TestExpandEnvMultiple：一行内多个 ${VAR} 都应被替换。
func TestExpandEnvMultiple(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	expanded := expandEnv([]byte("${DB_HOST}:${DB_PORT}/app"))
	if got := string(expanded); got != "localhost:5432/app" {
		t.Errorf("多变量替换应得 localhost:5432/app，实际 %q", got)
	}
}

// TestExpandEnvUnclosed：未闭合的 ${ 不替换（保留字面 $）。
// 因为找不到 }，扫描器退回把 '$' 当普通字符输出。
func TestExpandEnvUnclosed(t *testing.T) {
	expanded := expandEnv([]byte("bad: ${UNCLOSED"))
	if got := string(expanded); got != "bad: ${UNCLOSED" {
		t.Errorf("未闭合 ${ 应保留字面，实际 %q", got)
	}
}

// TestExpandEnvAdjacent：相邻的两个 ${VAR} 不互相干扰。
func TestExpandEnvAdjacent(t *testing.T) {
	t.Setenv("A", "x")
	t.Setenv("B", "y")
	expanded := expandEnv([]byte("${A}${B}"))
	if got := string(expanded); got != "xy" {
		t.Errorf("相邻变量应得 xy，实际 %q", got)
	}
}

// TestExpandEnvDollarAlone：单独的 $（不跟 {）应保留为字面 $。
func TestExpandEnvDollarAlone(t *testing.T) {
	expanded := expandEnv([]byte("price: $5 only"))
	if got := string(expanded); got != "price: $5 only" {
		t.Errorf("单独 $ 应保留字面，实际 %q", got)
	}
}

// TestParseWithEnvExpansion：端到端——Parse 在解析 YAML 前先做环境变量展开，
// 故 YAML 里引用 ${...} 的字段最终能拿到真实值。
func TestParseWithEnvExpansion(t *testing.T) {
	t.Setenv("PIPE_NAME", "envdemo")
	t.Setenv("CSV_PATH", "/data/in.csv")
	yaml := []byte(strings.Join([]string{
		"name: ${PIPE_NAME}",
		"steps:",
		"  - id: read",
		"    type: source",
		"    connector: csv",
		`    config: { path: "${CSV_PATH}" }`,
	}, "\n"))
	p, err := Parse(yaml)
	if err != nil {
		t.Fatalf("Parse 应成功（${} 已展开为合法 YAML）: %v", err)
	}
	if p.Name != "envdemo" {
		t.Errorf("name 应被替换为 envdemo，实际 %q", p.Name)
	}
	if len(p.Steps) != 1 || p.Steps[0].ID != "read" {
		t.Fatalf("应解析出 1 个步骤 read，实际 %+v", p.Steps)
	}
	path, _ := p.Steps[0].Config["path"].(string)
	if path != "/data/in.csv" {
		t.Errorf("config.path 应被替换为 /data/in.csv，实际 %q", path)
	}
}
