// Command atlas 是 consensus-atlas 的统一 demo 入口。
//
// 用法：
//
//	go run ./cmd/atlas -d raft        # 运行 Raft demo（5 节点选举 + 日志复制）
//	go run ./cmd/atlas -d paxos       # Multi-Paxos 两阶段共识
//	go run ./cmd/atlas -d gossip      # Gossip 反熵最终一致
//	go run ./cmd/atlas -d pbft        # PBFT 三阶段拜占庭容错
//	go run ./cmd/atlas -d bully       # Bully 选举（含 Ring）
//	go run ./cmd/atlas -d clock       # Lamport + Vector Clock 因果序
//	go run ./cmd/atlas -d twopc       # 两阶段提交（原子提交协议）
//	go run ./cmd/atlas -d crdt        # G-Counter 无冲突复制数据类型
//	go run ./cmd/atlas -d byzgen      # 拜占庭将军问题（OM 口头消息算法）
//	go run ./cmd/atlas -d snapshot    # Chandy-Lamport 分布式快照
//	go run ./cmd/atlas -d viewstamped # Viewstamped Replication
//	go run ./cmd/atlas -d zab         # ZooKeeper Atomic Broadcast
//	go run ./cmd/atlas -d all         # 依次运行全部 demo
//	go run ./cmd/atlas -list          # 打印全部 12 算法的清单表格
//	go run ./cmd/atlas -version       # 打印版本号
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/QiuShichang/consensus-atlas/internal/bench"
	"github.com/QiuShichang/consensus-atlas/internal/byzantine"
	"github.com/QiuShichang/consensus-atlas/internal/byzgen"
	"github.com/QiuShichang/consensus-atlas/internal/clock"
	"github.com/QiuShichang/consensus-atlas/internal/crdt"
	"github.com/QiuShichang/consensus-atlas/internal/gossip"
	"github.com/QiuShichang/consensus-atlas/internal/leader_elect"
	"github.com/QiuShichang/consensus-atlas/internal/paxos"
	"github.com/QiuShichang/consensus-atlas/internal/raft"
	"github.com/QiuShichang/consensus-atlas/internal/snapshot"
	"github.com/QiuShichang/consensus-atlas/internal/twopc"
	"github.com/QiuShichang/consensus-atlas/internal/viewstamped"
	"github.com/QiuShichang/consensus-atlas/internal/zab"
)

var version = "dev"

func main() {
	var (
		demo    string
		showVer bool
		benchN  int
		list    bool
	)
	flag.StringVar(&demo, "d", "raft", "demo: raft|paxos|gossip|pbft|bully|clock|twopc|crdt|byzgen|snapshot|viewstamped|zab|all")
	flag.BoolVar(&showVer, "version", false, "打印版本号")
	flag.IntVar(&benchN, "bench", 0, "跑跨算法性能基准（指定次数，如 -bench 50），0=不跑")
	flag.BoolVar(&list, "list", false, "打印全部 12 算法清单（按家族分组，含论文/arXiv 链接）")
	flag.Parse()

	if showVer {
		fmt.Println("consensus-atlas", version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if list {
		runList()
		return
	}

	if benchN > 0 {
		runBench(ctx, benchN)
		return
	}

	if err := run(ctx, demo); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, demo string) error {
	if demo == "all" {
		all := []string{"raft", "paxos", "gossip", "pbft", "bully", "clock", "twopc", "crdt", "byzgen", "snapshot", "viewstamped", "zab"}
		for _, d := range all {
			fmt.Printf("\n========== ▶ %s ==========\n", d)
			if err := run(ctx, d); err != nil {
				return err
			}
		}
		return nil
	}

	switch demo {
	case "raft":
		return runRaft(ctx)
	case "paxos":
		return runPaxos(ctx)
	case "gossip":
		return runGossip(ctx)
	case "pbft":
		return runPBFT(ctx)
	case "bully":
		return runBully(ctx)
	case "clock":
		return runClock(ctx)
	case "twopc":
		return runTwopc(ctx)
	case "crdt":
		return runCrdt(ctx)
	case "byzgen":
		return runByzgen(ctx)
	case "snapshot":
		return runSnapshot(ctx)
	case "viewstamped":
		return runViewstamped(ctx)
	case "zab":
		return runZab(ctx)
	default:
		return fmt.Errorf("未知 demo: %s（可选: raft|paxos|gossip|pbft|bully|clock|twopc|crdt|byzgen|snapshot|viewstamped|zab|all）", demo)
	}
}

func runRaft(ctx context.Context) error {
	res, err := raft.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Raft 完成: Leader=%s term=%d commitIndex=%d 日志条目=%d 已复制到多数派=%v\n",
		res.LeaderID, res.Term, res.CommitIndex, res.LogLen, res.Replicated)
	return nil
}

func runPaxos(ctx context.Context) error {
	res, err := paxos.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Multi-Paxos 完成: 选定值=%v 轮数=%d promises=%d accepts=%d\n",
		res.ChosenValue, res.Rounds, res.Promises, res.Accepts)
	return nil
}

func runGossip(ctx context.Context) error {
	res, err := gossip.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Gossip 完成: %d 轮后收敛=%v 最终状态键数=%d\n",
		res.Rounds, res.Converged, len(res.FinalState))
	return nil
}

func runPBFT(ctx context.Context) error {
	res, err := byzantine.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ PBFT 完成: 已提交=%v sequence=%d view=%d 副本数=%d\n",
		res.Committed, res.Sequence, res.View, res.Replicas)
	fmt.Printf("\n场景一 诚实集群（4 节点全诚实）: prepared=%d committed=%d\n", res.Prepared, res.Commits)
	fmt.Printf("场景二 拜占庭容错（%s 为叛徒静默故障）: 诚实 replica %d/%d committed，叛徒自身 committed=%v\n",
		res.ByzTraitorID, res.ByzHonestOK, res.ByzHonestNum, res.ByzTraitorOK)
	fmt.Printf("       → 结论：n=4(f=1,quorum=3) 下，%d 个诚实节点仍达成一致=%v（PBFT 容忍 1 拜占庭）\n",
		res.ByzHonestNum, res.ByzCommitted)
	return nil
}

func runBully(ctx context.Context) error {
	res, err := leader_elect.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Leader 选举完成: Bully 当选 ID=%d, Ring 当选 ID=%d\n",
		res.BullyLeader, res.RingLeader)
	return nil
}

func runClock(ctx context.Context) error {
	res, err := clock.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ 逻辑时钟 demo 完成:\n")
	fmt.Printf("   Lamport 终值: %d\n", res.LamportFinal)
	fmt.Printf("   Vector n2 终值: %v\n", res.VectorFinalN2)
	fmt.Printf("   n2 vs n3 关系: %s（应为 Concurrent）\n", res.N2vsN3)
	fmt.Printf("   n1发 vs n2收 关系: %s（应为 HappensBefore）\n", res.SerializedN1vsN2)
	return nil
}

func runTwopc(ctx context.Context) error {
	res, err := twopc.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ 两阶段提交 demo 完成（参与方=%d）:\n", res.Participants)
	fmt.Printf("   场景一 全 Yes：事务 %s → Committed=%v，各参与方终态=%v\n",
		res.CommitTxn, res.Committed, res.FinalStates)
	fmt.Printf("   场景二 %s 拒绝：事务 %s → Aborted=%v，各参与方终态=%v\n",
		res.Rejecter, res.AbortTxn, res.Aborted, res.AbortStates)
	return nil
}

func runCrdt(ctx context.Context) error {
	res, err := crdt.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ G-Counter CRDT demo 完成（节点=%d）:\n", res.NodeCount)
	fmt.Printf("   各节点本地增量=%v\n", res.LocalInc)
	fmt.Printf("   %d 轮交换后收敛=%v，期望值=%d，各副本 Value=%v\n",
		res.Rounds, res.Converged, res.Expected, res.FinalValue)
	return nil
}

func runByzgen(ctx context.Context) error {
	res, err := byzgen.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ 拜占庭将军（OM）demo 完成（n=%d, f=%d）:\n", res.N, res.F)
	fmt.Printf("   司令官 %s（叛徒=%v）发命令 %q\n", res.CommanderID, res.CommanderBad, res.Order)
	fmt.Printf("   各 lieutenant 决定=%v\n", res.Decisions)
	fmt.Printf("   忠诚 lieutenant 达成一致=%v，共同决定值=%q\n", res.LoyalAgree, res.LoyalValue)
	return nil
}

func runSnapshot(ctx context.Context) error {
	res, err := snapshot.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Chandy-Lamport 快照 demo 完成（进程=%d，发起者=%s）:\n", res.Processes, res.Initiator)
	fmt.Printf("   各进程本地状态=%v\n", res.ProcessStates)
	fmt.Printf("   通道拓扑=%v\n", res.Channels)
	fmt.Printf("   各通道在途消息=%v\n", res.ChannelStates)
	fmt.Printf("   快照一致（CUT）=%v\n", res.Consistent)
	return nil
}

func runViewstamped(ctx context.Context) error {
	res, err := viewstamped.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ Viewstamped Replication demo 完成（副本=%d）:\n", res.Replicas)
	fmt.Printf("   阶段一 正常操作：初始 Primary=%s（view=%d），提交 %d 个请求，CommitNum=%d\n",
		res.InitialPrimary, res.InitialView, res.OpsCommitted, res.LastCommit)
	fmt.Printf("   各 Backup CommitNum=%v\n", res.BackupCommitted)
	fmt.Printf("   阶段二 视图变更：%s 下线 → 新 Primary=%s（view=%d），换主成功=%v\n",
		res.DownPrimary, res.FinalPrimary, res.FinalView, res.ViewChanged)
	fmt.Printf("   新 Primary 继续服务：处理新请求后 CommitNum=%d\n", res.NewPrimaryCommit)
	return nil
}

func runZab(ctx context.Context) error {
	res, err := zab.Demo(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n✅ ZAB（ZooKeeper Atomic Broadcast）demo 完成（follower=%d，epoch=%d）:\n",
		res.Followers, res.Epoch)
	fmt.Printf("   Leader 分配 zxid（递增）=%v\n", res.ProposedZXIDs)
	fmt.Printf("   Leader 已 commit=%v，单调有序=%v\n", res.CommittedZXIDs, res.Ordered)
	fmt.Printf("   各 Follower commit=%v\n", res.FollowerCommitted)
	return nil
}

// runBench 跑跨算法性能基准对比。每个算法的 Demo 跑 n 次，统计平均耗时 + 吞吐。
func runBench(ctx context.Context, n int) {
	// 各算法的 demo 包装成 func(context.Context) error
	cases := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"raft", func(c context.Context) error { _, err := raft.Demo(c); return err }},
		{"paxos", func(c context.Context) error { _, err := paxos.Demo(c); return err }},
		{"gossip", func(c context.Context) error { _, err := gossip.Demo(c); return err }},
		{"pbft", func(c context.Context) error { _, err := byzantine.Demo(c); return err }},
		{"bully", func(c context.Context) error { _, err := leader_elect.Demo(c); return err }},
		{"clock", func(c context.Context) error { _, err := clock.Demo(c); return err }},
		{"twopc", func(c context.Context) error { _, err := twopc.Demo(c); return err }},
		{"crdt", func(c context.Context) error { _, err := crdt.Demo(c); return err }},
		{"byzgen", func(c context.Context) error { _, err := byzgen.Demo(c); return err }},
		{"snapshot", func(c context.Context) error { _, err := snapshot.Demo(c); return err }},
		{"viewstamped", func(c context.Context) error { _, err := viewstamped.Demo(c); return err }},
		{"zab", func(c context.Context) error { _, err := zab.Demo(c); return err }},
	}
	var results []bench.Result
	for _, c := range cases {
		r := bench.Benchmark(c.name, c.fn, n)
		results = append(results, r)
		fmt.Println(bench.FormatResult(r))
	}
	fmt.Println()
	fmt.Println("──────── 对比汇总 ────────")
	fmt.Print(bench.FormatSummary(bench.Summary{Results: results}))
}

// algoMeta 描述单个共识算法的元数据（用于 -list 清单）。
//
// 数据来源：各包 doc.go 的论文注释 + demo 摘要。每条记录：
//   - name：与 -d 选项一致的算法 key
//   - family：所属家族（按"故障模型 + 解决问题"粗分）
//   - oneLine：一句话本质
//   - paper：论文/出处（作者 年份）
type algoMeta struct {
	name    string
	family  string
	oneLine string
	paper   string
}

// algoMetadata 是 12 个算法的清单（按家族分组排列）。
// 顺序即打印顺序：先 crash-fault 共识，再拜占庭、选举、时序、原子提交、最终一致。
var algoMetadata = []algoMeta{
	// ── Crash-fault 共识（容忍崩溃，> 1/2 多数派）──
	{"raft", "Crash-fault 共识", "强 Leader 选举 + 日志复制（term/commitIndex）", "Ongaro & Ousterhout, USENIX ATC 2014"},
	{"paxos", "Crash-fault 共识", "Multi-Paxos 两阶段：Prepare/Promise + Accept", "Lamport, ACM TOCS 1998 / Paxos Made Simple 2001"},
	{"viewstamped", "Crash-fault 共识", "VR 视图变更 + 主备复制", "Oki & Liskov, PODC 1988 / VR Revisited 2012"},
	{"zab", "Crash-fault 共识", "ZooKeeper 原子广播（epoch + zxid 主序）", "Junqueira, Reed & Serafini, 2008"},

	// ── Byzantine 共识（容忍恶意，> 2/3 多数派）──
	{"pbft", "Byzantine 共识", "PBFT 三阶段：pre-prepare/prepare/commit", "Castro & Liskov, OSDI 1999"},
	{"byzgen", "Byzantine 共识", "拜占庭将军 OM 口头消息算法（3f+1）", "Lamport, Shostak & Pease, ACM TOPLAS 1982"},

	// ── Leader 选举 ──
	{"bully", "Leader 选举", "Bully 选举 + Ring 选举（显式选主）", "Garcia-Molina, IEEE TC 1982"},

	// ── 因果序与分布式快照 ──
	{"clock", "因果序与时序", "Lamport Clock + Vector Clock（happens-before）", "Lamport, CACM 1978"},
	{"snapshot", "因果序与时序", "Chandy-Lamport 异步分布式快照（CUT 一致）", "Chandy & Lamport, ACM TOCS 1985"},

	// ── 原子提交 ──
	{"twopc", "原子提交", "两阶段提交：Vote-Request + Commit/Abort", "Gray, IBM RJ 1978 (Notes on DBOS)"},

	// ── 最终一致 / 反熵 ──
	{"gossip", "最终一致 / 反熵", "Gossip Push-Pull 反熵（流行病传播）", "Demers et al., ACM PODC 1987"},
	{"crdt", "最终一致 / 反熵", "G-Counter CRDT（max 合并收敛，无协调写）", "Shapiro et al., INRIA RR 7506, 2011"},
}

// algoPapers 是各算法的论文/arXiv 链接（与 algoMetadata 一一对应，便于跳转原文）。
var algoPapers = map[string]string{
	"raft":        "https://raft.github.io/raft.pdf",
	"paxos":       "https://lamport.org/pubs/paxos-simple.pdf",
	"viewstamped": "https://pmg.csail.mit.edu/papers/vr-revisited.pdf",
	"zab":         "https://zookeeper.apache.org/doc/r3.4.13/zab.pdf",
	"pbft":        "https://pmg.csail.mit.edu/papers/osdi99.pdf",
	"byzgen":      "https://lamport.org/pubs/byz.pdf",
	"bully":       "（无公开 PDF）",
	"clock":       "https://lamport.org/pubs/time-clocks.pdf",
	"snapshot":    "https://lamport.org/pubs/chandy-lamport.pdf",
	"twopc":       "（无公开 PDF）",
	"gossip":      "https://dl.acm.org/doi/10.1145/41840.41841",
	"crdt":        "https://hal.inria.fr/inria-00555588/",
}

// totalTestCount 是 internal/ 下所有 *_test.go 中 Test* 函数的总数。
// 用 `grep -cE "^func Test" internal/**/*_test.go` 统计得到（更新测试时需同步）。
const totalTestCount = 102

// runList 打印全部算法的清单表格（按家族分组），并附 bench 算法数 + 总测试数。
func runList() {
	fmt.Printf("=== 共识算法清单（%d 算法 / 按家族分组）===\n\n", len(algoMetadata))

	// 按家族分组（保持 algoMetadata 内顺序，遇新家族即起新节）。
	current := ""
	for _, a := range algoMetadata {
		if a.family != current {
			if current != "" {
				fmt.Println()
			}
			current = a.family
			fmt.Printf("【%s】\n", current)
		}
		fmt.Printf("  %-13s %s\n", a.name, a.oneLine)
		fmt.Printf("  %-13s %s — %s\n", "", a.paper, algoPapers[a.name])
	}

	fmt.Println()
	fmt.Println("──────── 统计 ────────")
	fmt.Printf("  bench 跨算法对比数：%d（-bench N 可跑横向基准）\n", len(algoMetadata))
	fmt.Printf("  总测试函数数：%d（go test ./...）\n", totalTestCount)
	fmt.Println("  用法：go run ./cmd/atlas -d <算法名>  运行单个 demo")
}
