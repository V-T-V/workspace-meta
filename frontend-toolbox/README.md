# 前端工具箱 🧰

> 47 个纯前端开发者日常工具集合 —— JSON 格式化、编码解码、加密哈希、文本处理、图片压缩、二维码等，数据不离开浏览器。

一个纯前端、仅 2 个运行时依赖（qrcode + jszip）、可双击打开的「开发者工具箱」。所有计算在浏览器本地完成，不上传任何数据。左侧分组导航 + 右侧主工作区，每个工具一个独立 URL。

## 特性

| 特性 | 说明 |
|------|------|
| 🧰 **47 个工具** | 覆盖格式化 / 编码 / 加密 / 文本 / 转换 / 文件图片 6 大组 |
| 🔒 **纯本地** | 无后端、无上传、无埋点，数据不出浏览器 |
| 📦 **可双击打开** | 构建产物相对路径，静态托管或本地直接打开 |
| ⚡ **零依赖** | 核心用浏览器原生 API（Web Crypto / Canvas），仅二维码引一个小库 |
| 🌗 **深浅主题** | 跟随系统 / 手动切换，偏好持久化 |
| 🔍 **全局搜索** | 顶栏搜索直达任意工具（标题 + 关键词） |
| 🧩 **可扩展** | 新增工具只需建文件夹，注册中心自动发现 |

## 工具清单（40 个）

### 🎨 格式化（6）
- **JSON 格式化** —— 美化 / 压缩 / 转义 / 排序 / 统计
- **XML / HTML 美化** —— 按标签层级缩进
- **SQL 美化** —— 关键字大写 + 子句换行
- **CSS 美化 / 压缩** —— 规则缩进与压缩
- **HTML 美化** —— 片段层级缩进
- **Markdown 预览** —— 实时渲染为 HTML

### 🔐 编码解码（7）
- **Base64** · **URL** · **HEX** · **HTML 实体** · **Unicode 转义** —— 双向实时
- **JWT 解析** —— 解码三段，展示声明与过期
- **QueryString 解析** —— 查询串转键值/JSON

### #️⃣ 加密 Hash（5）
- **MD5** —— 纯 TS 实现（仅校验用，已不安全）
- **SHA 家族** —— SHA-1/256/384/512（Web Crypto）
- **HMAC 签名** —— SHA-1/256/512
- **bcrypt 哈希信息** —— 解析版本/cost/盐/摘要
- **UUID / ID 生成** —— UUID v4 / NanoID / 雪花 / ULID / 令牌

### 📝 文本处理（8）
- **大小写转换** —— 10 种命名风格
- **文本对比** —— 基于 LCS 的行级 diff
- **字数统计** —— 字符/词/行/段落/字节
- **行排序** · **行去重** —— 多种模式
- **正则测试** —— 实时匹配 + 捕获组
- **Lorem 生成** · **随机字符串**

### 🔄 转换对照（6）
- **时间戳转换** · **进制转换** · **颜色格式转换** · **单位换算** · **ASCII 编码表** · **文字转 ASCII Art**

### 🖼️ 文件图片（8）
- **图片压缩** · **图片批量压缩** · **图片格式转换** · **图片裁剪** · **图片缩放**
- **二维码生成** · **二维码识别**
- **文件 ↔ Base64** · **文件 Hash**

## 技术栈

- **构建**：Vite 5 + TypeScript 5.9（strict）
- **UI**：纯 DOM + CSS，无前端框架
- **路由**：自写零依赖 hashchange
- **注册表**：`import.meta.glob` 自动发现
- **加密**：Web Crypto API（SubtleCrypto）
- **图片**：Canvas 2D
- **依赖**：仅 `qrcode`（二维码生成）

## 快速开始

```bash
npm install
npm run dev        # http://localhost:5230
```

构建可双击打开的静态站：

```bash
npm run build      # 产物在 dist/，相对路径
```

测试：

```bash
npm test           # 8 个测试文件覆盖 lib 纯函数
npm run type-check # TS strict
npm run lint       # ESLint
```

## 架构

详见 [`AGENTS.md`](./AGENTS.md)。核心特点：

- **注册表自动发现**：`src/tools/<组>/<工具>/meta.ts + index.ts`，新增工具无需改注册中心。
- **lib 与 UI 分离**：`src/lib/` 纯函数（可测），`src/tools/` 是把 lib 接到 UI 的薄胶水层。
- **零依赖**：除二维码外，全部用浏览器原生 API。

## 开发：新增一个工具

1. 在 `src/tools/<组>/<新工具>/` 建两个文件：

```ts
// meta.ts
import type { ToolMeta } from '../../../types.ts';
const meta: ToolMeta = {
  id: 'my-tool',
  groupId: 'text',           // 必须在 taxonomy.ts 的组里
  title: '我的工具',
  summary: '一句话描述',
  keywords: ['my', 'tool'],
  icon: '🔧',
};
export default meta;
```

```ts
// index.ts
import type { Tool, ToolInstance } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx) {
        const layout = createToolLayout(meta);
        const input = textarea('输入…');
        layout.inputArea.append(input);
        ctx.container.append(layout.container);
      },
    };
  },
};
export default tool;
```

2. 完成。侧栏、首页、搜索、路由全部自动生效。

## 许可

MIT
