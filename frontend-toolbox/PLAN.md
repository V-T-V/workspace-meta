# 前端工具箱 · 设计规划（留档）

> 本文档为项目初版规划，实现完成后留档备查。当前实现状态以代码与 AGENTS.md 为准。

## 定位

纯前端、零运行时依赖、可双击打开的「开发者日常工具集合站」。对标 devtoolkit / itools / tool.chen，但完全本地运行、数据不离开浏览器。

## 技术栈

| 项 | 选择 |
|----|------|
| 构建 | Vite 5.4 |
| 语言 | TypeScript ~5.9 strict + noUncheckedIndexedAccess |
| 路由 | 自写零依赖 hashchange |
| 注册表 | import.meta.glob 自动发现 |
| UI | 纯 DOM + CSS |
| 加密 | Web Crypto API |
| 图片 | Canvas 2D |
| 测试 | node --test + tsx |
| 端口 | 5230 |

## 核心架构

- **工具契约**：`Tool { meta: ToolMeta; create(): ToolInstance }`，每个工具文件夹含 meta.ts + index.ts。
- **注册表**：`import.meta.glob('../tools/*/*/meta.ts', { eager })` 收集元数据；`import.meta.glob('../tools/*/*/index.ts')` 懒加载实现。
- **路由**：`#/` 首页，`#/tool/<id>` 工具页。
- **分层**：lib（纯函数）→ tools（UI 胶水）→ ui（原语）→ core（基础设施）。

## 工具范围（40 个，6 组）

详见 README.md 工具清单。

## 不在范围

- Service Worker / PWA 离线
- 多语言（仅中文）
- 工具使用历史详情（仅存主题与最近使用）
- 需 WASM 的功能（完整 bcrypt 生成、PDF 转换）

## 实现完成情况

全部 40 个工具 + 核心架构 + lib 库 + UI 原语 + 外壳 + 测试 + 文档 均已实现。
