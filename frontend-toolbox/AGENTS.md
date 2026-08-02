# 前端工具箱（frontend-toolbox）· AGENTS.md

## 项目内容（What）

**前端工具箱** —— 一个纯前端、**仅 2 个运行时依赖**、可双击打开的「开发者日常工具集合站」。覆盖 JSON / 编码 / 加密 / 文本 / 时间 / 颜色 / 图片 / 文件等 **42 个**前端常用工具，左侧分组导航 + 右侧主工作区，每个工具一个独立 URL。

对标 devtoolkit / itools / tool.chen，但**完全本地运行、数据不离开浏览器**。

## 目标（Goal）

- **一站式**：把开发日常高频小工具聚合在一个站点，免来回切换在线工具。
- **纯前端**：无后端、无密钥上传、无埋点。所有计算在浏览器本地完成。
- **可双击打开**：构建产物相对路径，部署到任意静态托管或本地双击 index.html 可用。
- **可扩展**：新增工具只需建文件夹（meta.ts + index.ts），注册中心自动发现，无需改核心代码。

## 当前情况（Status）

- ✅ 40 个工具全部实现，覆盖 6 大组（格式化 / 编码 / 加密 / 文本 / 转换 / 文件图片）。
- ✅ 核心架构：types 契约 + taxonomy 分组 + registry 自动发现 + 零依赖 hash 路由 + 全局搜索 + 深浅主题。
- ✅ lib 纯函数库（10 个）：codec / hash / json / text / time / color / id / format / markdown / unit / image，全部可测。
- ✅ UI 原语（5 个）：toast / code-block / file-drop / layout / components。
- ✅ 外壳：顶栏（品牌+全局搜索+主题切换）+ 侧栏（分组折叠）+ 首页（最近使用+按组宫格）+ 主工作区。
- ✅ 测试：8 个测试文件覆盖 lib 纯函数。
- ✅ 唯一运行时依赖：`qrcode`（二维码生成）和 `jszip`（批量 zip 打包）。
- ⏳ 后续可扩展：PWA 离线、多语言、更多图片处理（滤镜/水印）、代码编辑器升级。

## 技术栈与架构

```
frontend-toolbox/
├── index.html                  # 单页入口
├── package.json                # Vite 5 + TS 5.9 strict，仅 qrcode 一个运行时依赖
├── tsconfig.json               # strict + noUncheckedIndexedAccess（对齐 poetry-garden）
├── vite.config.ts              # base:'./'（可双击），port:5230
├── src/
│   ├── types.ts                # ToolMeta / Tool / ToolGroup 核心契约
│   ├── taxonomy.ts             # 6 大分组定义（单一来源）
│   ├── main.ts                 # 入口：外壳 + 路由 + 工具懒加载
│   ├── core/                   # 与具体工具无关的基础设施
│   │   ├── registry.ts         # import.meta.glob 自动发现工具
│   │   ├── router.ts           # 零依赖 hashchange 路由
│   │   ├── search.ts           # 工具全局搜索（标题+关键词+拼音首字母）
│   │   ├── theme.ts            # 深色/浅色/auto + 持久化
│   │   └── storage.ts          # localStorage 封装（最近使用）
│   ├── shell/                  # 外壳
│   │   ├── Shell.ts            # 顶栏 + 侧栏 + 主区
│   │   ├── Sidebar.ts          # 分组折叠导航
│   │   └── Home.ts             # 首页宫格
│   ├── ui/                     # 跨工具复用的 UI 原语
│   │   ├── components.ts       # el/button/textarea/select/checkbox/copyText/downloadBlob
│   │   ├── toast.ts            # 全局提示
│   │   ├── code-block.ts       # 带复制按钮的代码块
│   │   ├── file-drop.ts        # 文件拖拽上传
│   │   └── layout.ts           # 工具页骨架（标题+输入区+输出区）
│   ├── lib/                    # 纯函数库（无 DOM，可 node --test 直接测）
│   │   ├── codec.ts            # Base64/URL/HEX/HTML/Unicode/Binary
│   │   ├── hash.ts             # MD5(纯实现)/SHA(Web Crypto)/HMAC
│   │   ├── json-utils.ts       # 格式化/压缩/排序/统计/路径取值
│   │   ├── text-utils.ts       # 大小写/统计/排序/去重/diff/正则/lorem
│   │   ├── time-utils.ts       # 时间戳/格式化/相对时间/差值
│   │   ├── color-utils.ts      # HEX/RGB/HSL 互转 + 配色
│   │   ├── id-utils.ts         # UUID/NanoID/雪花/ULID/令牌
│   │   ├── format-utils.ts     # CSS/HTML/XML/SQL 美化
│   │   ├── markdown-utils.ts   # 轻量 Markdown 渲染（防 XSS）
│   │   ├── unit-utils.ts       # 单位换算（长度/重量/数据/面积/时间/温度）
│   │   └── image-utils.ts      # 图片压缩/转换/裁剪/缩放（Canvas，依赖 DOM）
│   ├── styles/                 # themes.css（变量）+ main.css（布局组件）
│   └── tools/                  # ⭐ 每个工具一个文件夹，自动发现
│       ├── format/             # json csv xml sql css html-beautify markdown
│       ├── encode/             # base64 url hex html-entity unicode jwt qs
│       ├── crypto/             # hash-md5 hash-sha hmac bcrypt-info uuid-gen
│       ├── text/               # case-convert diff word-count sort-lines dedup-lines regex-tester lorem placeholder
│       ├── convert/            # timestamp base-convert color unit charset ascii-art
│       └── file/               # image-compress image-compress-batch image-convert image-crop image-resize qrcode-gen qrcode-scan file-base64 hash-file
│       └── code/               # js-run ast-view js-beautify js-minify number-bits
└── test/                       # 11 个 *.test.ts
```

**核心架构决策：**

- **注册表自动发现（复用 algorithms-atlas 模式）**：`import.meta.glob('../tools/*/*/meta.ts', { eager:true })` 收集元数据进首包（纯数据，用于导航/搜索）；`import.meta.glob('../tools/*/*/index.ts')` 懒加载工具实现（按需分块）。**新增工具只需建文件夹，无需改注册中心**。
- **零依赖 hash 路由（复用 atlas/poetry-garden 模式）**：`#/` 首页，`#/tool/<id>` 工具页。基于 hashchange，静态部署友好。
- **lib 与 tools 分离**：lib 是纯函数（无 DOM），tools 是把 lib 接到 UI 的薄胶水层。所有业务逻辑在 lib 中可测。
- **MD5 纯 TS 实现**：Web Crypto 不支持 MD5（已不安全），故按 RFC 1321 实现，仅用于校验/展示。
- **Hash 用 Web Crypto**：SHA 家族与 HMAC 用浏览器原生 SubtleCrypto，免装 crypto-js。
- **图片处理用 Canvas 2D**：压缩/转换/裁剪/缩放全部原生 API，免装 sharp/browser-image-compression。
- **主题三态**：light / dark / auto（跟随系统），localStorage 持久化。

## 如何运行

```bash
npm install
npm run dev        # → http://localhost:5230
npm run type-check # TS strict（含 noUncheckedIndexedAccess）
npm run lint       # ESLint 9 flat config
npm test           # 8 个测试文件
npm run build      # 生产构建（base:'./'，可双击打开）
```

## 关键约定

- **端口 5230**：避开工作区兄弟项目（poetry-garden 5185 / superpower-system 5217 等）。
- **TS strict**：`strict` + `noUncheckedIndexedAccess` + `verbatimModuleSyntax` + `allowImportingTsExtensions`（对齐 poetry-garden）。
- **零运行时依赖**：核心功能用浏览器原生 API（Web Crypto / Canvas / URLSearchParams / BarcodeDetector）。唯一例外是 `qrcode`（二维码生成）。
- **工具契约**：每个工具文件夹含 `meta.ts`（默认导出 ToolMeta）+ `index.ts`（默认导出 Tool，含 `meta` 与 `create()`）。registry 按文件夹名作为工具 id。
- **扩展工具**：在 `src/tools/<组>/<工具>/` 建 meta.ts + index.ts，groupId 必须存在于 taxonomy.ts。
- **测试**：`node --test --import tsx`，源码 import 带 `.ts` 后缀。仅测 lib 纯函数（DOM 相关的 image-utils 不测）。
- **数据安全**：所有工具的计算在本地完成，不上传任何数据；localStorage 仅存主题偏好与最近使用列表。

## 与其他项目的关系

- **独立项目，零代码依赖**。不 import 任何工作区其他项目。
- 架构借鉴 **algorithms-atlas** 的注册表自动发现 + 零依赖路由模式，但工具领域完全不同。
- 配置骨架对齐 **poetry-garden** / **superpower-system**（相同 Vite/TS/ESLint/Prettier 版本与 strict 程度）。
- 定位区别：本项目是「工具集合」，非 Agent/研究/游戏，与工作区现有项目无重叠。
