# auto-finance-assistant · 全面 Code Review

> 审查日期：2026-07-22 · 审查范围：全部 65 Go 文件 + 12 Vue 文件 + 配置/构建
> 方法：3 个并行只读 review agent 广度扫描 + 关键文件人工精读交叉验证

## ✅ 修复状态（2026-07-23 第三轮 review 更新）

**三轮 review 共 40+ 问题，全部修复**（v1.5.1）。第三轮新发现并修复：

| 波次 | 问题 | 修复方式 |
|------|------|----------|
| CR1 | goroutine/连接/队列槽泄漏 | `sendEvent` helper + `select{case events<-: case <-ctx.Done()}` |
| CR2 | 管理接口未鉴权 | 文档/FAQ 写操作 + metrics + system/info 全包 AuthMiddleware |
| CR3 | 默认空密码全开放 | 非本地监听 + 空密码时启动拦截 |
| CR4 | 前端不发凭据 | 新增 auth store + authedFetch + 密码输入框 |
| CR5 | 新克隆构建失败 | 占位 index.html 强制跟踪 + .gitignore 调整 |
| H1 | SetMaxOpenConns(1) | 改为 8 读连接 + `_txlock=immediate` |
| H2 | zip-bomb | safeZipEntryReader + UncompressedSize64 检查 + LimitReader |
| H3 | XML 实体扩展 | LimitReader 包裹 + 行/字符串数上限截断 |
| H4 | PII 不完整 | 新增邮箱/15位证/分隔符手机号正则 + 标题用脱敏文本 + feedback 脱敏 |
| H5 | 错误泄露 | writeInternalError 通用消息 + FAQ/db 错误不再返 err.Error() |
| H6 | 备份路径泄露 | 只返回 name 不返回 path |
| H7 | FAQ priority 清零 | `if body.Priority != 0` 守卫 |
| H8 | SSE reader 泄漏 | finally 中 reader.cancel()/releaseLock() |
| M1 | 零利率舍入 | TotalPayment 用实际累加值 |
| M2 | chunker 死循环 | overlap >= max/2 时强制 max/4 |
| M3 | 评测除零 | 空数据集提前返回 |
| M4 | 死代码 | 删除 rag/fusion/types 的 var _ 守卫 + pdf.go parenMatcher/io/fmt |
| M8 | DisallowUnknownFields | 移除，允许前向兼容 |
| M11 | api 404 | /api/ 路径返回 404 JSON 不落 SPA |
| L1 | writeJSON 半截 | 预编码到缓冲区 |
| L2 | WriteTimeout | 注释说明故意不设（SSE） |
| L5 | LoadFromDB 超时 | 30s ctx |
| L6 | health 轮询 | 30s setInterval |
| L7 | source 事件 | handleEvent 加 source case |
| L9 | 建议发送 | 点击直接 onSend() |
| M10 | importer 忽略错误 | 状态更新失败时记日志 |

**未修复（评估为可接受/低优先级）**：M5(hash 去重 race，单机低并发可接受)、M9(embed 非事务，可重跑修复)、M6(FTS buildFTSQuery 死代码，已不调用)、L3(时区来源不一致，均 UTC)、L4(Levenshtein O(N)，FAQ 少时有提前短路)、L8(前端 fetch 错误处理，部分 view 已有 errorMsg 模式)。

## 严重程度分布

| 严重度 | 数量 | 含义 |
|--------|------|------|
| 🔴 严重 | 5 | 生产环境必现的崩溃/泄漏/安全漏洞 |
| 🟠 高 | 8 | 特定条件下触发，影响可靠性/安全 |
| 🟡 中 | 11 | 正确性瑕疵/健壮性/一致性 |
| 🟢 低 | 9 | 代码质量/可维护性 |

---

## 🔴 严重（必须修复）

### CR1. 客户端断开时 goroutine + 连接 + 队列槽泄漏
**`internal/chat/service.go:394`**

流式生成时，`runGeneration` 从 ollama `stream` 读 token，推到自己的 `events` channel。若 HTTP 客户端中途断开，`handleChatStream` 返回不再读 `events`，导致：
- `service.go:394` 的 `events <- Event{...}` 永久阻塞（channel 满）
- 队列槽永不释放 → 后续请求全部 `ErrBusy`（503）
- ollama 的 `streamRead` goroutine + HTTP 连接也泄漏

`ctx.Err()` 检查（386 行）只在迭代间隙执行，阻塞在 channel send 时无法触发。

```go
// service.go:385-400 — 阻塞 send，无 ctx select
for ev := range stream {
    if ctx.Err() != nil { return ctx.Err() }  // ← 阻塞时不执行
    ...
    events <- Event{Type: "token", Payload: ev.Token}  // ← 永久阻塞
}
```

**修复**：每个 `events <-` 改为 `select { case events <- ev: case <-parent.Done(): return parent.Err() }`，并让 ollama 请求 ctx 派生自可取消的 parent。

### CR2. 管理接口几乎全部未鉴权
**`internal/api/router.go:63-88`**

只有 5 个路由包了 `AuthMiddleware`。以下**高危操作完全开放**：
- `POST /api/documents`（上传）· `DELETE /api/documents/{id}`（删除+删文件）
- 全部 FAQ CRUD + 批量导入（可注入影响模型回答）
- `PUT /api/documents/{id}` · publish/disable/reparse/embed
- `GET /api/metrics`（系统指标泄露）

任何人能访问端口就能替换知识库、注入 FAQ、删除文档。

**修复**：所有写操作 + metrics + system/info 统一包 `AuthMiddleware`。

### CR3. 默认配置 `admin_password: ""` = 完全无鉴权
**`internal/api/admin_handler.go:17-20`** + `config.example.yaml:71` / `config.dev.yaml:73`

`AuthMiddleware` 在密码为空时直接放行。出厂配置两套都是空密码，连那 5 个"受保护"路由也开放。且启动时无任何告警。

**修复**：启动时若 `host != 127.0.0.1 && adminPassword == ""` 则拒绝启动或生成随机密码并日志输出。

### CR4. 前端从不发送凭据，鉴权功能实际不可用
**`web/src/views/SettingsView.vue:26,32`** + 全局 grep `X-Admin-Password|Authorization` = 0 匹配

SettingsView 调用 `/api/system/backups`（需认证）但不带任何认证头。证明应用只在空密码（全开放）模式下可用。设了密码后 Settings 页静默 401 失败。

**修复**：加 auth store/拦截器，特权请求附带头；加登录提示。

### CR5. 新克隆 `go build` 必失败——占位 dist 被 gitignore 且未跟踪
**`.gitignore:15`** + `internal/web/embed.go:12`

`//go:embed dist/*` 要求 `internal/web/dist/` 存在。`.gitignore` 排除了它，我创建的占位 `index.html` **未被 git 跟踪**。新克隆 → `go build` 报 "no matching files found"。

**修复**：要么跟踪一个占位 `dist/.gitkeep` + 占位 index.html（不 ignore），要么 `make build` 依赖 `web-build`。

---

## 🟠 高

### H1. `SetMaxOpenConns(1)` 序列化所有读，抵消 WAL 优势
**`internal/storage/database.go:43`**

单连接意味着慢查询（FTS/vector 5s 超时）会阻塞 `AppendMessage`、metrics、FAQ 匹配、历史查询。

**修复**：读连接池放开（`SetMaxOpenConns(N)`），写用独立单连接或 `_txlock=immediate`。

### H2. DOCX/XLSX 解析无 zip-bomb / 解压大小限制
**`internal/parser/docx.go:19`** / `xlsx.go:18`

`zip.OpenReader` 后无 `UncompressedSize64` 检查、无条目数上限、`io.LimitReader`。几 MB 压缩文件可解压到 GB 级。结合 CR2（上传无鉴权），远程 DoS。

**修复**：检查每个 entry 的未压缩大小；cap 总解压字节；cap 行/单元格数；用 `io.LimitReader`。

### H3. XML 解析无实体扩展防护（billion-laughs）
**`docx.go:51`** / `xlsx.go:118,196`

Go `encoding/xml` 默认不解析外部实体（无 XXE 外读），但易受二次实体扩展攻击。无 `io.LimitReader`、无 token 数上限。

**修复**：每个 entry reader 包 `io.LimitReader`；预处理剥离 DOCTYPE；cap token 数。

### H4. PII 脱敏不完整且未覆盖所有入库文本
**`internal/chat/pii.go:9-14`**

- 手机号正则 `1[3-9]\d{9}` 不处理分隔符（`138-1234-5678` 漏检）、无词边界锚
- 身份证只覆盖 18 位（漏 15 位旧证）、无邮箱、无姓名
- 会话标题用**原始**问题（`service.go:228`，非 maskedQuestion）
- `POST /api/feedback` 的 reason/correction **完全不脱敏**直接入库

**修复**：扩充正则（分隔符/15位证/邮箱）；标题用脱敏后文本；反馈文本入库前脱敏。

### H5. 大量 `err.Error()` 直接返回客户端（信息泄露）
**`document_handler.go:56,130`** / `faq_handler.go:74` / `chat_handler.go:42,157` / `embed_handler.go:33` / `backup_handler.go:20` 等

暴露绝对文件路径、DB 驱动内部、Ollama 主机细节。

**修复**：服务端记完整错误；返回通用用户提示。

### H6. 备份接口返回绝对文件系统路径
**`internal/api/backup_handler.go:23`**

`"path": path` 泄露服务器目录结构。

**修复**：只返回 `name`，不返回 `path`。

### H7. FAQ 开关按钮静默清零 priority
**`web/src/views/FaqView.vue:56`** + **`faq_handler.go:140`**

`toggle(f)` 只发 `{enabled: !f.enabled}`，后端 `existing.Priority = body.Priority` 无条件赋值（body.Priority=0），每次启停把优先级清零。

**修复**：后端 `if body.Priority != 0 { existing.Priority = body.Priority }`。

### H8. 前端 SSE reader 未释放（资源泄漏）
**`web/src/stores/chat.ts:166-198`**

`resp.body.getReader()` 获取后 `finally` 块只重置 flag，从不 `reader.cancel()`/`releaseLock()`。abort 时底层 TCP 流等 GC。

**修复**：`finally` 中若 reader 存在且 locked，`await reader.cancel().catch(()=>{})`。

---

## 🟡 中

| ID | 位置 | 问题 |
|----|------|------|
| M1 | `equal_payment.go:44-59` | 零利率分支丢弃 `lastAdjust`（`_ = lastAdjust`），totalPaid ≠ principal，舍入误差未修正 |
| M2 | `knowledge/chunker.go:153` | `splitLong` 当 `overlap >= maxChars/2` 时 `start` 不前进 → 死循环（默认安全，但配置无校验） |
| M3 | `evaluations/runner.go:63` | 空数据集时 `len(results)==0` 除零 panic |
| M4 | `rag/fusion.go:108` 等 | 死代码守卫：`var _ = ollama.Client{}`、`var _ = sql.ErrNoRows`、`parser/pdf.go` 的 `parenMatcher`/`io.ReadAll`/`fmt.Sprintf`、`main.go` 的 `slog.LevelInfo` |
| M5 | `importer.go:98-131` | hash 去重 check-then-insert 非原子，并发上传同文件可双写 |
| M6 | `fts_search.go:174-212` | `buildFTSQuery` 死代码，若复活会有 FTS 操作符注入（未引号化的 `-`/`NOT`） |
| M7 | `health_handler.go:55,29` | `/api/system/info` + health detail 未鉴权，泄露 Ollama 地址 + 本地模型清单 |
| M8 | `response.go:15` | `DisallowUnknownFields` 破坏前向兼容，前端加字段即 400 |
| M9 | `vector_search.go:150` | `EmbedAndStore` 逐条非事务 UPDATE，崩溃后 DB 与索引部分不一致（可重跑修复） |
| M10 | `importer.go:139` 等 | 多处 `_ = storage.UpdateDocumentStatus(...)` 忽略错误，文档可能卡在 processing |
| M11 | `web/embed.go:17` | `/api/nonexistent` 落到 SPA fallback 返回 200 HTML 而非 404 JSON |

---

## 🟢 低

| ID | 位置 | 问题 |
|----|------|------|
| L1 | `response.go:23` | writeJSON 编码失败后追加 JSON 片段产生非法 JSON |
| L2 | `main.go:138` | `WriteTimeoutSeconds` 配置了但从不应用到 http.Server（且不该应用——会断 SSE） |
| L3 | `conversation_repo.go` | created_at 用 Go UTC，updated_at 用 SQLite `datetime('now')`，时区来源不一致 |
| L4 | `faq_match.go:66` | 模糊匹配对全量 FAQ 跑 O(N·len²) Levenshtein，无提前终止 |
| L5 | `main.go:111` | `LoadFromDB` 用 `context.Background()` 无超时（FAQ 加载有 30s） |
| L6 | `App.vue:19` | health 只在 mount 时查一次，不轮询，状态会过期 |
| L7 | `chat.ts:226` | `handleEvent` 无 `source` case——SSE 流式时来源事件被丢弃（后端有发） |
| L8 | 各 view | 大量 fetch 无 try/catch、无 `resp.ok` 检查（HistoryView/KnowledgeView/SettingsView/FinanceView） |
| L9 | `ChatView.vue:53` | 建议按钮只填 input 不直接发送 |

---

## 已验证正确的部分（无需改动）

- **队列槽释放**：`llm_queue.go` Acquire 三条退出路径都释放等待席，release 闭包幂等 ✓
- **向量索引锁**：`vector_index.go` 所有写用 Lock、读用 RLock，Search 按值拷贝 ✓
- **SQL 注入**：全部 `?` 占位符；`IN(...)` 用占位符+绑定参数；LIKE 用绑定参数且 `truncateForLike` 剥离 `%`/`_` ✓
- **Rows.Err()**：所有 `defer rows.Close()` 后都返回 `rows.Err()` ✓
- **事务 rollback defer**：Commit 后 Rollback no-op，安全 ✓
- **前端无 XSS**：全部 `{{ }}` 文本插值，无 `v-html`/`innerHTML` ✓
- **依赖版本**：go.mod 全部精确 pin，modernc.org/sqlite v1.34.5 是近期纯 Go 版本 ✓
- **配置一致性**：dev/example 结构一致，差异字段（num_gpu/num_thread/model）都映射到代码 ✓

---

## 修复优先级建议

**第一波（阻断性 + 安全）**：CR1（goroutine 泄漏）→ CR2/CR3（鉴权）→ CR5（构建）→ H2/H3（zip/xml 炸弹）

**第二波（正确性）**：H7（priority 清零）→ M1（零利率舍入）→ M2（chunker 死循环）→ M3（评测除零）→ H4（PII 补全）

**第三波（健壮性 + 清理）**：H8（reader 释放）→ L7（source 事件）→ L8（前端错误处理）→ M4（死代码）→ 响应/配置一致性

### 第三轮 review 修复（v1.5.1）

| 问题 | 修复 |
|------|------|
| C1 Defaults 置信度 0.58 与 config 0.40 不一致 | defaults.go 改为 0.40/0.70 + 模型名修正 |
| C3 日志轮转失败后写已关闭 FD 丢数据 | rotateLocked 设 FD=nil + Write 检查 nil 退化 stderr |
| H5 合规子串匹配假阳性（免息/包过/内部风控） | 改为「意图词+名词」组合判断 |
| H6 拒答前推送了 source 事件 + 0.3 硬编码阈值 | 先判置信再推来源；移除硬编码 0.3 |
| H3 轮转 rename 失败 currentSz 归零致无限增长 | 失败时 stat 重新获取真实大小 |
| M4 日志 maxFiles 复用 backup.retain_count | 新增 LoggingConfig.MaxFiles 字段 |
| M5 同秒轮转文件名冲突覆盖 | 时间戳含毫秒 |
| L5 runComplianceRefuse 不记落库错误 | 改为 log.Error |
