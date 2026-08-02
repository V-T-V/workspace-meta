package inspector

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout 是单次 git 子进程的超时上限。
// 防止慢仓库（如网络文件系统、超大 working tree）的 git status 卡死整个扫描。
const gitTimeout = 5 * time.Second

// gitStatus 用 git CLI 查询 dir 的分支与 dirty 状态。
// 返回 (branch, dirty, ok)：ok=false 表示 git 不可用或非 git 仓库。
//
// 性能：原先每个项目要起 3 个 git 子进程（rev-parse is-inside-work-tree /
// rev-parse --abbrev-ref HEAD / status --porcelain），在 Windows 上进程
// 启动开销很大。现在合并成一条 `git status -b --porcelain`：
//   - 命令失败 → 非 git 仓库或 git 不可用 → ok=false
//   - 成功 → 第一行 "## branch..." 提取分支名；其后任何行 = dirty
//
// 子进程受 gitTimeout 超时保护。
func gitStatus(gitCmd, dir string) (branch string, dirty bool, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	// -b：在 porcelain 输出首行加上分支信息（## branch...），
	// 这样一条命令同时拿到「是否 git 仓库」「分支名」「是否 dirty」。
	cmd := exec.CommandContext(ctx, gitCmd, "status", "-b", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// 非 git 仓库、git 未安装、或超时：统一视为不可用。
		return "", false, false
	}

	// 解析输出。行分隔按 \n（Windows 上 git 也会用 LF 输出 porcelain 格式）。
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return "", false, true
	}

	// 首行形如 "## main" / "## main...origin/main" / "## main...origin/main [ahead 2]"
	// / "## HEAD (no branch)"（detached）。详见 git status 的 porcelain v1 文档。
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "## ") {
		header := strings.TrimPrefix(first, "## ")
		// 取到首个空白或 "..."（upstream 分隔符）之前的部分作为分支名。
		// cutAny 在第一个命中字符处截断。
		header = cutAny(header, " \t", "...")
		switch header {
		case "":
			// 空兜底，不写分支。
		case "HEAD":
			// detached HEAD：保留 "HEAD" 作为可读标识。
			branch = "HEAD"
		default:
			branch = header
		}
	}

	// 任意非空后续行 = 有未提交改动 = dirty。
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			dirty = true
			break
		}
	}
	return branch, dirty, true
}

// cutAny 返回 s 中首次出现 cutsets 中任一参数的前缀子串之前的部分。
// 例如 cutAny("main...origin/main", " \t", "...") == "main"。
// 若都不存在则返回整个 s。每个 cutset 内的字符序列都按整体子串匹配
// （空格 / tab 按单字符，"..." 按 3 字符子串）。
func cutAny(s string, cutsets ...string) string {
	min := len(s)
	found := false
	for _, cs := range cutsets {
		if cs == "" {
			continue
		}
		if i := strings.Index(s, cs); i >= 0 && i < min {
			min = i
			found = true
		}
	}
	if !found {
		return s
	}
	return s[:min]
}
