# Ollama 内网镜像部署指南

## 为什么需要

百台机器各自从公网拉同一个 4GB 模型，会：
- 占满出口带宽（100 台 × 4GB = 400GB 流量）
- 速度慢（受限于 ollama.com 国外节点）
- 部分机器可能拉取失败

**方案**：在一台内网机器搭 Ollama 镜像，所有 Agent 从内网拉。

## 搭建步骤（镜像机，1 台内网 Linux/Windows）

### 1. 安装 Ollama
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### 2. 预拉常用模型（一次性，从公网拉）
```bash
ollama pull qwen2.5:7b
ollama pull qwen2.5:3b
ollama pull nomic-embed-text
# 按需拉取其他
```

### 3. 配置为镜像模式（允许其他机器从本机拉模型）
```bash
# Ollama 0.3.0+ 支持 OLLAMA_ORIGINS 控制 CORS
# 编辑 /etc/systemd/system/ollama.service.d/override.conf
sudo mkdir -p /etc/systemd/system/ollama.service.d
sudo tee /etc/systemd/system/ollama.service.d/override.conf <<EOF
[Service]
Environment="OLLAMA_HOST=0.0.0.0:11434"
Environment="OLLAMA_ORIGINS=*"
EOF
sudo systemctl daemon-reload
sudo systemctl restart ollama
```

验证镜像机可达：
```bash
curl http://镜像机IP:11434/api/tags
```

## Agent 机器配置（每台 4060）

### 方式 A：Ollama 指向镜像（推荐）

每台 Agent 机器的 Ollama 拉模型时从镜像拉：
```bash
# 设置环境变量指向镜像（写入系统环境变量持久化）
setx OLLAMA_HOST "http://镜像机IP:11434" /M
# 重启 Ollama 服务后生效
```

这样 Agent 机器的 `ollama pull` 实际是从内网镜像拉，速度快、不占公网出口。

### 方式 B：用 gpu-mesh 的 pull API 统一分发

通过 Relay 控制台或 API 触发：
```bash
# 让所有有 ollama 引擎的 Agent 拉模型（经 Relay 下发指令，但数据从镜像拉）
curl -X POST https://gpu-mesh.yourdomain.com/api/models/pull \
  -H "Authorization: Bearer <token>" \
  -d '{"engine":"ollama","model":"qwen2.5:7b"}'
```

## 验证

在 Agent 机器上检查模型是否可用：
```bash
ollama list
ollama run qwen2.5:7b "hello"
```

在 gpu-mesh 控制台 `/v1/models` 应能看到已加载的模型列表。

## 模型选择建议（4060 8GB）

| 模型 | 大小 | 8GB 可跑 | 适用场景 |
|------|------|---------|---------|
| `qwen2.5:7b` | 4.4GB | ✅ Q4量化 | 通用对话主力 |
| `qwen2.5:3b` | 1.9GB | ✅ 轻松 | 高并发低成本 |
| `qwen2.5:1.5b` | 1GB | ✅ | 超轻量/批处理 |
| `deepseek-r1:7b` | 4.7GB | ✅ | 推理任务 |
| `nomic-embed-text` | 274MB | ✅ | 嵌入/RAG |
| `qwen2.5:14b` | 9GB | ❌ 超显存 | 不适用 8GB |

**推荐组合**：`qwen2.5:7b`（对话）+ `nomic-embed-text`（嵌入），总占用约 5GB，留 3GB 给推理上下文。
