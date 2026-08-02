# API 文档

> auto-finance-assistant v2.16.0  
> 基地址：`http://127.0.0.1:8080`

---

## 一、聊天 API

### POST /api/chat

非流式问答（聚合完整回答后一次返回）。

```json
// Request
{
  "question": "新车贷款首付多少",
  "conversationId": ""  // 可选，空则自动创建会话
}

// Response
{
  "messageId": "uuid",
  "answer": "新车最低首付比例为20%...",
  "intent": "model",           // model | faq | guard_shortcut | guard_reject:xxx | compliance_refuse | refuse
  "confidence": "high",
  "durationMs": 3236,
  "promptTokens": 30,
  "completionTokens": 46,
  "sources": [],
  "requiresHuman": false,
  "traceId": "uuid"
}
```

### POST /api/chat/stream

SSE 流式问答。逐 token 推送。

```
event: status
data: {"status":"生成中","traceId":"uuid"}

event: token
data: {"token":"新车"}

event: token
data: {"token":"贷款"}

event: complete
data: {"messageId":"uuid","answer":"完整回答...","durationMs":3236}
```

---

## 二、金融计算 API

> ⚠️ 字段名使用 **camelCase**（不是 snake_case）

### POST /api/finance/equal-payment

等额本息月供计算。

```json
// Request
{
  "principal": 200000,      // 贷款本金（元）
  "annualRate": 4.5,        // 年利率（%，如 4.5 表示 4.5%）
  "months": 36              // 期数（月）
}

// Response
{
  "type": "equal-payment",
  "principal": "200,000.00",
  "annualRate": 4.5,
  "months": 36,
  "monthlyPayment": "5,918.50",    // 每月还款
  "totalPayment": "213,066.00",    // 总还款
  "totalInterest": "13,066.00",    // 总利息
  "disclaimer": "计算结果仅供参考..."
}
```

### POST /api/finance/equal-principal

等额本金月供计算（首月最高，逐月递减）。

```json
// Request（同等额本息）
{
  "principal": 200000,
  "annualRate": 4.5,
  "months": 36
}

// Response
{
  "type": "equal-principal",
  "principal": "200,000.00",
  "annualRate": 4.5,
  "months": 36,
  "firstPayment": "6,388.89",     // 首月还款
  "lastPayment": "5,574.07",      // 末月还款
  "monthlyPrincipal": "5,555.56", // 每月本金
  "totalPayment": "211,375.00",
  "totalInterest": "11,375.00",
  "disclaimer": "计算结果仅供参考..."
}
```

### POST /api/finance/down-payment

首付计算。

```json
// Request
{
  "vehiclePrice": 200000,     // 车价（元）
  "downPaymentPct": 0.3       // 首付比例（0.3 = 30%）
}

// Response
{
  "downPayment": "60,000.00",     // 首付金额
  "loanPrincipal": "140,000.00",  // 贷款本金
  "vehiclePrice": "200,000.00",
  "downPaymentPct": 0.3
}
```

---

## 三、会话管理 API

### POST /api/conversations

```json
// Request
{ "title": "贷款咨询" }

// Response
{ "id": "uuid", "title": "贷款咨询", "createdAt": "...", "updatedAt": "..." }
```

### GET /api/conversations

```json
{ "items": [...], "total": 10 }
```

### GET /api/conversations/{id}

返回会话详情 + 消息列表。

### DELETE /api/conversations/{id} （需认证）

删除会话及其所有消息（PIPL 被遗忘权）。

---

## 四、管理 API（需认证）

认证方式：`X-Admin-Password` 头 或 `Authorization: Bearer xxx`。

### 知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/documents | 上传文档 |
| GET | /api/documents | 文档列表 |
| GET | /api/documents/{id} | 文档详情 |
| PUT | /api/documents/{id} | 更新元数据 |
| DELETE | /api/documents/{id} | 删除文档 |
| POST | /api/documents/{id}/publish | 发布文档 |
| POST | /api/documents/{id}/embed | 向量化 |

### FAQ

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/faqs | 创建 FAQ |
| GET | /api/faqs | FAQ 列表 |
| PUT | /api/faqs/{id} | 更新 FAQ |
| DELETE | /api/faqs/{id} | 删除 FAQ |
| POST | /api/faqs/test | 测试匹配 |

### 合规与审计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/compliance/logs?type=guard_block | 合规日志 |
| GET | /api/compliance/stats | 合规统计（拦截率） |
| GET | /api/audit/logs | 管理审计日志 |
| GET | /api/feedback | 用户反馈 |
| GET | /api/refused | 拒答列表 |
| GET | /api/metrics | 运行指标 |

### 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/health | 健康检查（开放） |
| GET | /api/system/info | 系统信息 |
| POST | /api/system/backup | 手动备份 |
| POST | /api/system/purge?days=90 | 手动清理过期数据 |

---

## 五、错误响应

```json
{
  "error": {
    "code": "unauthorized",
    "message": "管理员认证失败"
  }
}
```

| HTTP 状态 | code | 说明 |
|-----------|------|------|
| 400 | invalid_body | 请求体格式错误 |
| 401 | unauthorized | 认证失败 |
| 429 | rate_limited | 请求过于频繁 |
| 503 | ollama_unavailable | 模型服务不可用 |

---

*auto-finance-assistant v2.16.0 API 文档*
