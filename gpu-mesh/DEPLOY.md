# gpu-mesh 部署手册

> 上百台异地 Windows 4060 机器的完整部署方案。从单台 Relay 到百台 Agent 纳管的端到端操作。

## 架构回顾

```
                         ┌─────────────────────┐
                         │   公网 VPS (Relay)   │
                         │   静态IP + 域名       │
                         └──────────┬──────────┘
                                    │ 反向 WS 长连接
            ┌───────────┬───────────┼───────────┬───────────┐
            ▼           ▼           ▼           ▼           ▼
        Agent 北京   Agent 上海   Agent 广州   Agent 成都   ...百台
         4060 8GB    4060 8GB     4060 8GB     4060 8GB
```

**核心特性**：Agent 主动外连穿透 NAT，无需公网 IP；节点间不直接通信，所有协调经 Relay。

## 部署清单

| 组件 | 数量 | 部署方式 | 脚本 |
|------|------|---------|------|
| Relay | 1 台公网 VPS | systemd 或 Docker | `deploy/relay-init.sh` |
| Nginx (可选) | 同 VPS | 反代 + TLS | `deploy/nginx/gpu-mesh-nginx.conf` |
| Ollama 镜像 | 1 台内网机 | 预拉模型 | `deploy/ollama-mirror.md` |
| Agent | ×100 | Windows 服务 | `scripts/install-agent.ps1` / 批量脚本 |

---

## 一、Relay 部署（公网 VPS）

### 方式 A：systemd 直装（推荐生产）

```bash
# SSH 到 VPS，上传代码后执行
sudo bash deploy/relay-init.sh
```

脚本自动完成：建用户 → 编译/安装二进制 → 生成 token → 装 systemd 服务 → 启动 + 健康检查。

输出示例：
```
✓ GPU Mesh Relay 部署完成
控制台:    http://1.2.3.4:7780/?token=abc123...
Agent 接入: ws://1.2.3.4:7780/agent
```

### 方式 B：Docker Compose

```bash
cd deploy/docker
echo "GPU_MESH_TOKEN=$(openssl rand -hex 32)" > .env
docker compose up -d
docker compose logs -f
```

带 Nginx + TLS：
```bash
docker compose --profile with-nginx up -d
```

### 管理

```bash
systemctl status gpu-mesh-relay    # 状态
systemctl restart gpu-mesh-relay   # 重启
journalctl -u gpu-mesh-relay -f    # 日志
```

---

## 二、Nginx 反代 + TLS（生产强烈建议）

让 Relay 走 `wss://`（加密），防止心跳和推理数据明文传输。

```bash
# 1. 装 Nginx + Certbot
sudo apt install nginx certbot python3-certbot-nginx

# 2. 配置
sudo cp deploy/nginx/gpu-mesh-nginx.conf /etc/nginx/conf.d/
# 编辑 server_name 为你的域名，DNS A 记录指向 VPS
sudo nano /etc/nginx/conf.d/gpu-mesh-nginx.conf

# 3. 申请 TLS 证书（Let's Encrypt 免费）
sudo certbot --nginx -d gpu-mesh.yourdomain.com

# 4. 重载
sudo nginx -t && sudo systemctl reload nginx
```

配置要点（已内置）：
- WS 长连接 `proxy_read_timeout 86400s`（24h 不中断）
- SSE `proxy_buffering off`（流式推理实时推送）
- 强制 HTTPS（80 重定向 443）

---

## 三、Ollama 内网镜像（百台规模必做）

百台机器各自从公网拉 4GB 模型会打满带宽。搭 1 台内网镜像旁路。

详见 [`deploy/ollama-mirror.md`](./deploy/ollama-mirror.md)。

核心：1 台内网机预拉模型 → Agent 的 `OLLAMA_HOST` 指向它 → 拉模型走内网。

---

## 四、Agent 部署（每台 4060）

### 单台部署

每台机器管理员 PowerShell 执行：
```powershell
.\scripts\install-agent.ps1 -Relay "wss://gpu-mesh.yourdomain.com" `
  -Token "abc123..." -AgentID "gpu-bj-001"
```

脚本自动：装服务 → 开机自启 → 启动 → 验证上线。

### 批量部署（上百台）

```powershell
# 1. 准备主机清单
copy deploy\playbook\agents.csv.example deploy\playbook\agents.csv
# 编辑 agents.csv 填入 host/user/password/agent_id

# 2. 批量执行
.\deploy\playbook\batch-install-agent.ps1 `
  -Csv deploy\playbook\agents.csv `
  -Relay "wss://gpu-mesh.yourdomain.com" `
  -Token "abc123..."
```

输出汇总表 + 导出 CSV 结果。

### Agent 运维

```powershell
# 常用
Start-Service gpu-mesh-agent       # 启动
Stop-Service gpu-mesh-agent        # 停止（优雅退出，等待在途任务）
Restart-Service gpu-mesh-agent     # 重启
& "C:\Program Files\gpu-mesh\gpu-mesh-agent.exe" uninstall  # 卸载
```

**Agent 日志位置**（服务模式，按天轮转，保留 7 天）：
```
C:\ProgramData\gpu-mesh\logs\agent-YYYY-MM-DD.log
```
```powershell
# 看实时日志
Get-Content "C:\ProgramData\gpu-mesh\logs\agent-$(Get-Date -Format yyyy-MM-dd).log" -Tail 50 -Wait

# 查今天的日志
Get-Content "C:\ProgramData\gpu-mesh\logs\agent-$(Get-Date -Format yyyy-MM-dd).log"
```

---

## 五、验收清单

部署完成后逐项验证：

### 基础连通性
- [ ] 打开 `https://gpu-mesh.yourdomain.com/`（带 token），控制台加载正常
- [ ] `curl https://域名/healthz` 返回 `{"relay":"ok","store":"ok","agents_online":N,...}`（深度健康检查）
- [ ] 控制台显示所有在线 Agent（数量与部署一致）
- [ ] 每台 Agent 的 GPU 利用率/显存/温度有数值（证明 nvidia-smi 采集正常）
- [ ] 引擎列显示 `ollama`（证明引擎探测正常）

### 推理功能
- [ ] 控制台"推理测试"面板：选模型 → 输入问题 → 收到回复
- [ ] 流式推理：勾选"流式" → 看到逐 token 打字机效果
- [ ] 命令行验证：`curl -H "Authorization: Bearer <token>" https://域名/v1/chat/completions -d '{"model":"qwen2.5:7b","messages":[{"role":"user","content":"hi"}]}'`

### 让位机制（核心特性）
- [ ] 在某台机器上动鼠标/键盘 → 控制台 5s 内该机器让位状态变 `busy_human`
- [ ] 该机器让位后，新推理请求不再派给它（派给其他 idle 节点）
- [ ] 用户停止使用 5 分钟后 → 状态恢复 `idle`

### 高可用
- [ ] 关掉一台 Agent → 控制台 45s 内标记离线
- [ ] 告警 webhook 收到离线通知（若配置了）
- [ ] 该 Agent 重启后自动重连上线

### 批量与训练
- [ ] 控制台"批量提交"面板：输入多行 → 提交 → 看到分片进度 → 结果聚合
- [ ] 提交训练作业 → 状态显示 queued → 等 idle 节点 → running（需 Python 环境）

---

## 六、生产加固清单

### 安全
- [ ] Relay 强制 `-token`（随机 32 字节）
- [ ] 启用 mTLS（`-mtls`，见 `internal/mtls`）
- [ ] Nginx TLS（wss/https）
- [ ] 防火墙只放行 443/7780
- [ ] 控制台加 IP 白名单
- [ ] 审计日志开启（`-audit`）

### 可靠性
- [ ] Relay systemd `Restart=always`
- [ ] Agent Windows 服务"恢复"策略设为自动重启
- [ ] 数据目录定时备份（`/var/lib/gpu-mesh/relay.db`）
- [ ] 配置告警 webhook（钉钉/飞书）

### 性能（百台规模）
- [ ] Ollama 内网镜像（避免公网拉模型）
- [ ] 热门模型预加载（控制台或 API 触发 pull 到多个 idle 节点）
- [ ] 监控 `/api/metrics`（推理 QPS/延迟/让位 NACK）

---

## 七、故障排查

### Agent 连不上 Relay
```powershell
# 1. 确认网络
Test-NetConnection gpu-mesh.yourdomain.com -Port 7780

# 2. 看服务日志
Get-EventLog -LogName Application -Source gpu-mesh-agent -Newest 20 | Format-List

# 3. 常见原因：
#    - 代理软件干扰（已自动禁用，若仍有问题检查 HTTP_PROXY）
#    - token 不匹配
#    - 防火墙未放行
```

### GPU 不显示利用率
```bash
# Agent 机器上检查
nvidia-smi  # 必须有输出
# 若无，装 NVIDIA 驱动
```

### 推理超时
- 看 `/api/metrics` 的 `inference_avg_ms`
- 模型太大（7B 在 4060 上首次加载慢）
- 让位状态导致（BUSY 节点推理慢）
- Ollama 服务未启动

### 控制台看不到机器
- 确认 Agent 服务在运行（`Get-Service gpu-mesh-agent`）
- 确认心跳间隔（默认 5s），等 10s 刷新
- 看 Relay 日志 `journalctl -u gpu-mesh-relay | grep "Agent.*上线"`

---

## 附录：网络拓扑决策

为什么用星型反向 WS 而非其他：

| 方案 | 可行性 | 原因 |
|------|--------|------|
| P2P 互联 | ❌ | NAT 后无法直连 |
| Relay 主动连 Agent | ❌ | Agent 无公网 IP |
| **反向 WS 星型** ✅ | ✅ | Agent 主动外连穿透 NAT |

节点间**永不直接通信**，所有协调经 Relay——这是百台异地机器可靠协作的唯一方案。
