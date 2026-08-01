# K 组 · 基础设施与工程工具线（8 个项目）

> 创建于 2026-08-01~02，从工作区盲区分析到 8 个项目落地，累计 28 个功能扩展。
> git commit: `cc17908`（初始创建）→ `c383427`（文档同步）→ `6ffad07`（第 5 轮扩展）

## 项目总表

| 项目 | 定位 | 测试包 | 关键能力 |
|------|------|--------|----------|
| [consensus-atlas](./consensus-atlas) | 分布式系统教学 | 14 | 12 算法（Raft/Paxos/PBFT/Gossip/2PC/CRDT/拜占庭将军/快照/VR/ZAB）+ bench + -list |
| [lang-impl](./lang-impl) | 编译器/语言实现 | 4 | M 语言：lex→parse→interpret + REPL + WASM 后端 + 数组 + 内置函数 |
| [crypto-atlas](./crypto-atlas) | 密码学教学 | 12 | 11 算法（凯撒/维吉尼亚/XOR/AES/DES/SHA-256/MD5/RSA/DH/HMAC/TLS 握手模拟） |
| [regex-engine](./regex-engine) | 正则引擎 | 3 | Thompson NFA 抗 ReDoS + Match/FindAll/Replace/分组捕获/(?i)/-replace |
| [ai-safety-atlas](./ai-safety-atlas) | AI 安全测试 | 4 | 注入/越狱检测 + 多轮上下文 + 批量报告 + 自定义规则 + URL 钓鱼（P100%/R78.6%/F1=0.88） |
| [obs-lite](./obs-lite) | 可观测性平台 | 4 | metrics(counter/gauge/histogram/CallbackGauge) + trace + HTTP /metrics(Prometheus) |
| [flow-pipe](./flow-pipe) | ETL 数据管道 | 6 | 11 连接器 + retry + 死信 + 状态恢复 + DAG DOT + 定时调度 + CSV header |
| [workspace-ops](./workspace-ops) | 工作区管理 | 4 | scan/report/serve/test/diff 五子命令 + testrunner + REST API + Web 看板 |

**合计：51 个测试包全绿，28 个功能扩展，零外部依赖（纯 Go 标准库）。**

## 补的 8 个盲区

| 盲区 | 项目 | 状态 |
|------|------|------|
| 分布式系统教学 | consensus-atlas | ✅ 12 算法 |
| 后端 CLI 工具 | workspace-ops | ✅ 5 子命令 |
| 数据工程 / ETL | flow-pipe | ✅ 11 连接器 |
| 编译器 / 语言实现 | lang-impl | ✅ WASM 可执行 |
| 安全 / 密码学教学 | crypto-atlas | ✅ 11 算法 |
| 正则引擎 | regex-engine | ✅ 抗 ReDoS |
| AI 安全测试 | ai-safety-atlas | ✅ 红队评估 |
| 可观测性平台 | obs-lite | ✅ Prometheus 兼容 |

## 工程公约（8 项目统一）

- 模块名：`github.com/QiuShichang/<name>`
- Go 1.25.6
- 零 Web 框架（标准库 net/http + Go 1.22 方法路由）
- 零 ORM（database/sql + 手写 repo）
- 零 CGO（modernc.org/sqlite 纯 Go）
- 日志：log/slog
- 测试：go test（无第三方 runner）
- 文档：README.md + AGENTS.md（七段式）+ LICENSE + Makefile + .gitignore

## 28 个功能扩展清单

### consensus-atlas（2）
1. PBFT 拜占庭容错测试（IsTraitor + TestByzantineFaultTolerance）
2. `-list` 命令（12 算法按家族 + 论文链接）

### lang-impl（4）
3. WASM 后端（手写 WASM 二进制，node 验证 add(3,4)=7）
4. REPL 状态保持（历史累积模式）
5. 数组支持（字面量/索引/len/嵌套/越界检测）
6. 内置函数（substr/charAt/push）

### crypto-atlas（2）
7. HMAC-SHA256（RFC 2104，标准库交叉验证）
8. TLS 握手模拟（RSA+DH+AES+HMAC 5 阶段综合演示）

### regex-engine（5）
9. FindAll（子串位置提取）
10. ReplaceAll（替换）
11. 分组捕获 FindAllWithGroups（NFA 边带标记 + 状态快照）
12. `(?i)` 不区分大小写
13. `-replace` 命令行模式

### ai-safety-atlas（5）
14. 多轮对话上下文检测（渐进式越狱升级 Critical）
15. 批量检测 + JSON 报告
16. 自定义规则 API（AddRule/AddRuleIgnoreCase）
17. URL/钓鱼检测规则（短链接/裸IP/凭证URL/外泄服务）
18. `-batch` 批量文件检测模式

### obs-lite（2）
19. HTTP /metrics 端点（Prometheus scrape 兼容）
20. CallbackGauge（回调自动采集 OS 指标）

### flow-pipe（5）
21. retry 重试机制（source/transform/sink 三类）
22. dead_letter 死信兜底（失败行写入死信 sink，管道不中断）
23. 状态恢复（RunWithOptions + WithSkipSteps，失败后从成功步骤恢复）
24. DAG DOT 可视化（ToDOT + `-dot` flag → Graphviz）
25. CSV source header 配置（无标题行 CSV 支持）

### workspace-ops（3）
26. testrunner 实跑测试采集（`test` 子命令，真跑 go test/node --test）
27. scan 性能优化（git 3→1 子进程，减少 67%）
28. diff 模式（比较两次 scan 的项目新增/删除/变更）

## 跨项目集成验证

workspace-ops 成功扫描全部 8 个新项目（65 项目总量），所有项目被正确识别为 Go 技术栈，全部有 AGENTS.md，全部有测试文件。

## 演进历程

每个项目都经历了完整的工程闭环：

```
创建 → M1 → code review（4 agent）→ P0/P1 修复 →
M2 扩展 → 深度优化 → 深度扩展（第 1-5 轮）→
深度优化（bench 精度/crdt 死代码/wasm import 清理/PKCS7 防护/DOT 引号）→
跨项目集成验证 → git commit
```

## git 提交

```
6ffad07 feat: 扩展 4 个项目功能（第 5 轮）
c383427 docs: 更新工作区导航（K 组 8 项目描述同步 + 项目数修正）
cc17908 feat: 新增 8 个基础设施与工程工具项目（K 组）
```
