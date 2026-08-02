# gpu-mesh · AGENTS.md

## 项目内容（What）

Go 实现的**异地分布式 GPU 算力调度平台**——把上百台分散在不同地方（NAT 后）的 Windows GPU 机器纳管成统一算力池，提升利用率，并在机器有人使用时自动让位降级。

两组件架构：

```
任意 OpenAI SDK ──► 中继节点(公网 VPS) ◄──反向WS长连接── Windows Agent(GPU机器·Windows服务)
                     ├ GPU感知调度器                      ├ nvidia-smi 采集(利用率/显存/温度)
                     ├ 推理网关 /v1/chat/completions      ├ 让位检测器(GetLastInputInfo/前台窗口)
                     ├ 优先级任务队列                      ├ engine.Engine 抽象(Ollama+llama.cpp)
                     └ Web 控制台(embed.FS)               └ 执行器(推理/批量/训练)
```

**做**：反向 WS 穿透 NAT、Windows 服务化常驻、GPU 利用率监控、让位调度（有人用就降算力占比）、Ollama/llama.cpp 双引擎抽象、OpenAI 兼容推理网关（含流式 SSE）、GPU 感知四维调度、批量 Map-Reduce、LoRA/QLoRA 训练编排、mTLS+审计+多租户+告警。**全 6 Phase 完成**（43 Go 文件 / ~6400 行 / 4 直接依赖 / 31 单测 / 真实 Ollama 推理+流式验证通过）。

## 核心特性：让位调度（Yield Scheduling）

这是本项目的第一运维约束——**机器有人使用时，主动降低算力占用占比**。

### 检测维度（Agent 本地自治）
| 信号 | 采集方式 |
|------|---------|
| 用户空闲时间 | Win32 `GetLastInputInfo()` |
| 外部 GPU 占用 | nvidia-smi 总利用率扣除本 Agent 进程 |
| 前台窗口抖动 | `GetForegroundWindow()` 采样 |

### 三档状态机
- **IDLE** (idle>5min 且外部GPU<20%) → Budget 100% 全力跑
- **ACTIVE** (idle 60s~5min) → Budget 50% 降并发
- **BUSY_HUMAN** (idle<60s 或 外部GPU>50%) → Budget 10% 只跑轻量

Agent 本地反应（不依赖 Relay 往返），任务带 `MinBudget` 要求，配额不足时 Agent NACK 让 Relay 重调度到其他节点。

## 分阶段路线图

| Phase | 内容 | 状态 |
|-------|------|------|
| **1 · MVP** | 组网 + GPU 监控仪表盘 + 引擎探测 + 让位检测埋点 | ✅ 完成 |
| **2 · 推理网关** | 引擎抽象(Ollama+llama.cpp Chat/Embed/Pull) + OpenAI API + 轮询负载均衡 + 模型管理 | ✅ 完成 |
| **3 · GPU 感知调度** | 四维调度(让位/模型路由/显存/最少连接) + 让位重调度 + 模型预加载 | ✅ 完成 |
| **4 · 批量离线推理** | 数据集分片 Map-Reduce + 并行分发 + 失败重试 + 进度跟踪 + 结果聚合 | ✅ 完成 |
| **5 · 训练/微调** | LoRA/QLoRA(unsloth/peft)脚本生成 + 资源独占调度 + 断点续训 + 让位暂停 | ✅ 完成 |
| **6 · 生产加固** | mTLS CA(内置自签+enroll token+CRL) + 审计日志(JSONL) + 多租户配额(RPM/token) + 告警 webhook | ✅ 完成 |

## 技术栈与架构

- **语言**：Go 1.22+（开发机 go1.25.6，零 CGO 便于交叉编译 Windows）
- **核心依赖**（4 个生产依赖）：
  - `github.com/coder/websocket v1.8.15` —— 反向长连接
  - `github.com/kardianos/service v1.3.0` —— Windows 服务化
  - `github.com/google/uuid v1.6.0` —— 消息/任务 ID
  - `go.etcd.io/bbolt v1.5.0` —— 任务持久化
- **零依赖原则**：无 Web 框架（标准库 `net/http`）、无 ORM、无前端构建（原生 JS + `embed.FS`）

目录树：

```
gpu-mesh/
├── cmd/
│   ├── relay/main.go        # 中继入口(调度器+网关+Web)
│   └── agent/main.go        # Agent 入口(Windows 服务)
├── internal/
│   ├── proto/               # 协议层(Envelope + GPU/Yield/Register/Heartbeat/Task)
│   │   ├── envelope.go gpu.go task.go uuid.go
│   ├── relay/               # 中继
│   │   ├── server.go        # WS 接入 + serveAgent 消息分发
│   │   ├── registry.go      # 在线 Agent 注册表(含 GPU/让位快照) + SnapshotExcluding
│   │   ├── router.go        # REST API + SSE
│   │   ├── eventbus.go      # 事件总线(fan-out 到控制台)
│   │   ├── store.go         # bbolt 任务/结果持久化
│   │   ├── gateway.go       # Phase 2 OpenAI 兼容网关(/v1/chat/embeddings) + Phase 6 配额
│   │   ├── scheduler.go     # Phase 3 GPU 感知调度器(让位/模型/显存/最少连接) + 预加载
│   │   ├── batch.go         # Phase 4 批量 Map-Reduce 编排器
│   │   ├── train.go         # Phase 5 训练编排器(资源独占 + 断点续训)
│   │   └── audit.go         # Phase 6 审计(JSONL) + 多租户配额 + 告警 webhook
│   ├── agent/               # Agent
│   │   ├── agent.go         # 主结构(引擎探测缓存)
│   │   ├── connection.go    # 反向 WS + 指数退避重连 + EarlyInit(禁代理)
│   │   ├── heartbeat.go     # 心跳(带 GPU 快照 + 让位状态)
│   │   ├── tasks.go         # 任务分发 + 让位 NACK + 取消
│   │   ├── executors.go     # Phase 2 推理/嵌入/pull + Phase 4 批量执行器
│   │   ├── train_helpers.go # Phase 5 训练脚本生成(unsloth/peft) + 输出解析
│   │   ├── yield.go         # ★ 让位检测器(三档状态机)
│   │   ├── idle_windows.go  # Win32 GetLastInputInfo/GetForegroundWindow
│   │   ├── idle_other.go    # 非 Windows stub
│   │   ├── service.go       # kardianos Windows 服务化
│   │   └── config.go helpers.go
│   ├── gpumon/              # ★ GPU 监控
│   │   ├── nvidia_smi.go    # nvidia-smi 命令解析(零 CGO)
│   │   ├── collector.go     # 周期采集循环
│   │   └── nvidia_smi_test.go
│   ├── engine/              # 引擎抽象
│   │   ├── engine.go        # Engine 接口(Chat/Embed/Pull/Probe) + ProbeAll
│   │   ├── ollama.go        # OllamaEngine(调 :11434)
│   │   └── llamacpp.go      # LlamaCppEngine(调 :8080)
│   ├── mtls/                # Phase 6 mTLS CA(内置自签 + enroll token + CRL)
│   │   └── ca.go
│   └── web/                 # 控制台(embed.FS 原生 JS)
│       ├── web.go index.html
├── scripts/install-agent.ps1
├── deploy/                         # 部署套件（DEPLOY.md 总入口）
│   ├── relay-init.sh               # Relay 一键初始化（建用户/编译/装 systemd/生成 token）
│   ├── systemd/gpu-mesh-relay.service  # systemd 单元（含安全沙箱 + 资源限制）
│   ├── nginx/gpu-mesh-nginx.conf   # Nginx 反代（TLS + WSS 24h 长连接 + SSE 不缓冲）
│   ├── docker/Dockerfile + docker-compose.yml  # Docker 部署
│   ├── playbook/                   # Agent 批量部署
│   │   ├── batch-install-agent.ps1 # CSV 驱动 WinRM 远程安装百台
│   │   └── agents.csv.example      # 主机清单模板
│   └── ollama-mirror.md            # 内网镜像指南（旁路模型分发瓶颈）
├── DEPLOY.md                       # 完整部署手册（验收清单 + 故障排查）
└── Makefile
```

## 如何运行

```bash
# 编译（Windows EXE）
make agent          # → bin/gpu-mesh-agent.exe
make relay-windows  # → bin/gpu-mesh-relay.exe（或 make relay 产 Linux 版部署 VPS）

# 本地联调（两个终端）
make run-relay                    # 终端1：Relay 监听 :7780
make run-agent                    # 终端2：Agent 连本地 Relay

# 生产部署（公网 VPS）
./gpu-mesh-relay -addr :7780 -token s3cret   # 启用鉴权

# 每台 4060 机器（管理员 PowerShell）
.\install-agent.ps1 -Relay "ws://VPS公网IP:7780" -AgentID "gpu-bj-01" -Token "s3cret"
# 或手动：
gpu-mesh-agent.exe install -relay ws://VPS:7780 -id gpu-bj-01 -token s3cret
gpu-mesh-agent.exe start
```

打开 `http://VPS公网IP:7780/` 查看 GPU 仪表盘（利用率/显存/温度/让位状态实时刷新）。

## API 一览（Phase 2-6）

```bash
# Phase 2 OpenAI 兼容推理（任意 OpenAI SDK 直接用）
curl http://VPS:7780/v1/chat/completions -d '{"model":"qwen2.5:7b","messages":[...]}'
curl http://VPS:7780/v1/embeddings -d '{"model":"nomic-embed-text","input":[...]}'
curl http://VPS:7780/v1/models

# 模型管理
curl -X POST http://VPS:7780/api/models/pull -d '{"engine":"ollama","model":"qwen2.5:7b"}'

# Phase 4 批量离线推理（Map-Reduce）
curl -X POST http://VPS:7780/api/batches -d '{"model":"qwen2.5:7b","items":["...","..."],"shard_size":20}'
curl http://VPS:7780/api/batches/{batch_id}  # 查进度

# Phase 5 训练/微调（LoRA/QLoRA）
curl -X POST http://VPS:7780/api/train -d '{"base_model":"Qwen/Qwen2.5-1.5B","dataset":"train.jsonl"}'
curl http://VPS:7780/api/train/{job_id}  # 查进度

# Phase 6 多租户 + 告警
curl -X POST http://VPS:7780/api/tenants -d '{"name":"业务A","rpm":100}'  # 生成 API Key
curl -X POST http://VPS:7780/api/alerts/webhook -d '{"name":"钉钉","url":"https://..."}'
```

## 关键约定（踩坑记录）

- **禁用系统代理**：受控端机器常装代理软件（Clash/v2ray），WS 握手经代理会失败（代理返回 200 非 101）。
  Agent 必须在 `main()` 第一行调用 `agent.EarlyInit()` 清空代理环境变量——因为 `net/http` 的 `envProxyOnce`（sync.Once）在首次 HTTP 请求时永久缓存代理配置，之后再 `os.Unsetenv` 无效。
  且 `dialRelay` 用显式 `noProxyClient`（自建 Transport + Proxy 返回 nil）双保险。
- **GetTickCount64 在 kernel32.dll**：不在 user32.dll（Win32 API 常见陷阱）。
- **Win32 syscall 容错**：`idleSecondsWindows`/`foregroundChangedWindows` 都 defer recover，API 失败降级返回（保守视为 BUSY，不抢用户资源）。
- **Go 1.22 新 ServeMux**：所有路由统一用 `"METHOD /path"` 形式（`GET /agent` 而非 `/agent`），否则与 `GET /` 根路径冲突 panic。
- **流式推理链路**：`/v1/chat/completions` 带 `stream:true` 时，Agent 调 `ChatStream` 逐 token 经 `TaskProgress(Step="delta")` 回传，Relay 网关转 OpenAI SSE `data:` 帧透传。让位重调度同样适用于流式（首节点 NACK 会换节点重投）。

## 测试覆盖（go test ./... 全绿，31 个测试）

| 包 | 覆盖点 |
|----|--------|
| `gpumon` | nvidia-smi CSV 解析（单卡/多卡/N/A 值/坏行/空输入）|
| `relay` (scheduler) | 让位优先(IDLE>ACTIVE>BUSY) / MinBudget 过滤 / 显存过滤 / 训练独占排除 / 模型路由 / 最少连接 / 槽位增减 |
| `relay` (batch) | shardItems 分片（正常/整除/超长/默认/空）|
| `agent` (connection) | normalizeRelayURL（裸 host/ws:// 补 path）/ appendToken |
| `mtls` | CA 初始化 / 状态持久化 / enroll token 一次性 / CSR 签发 / 撤销+CRL / TLSConfig |

## 与其他项目的关系

- **独立项目**，不依赖 go-rmm。但借鉴 go-rmm 经过 P0-P7 迭代验证的可靠架构模式：反向 WS 穿透 NAT、信封协议、Windows 服务化、bbolt 持久化、mTLS CA 模式。
- go-rmm 缺 GPU 感知调度/模型管理/推理网关/让位机制——这正是 gpu-mesh 要补的核心价值。
