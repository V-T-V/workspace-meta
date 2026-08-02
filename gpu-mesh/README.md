# GPU Mesh · 异地分布式 GPU 算力调度平台

> 把上百台分散在不同地方（NAT 后）的 Windows GPU 机器纳管成统一算力池，提升利用率，**机器有人使用时自动让位降级**。

## 解决什么问题

上百台 4060 8GB 显卡的机器，分布在不同的地方（异地公网/NAT 后），利用率很低，且没有统一的调度系统。

GPU Mesh 把它们组成一个集群：
- 🖥️ **统一纳管**：反向 WS 穿透 NAT，百台异地机器一个控制台管到底
- 📊 **利用率可视化**：每台机器的 GPU 利用率/显存/温度/让位状态实时可见，回答"低在哪"
- 🤝 **让位调度**：机器有人使用时自动降低算力占比（100% → 50% → 10%），不抢用户资源
- 🚀 **推理服务**：组成 OpenAI 兼容的 LLM 推理集群（Phase 2）
- 🎯 **智能调度**：GPU 感知 + 模型路由 + 最少连接（Phase 3）
- 📦 **批量处理**：数据集分片 Map-Reduce 到百台并行（Phase 4）

## 快速开始

### 1. 编译

```bash
make agent          # 编译 Windows Agent（部署到每台 4060）
make relay          # 编译 Linux Relay（部署到公网 VPS）
```

### 2. 部署中继（公网 VPS）

```bash
./gpu-mesh-relay -addr :7780 -token your-secret
# 控制台: http://VPS公网IP:7780/?token=your-secret
```

### 3. 部署 Agent（每台 4060 机器，管理员 PowerShell）

```powershell
.\install-agent.ps1 -Relay "ws://VPS公网IP:7780" -AgentID "gpu-bj-01" -Token "your-secret"
```

或手动：

```bash
gpu-mesh-agent.exe install -relay ws://VPS:7780 -id gpu-bj-01 -token your-secret
gpu-mesh-agent.exe start
```

### 4. 查看仪表盘

打开 `http://VPS公网IP:7780/`，看到所有在线 Agent 的：

| 主机 | GPU | 利用率 | 显存 | 温度 | 让位状态 |
|------|-----|--------|------|------|---------|
| gpu-bj-01 | RTX 4060 | ▓▓░░ 35% | 3.4/8GB | 52℃ | 🟢 IDLE 100% |
| gpu-sh-02 | RTX 4060 | ▓░░░ 10% | 0.5/8GB | 45℃ | 🟡 ACTIVE 50% |
| gpu-gz-03 | RTX 4060 | ░░░░ 0%  | 0/8GB   | 30℃ | 🔴 BUSY 10% |

## 让位调度（核心特性）

机器有人使用时，Agent 本地自动检测并降低算力占比：

| 状态 | 触发条件 | 算力配额 |
|------|---------|---------|
| 🟢 **IDLE** | 用户空闲 >5min 且外部 GPU<20% | **100%** 全力跑集群任务 |
| 🟡 **ACTIVE** | 用户空闲 60s~5min | **50%** 降并发/降量化 |
| 🔴 **BUSY_HUMAN** | 用户空闲 <60s 或外部 GPU>50% | **10%** 只跑轻量或暂停 |

检测维度：
- 用户空闲时间（`GetLastInputInfo`）—— 最直接信号
- 外部 GPU 占用（nvidia-smi 扣除本 Agent 进程）
- 前台窗口抖动（`GetForegroundWindow`）

Agent 本地自治（反应最快），任务带 `MinBudget` 要求，配额不足时 NACK 让 Relay 重调度到其他空闲节点。

## 技术栈

- **Go 1.22+**（零 CGO，便于交叉编译 Windows）
- `coder/websocket` 反向长连接 · `kardianos/service` Windows 服务化
- `bbolt` 任务持久化 · nvidia-smi 命令解析（零 CGO GPU 采集）
- Ollama + llama.cpp 双引擎抽象 · 原生 JS + embed.FS 控制台（无构建）

## 路线图

- [x] **Phase 1 · MVP**：组网 + GPU 监控仪表盘 + 引擎探测 + 让位检测
- [x] **Phase 2 · 推理网关**：OpenAI 兼容 API + 模型管理 + 轮询负载均衡
- [x] **Phase 3 · GPU 感知调度**：模型路由 + 显存感知 + 让位执行 + 预加载
- [x] **Phase 4 · 批量离线推理**：Map-Reduce + 失败重试
- [x] **Phase 5 · 训练/微调**：LoRA/QLoRA 编排 + 断点续训
- [x] **Phase 6 · 生产加固**：mTLS + 审计 + 多租户配额 + 告警

**全部 6 Phase 完成**（39 Go 文件 / ~5700 行 / 4 直接依赖 / 真实 Ollama 推理验证通过）。

详见 [AGENTS.md](./AGENTS.md)。

## 部署

完整部署方案见 [`DEPLOY.md`](./DEPLOY.md)：Relay 一键初始化（systemd/Docker）+ Nginx TLS + Ollama 内网镜像 + Agent 批量部署（CSV 驱动 WinRM 远程安装百台）+ 验收清单 + 故障排查。

```bash
# Relay（VPS）：make deploy-pack-relay 打包，上传解压后
sudo bash relay-init.sh

# Agent（每台 4060）：make deploy-pack-agent 打包
.\install-agent.ps1 -Relay wss://gpu-mesh.yourdomain.com -Token xxx -AgentID gpu-bj-001
# 或百台批量：.\batch-install-agent.ps1 -Csv agents.csv -Relay ... -Token ...
```
