# 备案材料清单总表

> auto-finance-assistant 大模型合规备案材料汇总  
> 最后更新：2026-07-27 | 版本：v2.9.3

---

## 一、材料总览

| 序号 | 文档名称 | 文件路径 | 状态 | 备注 |
|------|---------|---------|------|------|
| 1 | 合规研究报告 | `docs/COMPLIANCE_REPORT.md` | ✅ 已完成 | 初版合规分析 |
| 2 | 深度研究报告 | `docs/COMPLIANCE_DEEP_RESEARCH.md` | ✅ 已完成 | 含政策动态+技术自检 |
| 3 | 仅推理承诺书 | `docs/INFERENCE_ONLY_COMMITMENT.md` | ✅ 已完成 | 技术定性核心文件 |
| 4 | 架构白皮书 | `docs/ARCHITECTURE_WHITEPAPER.md` | ✅ 已完成 | 三层架构+推理链路 |
| 5 | 安全自评估报告 | `docs/SECURITY_ASSESSMENT.md` | ✅ 已完成 | 对标GB/T 45654，21项 |
| 6 | 隐私政策+用户协议 | `docs/PRIVACY_POLICY.md` | ✅ 已完成 | PIPL合规 |
| 7 | 运维+应急制度 | `docs/OPS_INCIDENT_RESPONSE.md` | ✅ 已完成 | P0-P3应急流程 |
| 8 | 网信办咨询函 | `docs/CONSULTATION_LETTER.md` | ✅ 已完成 | 3个核心问题 |
| 9 | 部署对比表 | `docs/DEPLOYMENT_COMPARISON.md` | ✅ 已完成 | 管理层决策参考 |
| — | 本清单 | `docs/FILING_CHECKLIST.md` | ✅ 已完成 | 本文档 |

---

## 二、技术合规整改记录

| # | 整改项 | 改动文件 | 版本 | 状态 |
|---|--------|---------|------|------|
| 1 | 审计日志中间件 | `api/audit.go` (新) | v2.9.0 | ✅ |
| 2 | 14类审计事件接入 | `api/router.go` `chat/service.go` | v2.9.0 | ✅ |
| 3 | 数据保留TTL清理 | `storage/retention.go` (新) | v2.9.0 | ✅ |
| 4 | 安全响应头 | `api/security_headers.go` (新) | v2.9.0 | ✅ |
| 5 | 频率限制 | `api/rate_limit.go` (新) | v2.9.0 | ✅ |
| 6 | 敏感内容输出检测 | `chat/guard.go` | v2.9.0 | ✅ |
| 7 | 合规拒答审计 | `chat/service.go` | v2.9.1 | ✅ |
| 8 | 模型调用审计 | `chat/service.go` | v2.9.1 | ✅ |
| 9 | 日志保留60→180天 | `config/defaults.go` | v2.9.3 | ✅ |
| 10 | 前端AI内容标识 | `web/views/ChatView.vue` | v2.9.3 | ✅ |

---

## 三、模型基础信息（核实后）

| 项目 | 内容 | 来源 |
|------|------|------|
| 官方型号 | **Qwen3-4B**（不是 Qwen3.5） | [HuggingFace Qwen/Qwen3-4B](https://huggingface.co/Qwen/Qwen3-4B) |
| 参数量 | 4.0 Billion | 同上 |
| 发布日期 | 2025-04-27 | 同上 |
| 开源授权 | Apache License 2.0 | 同上 |
| 备案主体 | 阿里巴巴达摩院（杭州）科技有限公司 | [阿里云帮助文档](https://help.aliyun.com/zh/model-studio/compliance-and-launch-faq-for-ai-apps-powered-by-the-tongyi-model) |
| 生成式AI备案号 | ZheJiang-TongYiQianWen-20230901 | 网信办公告 |
| 算法备案号 | 网信算备330110507206401230027号 | 同上 |
| 权重哈希(SHA-256) | `00fe7986ff5f6b463e62455821146049db6f9313603938a70800d1fb69ef11a4` | 本地计算 |

---

## 四、GB/T 45654-2025 合规度

| 维度 | 评估项数 | 符合 | 合规率 |
|------|---------|------|--------|
| 训练数据安全 | 3 | 3（豁免） | 100% |
| 模型安全 | 4 | 4 | 100% |
| 内容安全措施 | 6 | 6 | 100% |
| 安全评估 | 3 | 3 | 100% |
| 数据安全 | 5 | 5 | 100% |
| **合计** | **21** | **21** | **100%** |

---

## 五、评测验证记录

| 评测时间 | 版本 | 通过率 | 合格率 | 报告 |
|---------|------|--------|--------|------|
| 2026-07-27 | v2.9.3 | 95% A+ | ≥90% ✅ | `reports/eval-latest.md` |

---

## 六、待办事项

| # | 事项 | 负责人 | 截止 | 状态 |
|---|------|--------|------|------|
| 1 | 代码中模型名 "Qwen3.5" → "Qwen3-4B" 全量修正 | 开发 | 本周 | ⬜ |
| 2 | 发送咨询函至属地网信办 | 法务 | 2周内 | ⬜ |
| 3 | 等保三级测评（如尚未取得） | 运维 | 1月内 | ⬜ |
| 4 | 根据网信办回复决定是否正式备案 | 管理层 | 1月内 | ⬜ |

---

*本清单随备案进度持续更新。*
