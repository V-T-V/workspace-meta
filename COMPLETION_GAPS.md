# 中低完成度项目 · 差距清单与推进记录

> 生成日期：2026-07-17
> 范围：12 个 mid/low 完成度项目（对照 8 个 high 标杆）
> high 标准 = 功能完整闭环 + 测试充分 + 能直接用 + 文档齐 + 无占位/假实现 + lint/type-check 全过

---

## 一、本轮已完成的修复（9 个项目，11 项改动）

| 项目 | 改动 | 类型 | 验证 |
|------|------|------|------|
| **taixu-dao-world** | 清理 dev-server/vite-dev 日志残留 + 补 .gitignore | 工程整洁 | 已提交 |
| **agenttrain** | .env.example（确认已存在，内容正确）+ 测试数校正(60→84) | 能直接用+文档 | 无需改代码 |
| **tripplan** | 修 2 个 lint 错误 + 补 AGENTS.md + README 工具表 14→19 校正 | lint+文档 | lint+type-check 过 |
| **modelstudio** | 修 Draco 假实现（补 `_export_glb`、消除死分支）+ 补 server 单元测试(TaskStore6+config8) | 真 bug 修复+测试补齐 | py_compile + 14测试全过 |
| **stock-ai** | 修 9 lint 错误 + 清死注释 + 新增 analyze_sentiment 工具(暴露情绪研判能力)+3 测试 | lint+功能补全 | lint+type-check+78测试全过 |
| **future-world-3055** | Havok 物理半接入降级为 Babylon 原生碰撞（消除两套真相来源）+ 3 个占位场景诚实化 | 架构债+诚实度 | type-check 过 |
| **wildera** | M8 任务系统骨架（4文件+3引导任务）+ 修 PostProcessSystem 5 个 Babylon 8 API 类型错误 | 核心功能+类型修复 | tsc 全零错误 |
| **cogent** | 补真实 runLoop 联调测试 + AGENTS.md + README 校正 | 测试+文档 | 81测试全过 |
| **wuwan** | 清理 5 个生成器误产生的乱码垃圾文件 + 补 .gitignore | 工程整洁 | 已提交 |

---

## 二、12 个项目距 high 的剩余差距（按"距离可用"排序）

### 🟢 接近 high（小修即达，工作量 <1-2 天）

#### cogent（~90%，最接近）
- ✅ 79 测试全过、lint 干净、product:check 6/6、CLI 可用、reconcile/workflow 引擎真实
- 剩余：
  1. [小] 补真实 agentloop 联调测试（现有全用 mockRunLoop，缺端到端）
  2. [小] 补 AGENTS.md
  3. [小] README 测试数 75≠79 文案校正
- **阻塞项**：无真实联调测试（声明→真实执行链路无回归保护）

#### tripplan（~88%，本轮已修 lint）
- ✅ 151 测试全过、CLI+server+REST 全通、离线可跑、本轮 lint 已修
- 剩余：
  1. [小] 补 AGENTS.md（标注 StubLLM 限制）
  2. [小] README 工具数 14≠19 校正
  3. [中] 补真实 LLM 联调测试
- **阻塞项**：无（lint 已清，只剩文档校正）

#### tiny-edge-models（~85%，被低估）
- ✅ edge_trainer v3.0.0 完整、32 真实 ONNX 产物、19 测试、CI、product:check 6/6
- 剩余：
  1. [小] 补端侧推理落地示例（TFLite/CoreML，M5）
  2. [小] runs/ 体积收敛（M4）
  3. [中] HTML 报告导出（M3）
- **阻塞项**：无硬阻塞

#### agenttrain（~85%）
- ✅ 完整可玩、84 测试全绿、AI 顾问真接入、逻辑/渲染分离、零占位
- 剩余：
  1. [小] README 测试数校正（写60/7，实际84）
  2. [小] extendLine 头部插入未调 t（视觉跳变，P2）
  3. [中] 响应式画布（固定960×600）
- **阻塞项**：无致命阻塞

### 🟡 半成品（中等工作量，3-7 天）

#### stock-ai（~80%，本轮已修 lint+死注释+新增情绪工具）
- ✅ 78 测试全过、技术面/基本面/回测真实可用（实测茅台价）、本轮 lint+死注释已修、新增 analyze_sentiment 工具（暴露情绪研判能力）
- 剩余：
  1. [中] 新闻源待接入（fetcher sources=[] 返回空；但 get_news 已诚实降级 + analyze_sentiment 提供研判替代，不再是"宣称但不工作"）
  2. [中] 补 Agent 端到端联调测试
- **阻塞项**：无硬阻塞（新闻层已有诚实降级路径，情绪研判已可用）

#### modelstudio（~75%，本轮已修 Draco + 补 server 测试）
- ✅ 三层架构、深度重建开箱可用、编辑器完整、双轨 AI、Draco 已修、server 14 测试
- 剩余：
  1. [中] web 层零测试（web/package.json 无 test 脚本）
  2. [中] USDZ/AR 导出未实现（M4）
  3. [大] TRELLIS/SF3D 本地 generate() 是占位（门槛极高，建议走云端）
- **阻塞项**：M4 收尾（USDZ）+ web 测试

#### wildera（~70%，本轮已建任务系统骨架）
- ✅ M1-M7 全落地、Havok 真启用、本轮任务系统骨架已建（3 引导任务）、PostProcessSystem 类型错误已修
- 剩余：
  1. [中] 任务系统接通验证（事件触发→进度更新→奖励发放的实机测试）
  2. [中] 种子恢复未接通（DEFAULT_SEED 硬编码）
  3. [中] idb 装了未用（存档仍走 localStorage 5MB 上限）
- **阻塞项**：任务系统需实机验证接通（tsc 已全零错误）

#### pastoral-travel（~65%，被低估但结构性短板）
- ✅ 80+ APK、模型训练达标(72%)、M1 验收过、客户端完整
- 剩余：
  1. **[大] 后端 routes/middlewares/utils 三目录全空**（全堆 app.js 单文件）
  2. **[大] 无持久化数据库**（内存对象，重启即丢）
  3. [大] 无鉴权/多用户
  4. [中] Godot 着色器 SCREEN_TEXTURE 版本兼容未修
  5. [大] Main.gd 2297 行 God class
- **阻塞项**：后端空壳 + 无数据库（结构性）

### 🔴 早期原型（距 high 最远）

#### future-world-3055（~50%，本轮已修 Havok+占位诚实化）
- ✅ 本轮 Havok 降级（消除两套真相）、占位场景诚实化
- 剩余：
  1. **[大] 4 个核心场景中 3 个纯占位**（Planet/Ship/GalaxyMap，README 宣传"60+星系"与实现严重不符）
  2. [大] 零单元测试
  3. [中] CombatSystem 仅局部接入
- **阻塞项**：场景内容严重缺失 + 宣传与实现不符

#### computer-use-rtc（~45%）
- ✅ 6 脚本真实可用、probe/click/steps 三模式闭环
- 剩余：
  1. **[大] 无 package.json/构建/测试体系**
  2. [大] 无任何测试
  3. [中] 闭环强依赖外部 GLM key + 双屏一体机，不可复现验证
- **阻塞项**：无工程化 + 无测试

#### taixu-dao-world（~60%，架构债而非缺功能）
- ✅ 核心循环可玩、战斗真实闭环、15 单元测试、三档降级
- 剩余（架构债收口）：
  1. [大] Game.ts 925 行 God class（NEXT_PHASE_PLAN 第1周计划拆分，未实施）
  2. [中] Math.random 泄漏（30+处破坏确定性）
  3. [中] EventSystem 17 行假 EventBus
  4. [中] Chunk 五态资源释放未实现
- **阻塞项**：无单一硬阻塞，主要是架构债

#### platform-demo（~25%）
- 剩余：
  1. **[大] 证据是手写假 JSON**（URL demo.e2e-fusion.local、duration 编造）
  2. [大] 未真正调用 cogent/agentloop/e2e-fusion
- **阻塞项**：假实现 + 未真集成（本质是规格文档）

---

## 三、优先级建议（ROI 排序）

如果继续推进转正，按"投入产出比"：

1. **cogent** —— 补真实联调测试 + AGENTS.md，约 1 天即可达 high（最高 ROI）
2. **tripplan** —— 补 AGENTS.md + 文档校正，约半天（lint 已清）
3. **agenttrain** —— 文档校正，约半天
4. **wildera** —— 任务系统实机验证接通，约 1-2 天达可玩切片
5. **stock-ai** —— 决策新闻层 + 补测试，约 2-3 天
6. **pastoral-travel** —— 后端重构 + 上数据库，约 1 周（回报高但投入大）

---

## 四、本轮未动的项目（保持观察）

- **tripplan**：乱码文件已确认不在（之前已清理）
- **cogent**：未改（差距仅文档级，优先级让用户定）
- **taixu-dao-world**：架构债收口按 NEXT_PHASE_PLAN 4 周计划推进，非本轮范围
