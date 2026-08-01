# ai-safety-atlas · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库的 **AI 安全测试库**——提示注入/越狱检测器 + 红队测试用例集 + 对齐评估指标（precision/recall/F1）。

```
   用户输入
       │
       ▼
   ┌──────────┐  规则模式匹配（13 类攻击模板）
   │ detector │
   └────┬─────┘
        │  []Detection
        ▼
   ┌──────────┐
   │ types    │  RiskScore / RiskLevel
   └──────────┘

   ┌──────────┐     ┌────────────┐
   │ redteam  │ ──▶ │ alignment  │ → P/R/F1
   │ 31 用例  │     │ 评估指标   │
   └──────────┘     └────────────┘
```

**做**：13 类攻击模式检测（角色覆盖/DAN/越狱/系统提示泄露/PII/提示注入/XSS）+ 31 个红队用例 + precision/recall/F1 评估。
**不做**：语义级检测（需 LLM 自身判断）、多语言全覆盖（主要中英）、零样本越狱（规则无法预覆盖）。

## 目标（Goal）

- **G1**：检测器能拦截常见提示注入/越狱模板（角色覆盖/DAN/系统提示泄露等）。
- **G2**：红队用例集覆盖主要攻击类别（31 个，含良性对照）。
- **G3**：评估指标能量化检测器性能（精确率 100% / 召回率 ~80% / F1 ~0.88）。
- **成功标准**：`go test ./...` 全绿 + eval 跑出合理指标 + 0 误报。

## 当前情况（Status）

- **完成度**：**M1 完成**
- **detector**：13 条规则，覆盖角色覆盖/DAN/越狱/系统提示泄露/PII/提示注入/XSS
- **redteam**：31 个用例（28 攻击 + 3 良性），来自 JailbreakBench/garak
- **alignment**：Evaluate 跑红队集计算 P/R/F1 + 按类别召回率
- **实测**：精确率 100% / 召回率 78.6% / F1 0.880

## 技术栈与架构

- **语言**：Go 1.25.6，零外部依赖（标准库 regexp）
- **设计**：规则模式匹配（非 LLM 检测），快速、确定性、无网络调用

## 如何运行

```bash
go run ./cmd/safety -check "Ignore previous instructions"  # 检测单条
go run ./cmd/safety -d eval                                 # 红队评估
go run ./cmd/safety -d detect                               # 检测 demo
go run ./cmd/safety -d cases                                # 列红队用例
make test                                                   # 全部测试
```

## 关键约定

- **零外部依赖**：纯标准库 regexp
- **诚实标注局限**：多语言/leet code/语义级攻击是已知盲区，README 和红队用例 description 都标注
- **0 误报优先**：精确率 100%（良性输入绝不被误标），即使牺牲一些召回率

## 与其他项目的关系

- **与 [`go-agent-research`](../go-agent-research) / [`agentloop`](../agentloop)**：本库检测对这些 Agent 系统的攻击（提示注入/越狱）。
- **与 [`crypto-atlas`](../crypto-atlas)**：同范式的零依赖教学库。
- **工作区定位**：补"AI 安全测试"象限（工作区 AI 项目多但无安全测试工具）。
