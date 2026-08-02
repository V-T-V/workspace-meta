# auto-finance-assistant 部署指南

> 版本 1.6.7 · 单 EXE（含完整 Vue 前端）· Windows 原生部署

---

## 一、环境要求

### 目标机器（运行时）

| 组件 | 必需 | 说明 |
|------|------|------|
| Windows 10/11 | ✅ | 64 位 |
| Ollama | ✅ | 本地大模型服务，[下载](https://ollama.com/download) |
| Go / Node / gcc | ❌ | 不需要（EXE 已静态编译，前端已嵌入） |
| GPU（可选） | 推荐 | RTX 4060 8GB 加速推理；无 GPU 可用 CPU（较慢） |
| 内存 | ≥8GB | 推荐 16GB+ |

### 模型要求

| 用途 | CPU 验证 | 生产（RTX 4060） |
|------|----------|------------------|
| 对话 | `qwen2.5:3b` | `qwen3:4b` |
| 向量 | `nomic-embed-text` | `qwen3-embedding:0.6b` |

```bash
# CPU 验证模式
ollama pull qwen2.5:3b
ollama pull nomic-embed-text

# 生产模式
ollama pull qwen3:4b
ollama pull qwen3-embedding:0.6b
```

---

## 二、快速部署

### 方式 A：全自动部署（推荐，一条命令）

推荐直接双击部署包根目录的 **`运行部署.cmd`**（请右键选择“以管理员身份运行”）。它会自动选择 PowerShell 6+ 的 `pwsh.exe`，绕过本次执行策略，并在结束时保留窗口显示结果。

若在已打开的管理员 PowerShell 中运行，请执行：

```powershell
# CPU 验证模式（自动选盘 + 装 Ollama + 拉模型 + 配置 + 启动）
.\scripts\auto-deploy.ps1

# 生产模式（RTX 4060）
.\scripts\auto-deploy.ps1 -Production

# 指定安装到 E 盘
.\scripts\auto-deploy.ps1 -Drive E

# 只安装不启动
.\scripts\auto-deploy.ps1 -SkipStart
```

> 不建议直接双击 `.ps1`：Windows 会在脚本结束或报错后立即关闭窗口。若必须双击 `.ps1`，本版本会在结束时等待 Enter；`运行部署.cmd` 仍是更稳妥的入口。

**脚本自动完成 6 步：**
0. **选择安装盘**：优先 D 盘（`D:\AutoFinanceAssistant`），D 不可用则 C 盘；可用 `-Drive` 指定
1. 检测/下载/安装 Ollama（如未安装）
2. 启动 Ollama 服务（**模型存到选定盘** `D:\OllamaModels`，不占 C 盘）
3. 拉取对话模型 + 向量模型（带进度显示）
4. 生成 config.yaml
5. 启动 auto-finance-assistant

完成后浏览器打开 **http://127.0.0.1:8080** 即可。

> **磁盘策略**：Ollama 模型（2-5GB）默认存 C 盘用户目录，脚本自动通过 `OLLAMA_MODELS` 环境变量重定向到选定盘（如 D 盘），避免 C 盘膨胀。

> 全自动部署约需 5-15 分钟（取决于网络下载模型速度）。

### 可选：llama.cpp 后端

若不使用 Ollama，可运行独立的 llama.cpp 部署脚本。它会从当前 GitHub Release 自动选择 Windows CPU 包，或在 `-CUDA` 时选择内含 CUDA 12 运行库的包；聊天和向量模型分别启动在 8081、8082 端口。

```powershell
# CPU 验证
.\scripts\auto-deploy-llamacpp.ps1

# RTX 4060 生产部署（Qwen3 4B + CUDA）
.\scripts\auto-deploy-llamacpp.ps1 -Production -CUDA
```

也可双击 `运行部署-llamacpp.cmd` 进入 CPU 验证模式；命令行方式可追加 `-Production -CUDA`。

### 方式 B：手动部署（3 步）

#### 前置：安装 Ollama

1. 访问 https://ollama.com/download 下载 Windows 版
2. 安装后打开终端验证：`ollama serve`（保持运行）
3. 拉取模型：
   ```bash
   ollama pull qwen2.5:3b         # 对话模型
   ollama pull nomic-embed-text   # 向量模型
   ```

#### 步骤 1：解压部署包

将 `auto-finance-assistant-deploy-v1.6.7.zip` 解压到目标目录，例如：

```
C:\AutoFinanceAssistant\
├── auto-finance-assistant.exe    # 主程序（含嵌入前端）
├── config.yaml                    # 配置文件（从模板复制）
├── config.example.yaml            # 生产配置模板
├── config.dev.yaml                # CPU 验证配置模板
├── scripts\                       # 部署脚本
│   ├── start.ps1
│   ├── stop.ps1
│   ├── install-service.ps1
│   ├── uninstall-service.ps1
│   └── backup.ps1
└── data\                          # 运行时数据（自动创建）
```

### 步骤 2：生成配置

```powershell
cd C:\AutoFinanceAssistant

# 方式 A：CPU 验证（推荐先用这个测试）
Copy-Item config.dev.yaml config.yaml

# 方式 B：生产环境（RTX 4060）
Copy-Item config.example.yaml config.yaml
```

> **不创建 config.yaml 也能启动**：程序会用默认配置（qwen2.5:3b / 127.0.0.1:8080）。

### 步骤 3：启动

**前台运行（调试/验证）：**

```powershell
.\auto-finance-assistant.exe -config config.yaml run
```

浏览器打开 **http://127.0.0.1:8080** 即可使用。

**安装为 Windows 服务（生产，需管理员 PowerShell）：**

```powershell
.\auto-finance-assistant.exe -config config.yaml install
.\auto-finance-assistant.exe start
```

验证：`.\auto-finance-assistant.exe status`

---

## 三、配置说明

`config.yaml` 关键字段：

```yaml
server:
  host: 127.0.0.1     # 监听地址；局域网访问改 0.0.0.0（需设 admin_password）
  port: 8080

ollama:
  base_url: http://127.0.0.1:11434
  chat_model: qwen2.5:3b          # 生产改 qwen3:4b
  embedding_model: nomic-embed-text # 生产改 qwen3-embedding:0.6b

generation:
  num_thread: 8        # CPU 线程数（生产改用 num_gpu: 33）
  temperature: 0.3

rag:
  minimum_confidence: 0.40    # 检索置信度阈值
  high_confidence: 0.70

security:
  admin_password: ""          # 空则管理接口开放（仅 127.0.0.1 安全）
```

### CPU 验证 vs 生产

| 项 | config.dev.yaml | config.example.yaml |
|----|-----------------|---------------------|
| 模型 | qwen2.5:3b (1.9GB) | qwen3:4b (3.4GB) |
| 推理 | num_thread: 8 (CPU) | num_gpu: 33 (GPU) |
| 每次回答 | ~10-30 秒 | ~2-5 秒 |
| 超时 | 180 秒 | 90 秒 |
| 自动备份 | 关闭 | 开启 |

---

## 四、服务管理

```powershell
# 安装服务（需管理员）
.\auto-finance-assistant.exe -config config.yaml install

# 启动 / 停止 / 状态
.\auto-finance-assistant.exe start
.\auto-finance-assistant.exe stop
.\auto-finance-assistant.exe status

# 卸载服务
.\auto-finance-assistant.exe uninstall
```

或用脚本：
```powershell
.\scripts\install-service.ps1     # 安装并启动
.\scripts\stop.ps1                # 停止
.\scripts\uninstall-service.ps1   # 卸载
```

---

## 五、功能验证

### 健康检查

```bash
curl http://127.0.0.1:8080/api/health
# {"status":"ok","database":"ok","ollama":"ok","model":"qwen2.5:3b"}
```

### 知识库导入

1. 打开 http://127.0.0.1:8080 → 知识库页面
2. 上传 `.md` / `.txt` / `.docx` / `.xlsx` / `.pdf` 文档
3. 点击「发布」→ 文档进入检索
4. 点击「向量化」→ 启用语义检索

### 客服问答

在客服问答页面输入问题，如：
- "新车贷款首付多少" → RAG 检索 + grounded answer
- "你好" → 闲聊（不检索知识库）
- "保证审批通过吗" → 合规拒答

---

## 六、备份与恢复

### 手动备份

```powershell
.\scripts\backup.ps1
```

或在系统设置页面点击「立即备份」。

备份内容：
- `data/assistant.db`（SQLite 数据库）
- `config.yaml`（配置）

备份文件存放在 `data/backups/`，保留最近 7 份。

### 恢复

1. 停止服务
2. 用备份文件替换 `data/assistant.db`
3. 重启服务

---

## 七、常见问题

### Q: 启动时提示 "Ollama 不可达"

**A:** 先启动 Ollama：`ollama serve`，确认 `http://127.0.0.1:11434` 可访问。

### Q: 启动时提示缺少模型

**A:** 按提示拉取：
```bash
ollama pull qwen2.5:3b
ollama pull nomic-embed-text
```

### Q: 端口 8080 被占用

**A:** 修改 `config.yaml` 的 `server.port`，如改为 9090。

### Q: 服务安装后无法启动

**A:** 以管理员身份运行 PowerShell；检查 `config.yaml` 路径是否正确。查看 Windows 事件查看器中的服务日志。

### Q: 局域网其他机器无法访问

**A:** 
1. `config.yaml` 改 `host: 0.0.0.0`
2. 设置 `security.admin_password: "你的密码"`
3. 重启服务
4. 防火墙放行 8080 端口

### Q: 回答很慢（CPU 模式）

**A:** CPU 上 1.5B 模型每次回答 10-30 秒属正常。生产环境用 RTX 4060 + Qwen3 4B 可降至 2-5 秒。

### Q: 向量检索不工作

**A:** 需要先对文档执行「向量化」（知识库页面）。确认 `nomic-embed-text` 模型已拉取。

---

## 八、目录结构

```
AutoFinanceAssistant/
├── auto-finance-assistant.exe    # 主程序（17.5MB，含前端）
├── config.yaml                    # 配置（从模板复制）
├── config.dev.yaml                # CPU 模板
├── config.example.yaml            # 生产模板
├── DEPLOY.md                      # 本文档
├── scripts\
│   ├── start.ps1                  # 前台启动
│   ├── stop.ps1                   # 停止
│   ├── install-service.ps1        # 安装服务
│   ├── uninstall-service.ps1      # 卸载服务
│   └── backup.ps1                 # 备份
└── data\                          # 运行时自动创建
    ├── assistant.db               # SQLite 数据库
    ├── documents\                 # 上传的文档文件
    ├── logs\                      # 日志
    └── backups\                   # 备份
```

---

## 九、API 速查

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/health | 健康检查 | - |
| POST | /api/chat | 问答 | - |
| POST | /api/chat/stream | SSE 流式问答 | - |
| POST | /api/conversations | 创建会话 | - |
| GET | /api/documents | 文档列表 | - |
| POST | /api/documents | 上传文档 | ✓ |
| POST | /api/faqs | 创建 FAQ | ✓ |
| POST | /api/finance/equal-payment | 等额本息试算 | - |
| GET | /api/metrics | 运行指标 | ✓ |
| POST | /api/system/backup | 备份 | ✓ |

> 标 ✓ 的接口需在 `config.yaml` 设置 `admin_password` 后通过 `X-Admin-Password` 头认证。
