// =============================================================================
// 工具分组 · 单一来源（Single Source of Truth）
// 6 大组，覆盖前端日常工具的全部域。每个工具的 groupId 必须在此表中。
// 顺序即左侧导航与首页宫格的默认显示顺序。
// =============================================================================

import type { ToolGroup } from './types.ts';

export const GROUPS: readonly ToolGroup[] = [
  {
    id: 'format',
    name: '格式化',
    icon: '🎨',
    theme: '--c-blue',
    blurb: 'JSON / XML / SQL / CSS / HTML / Markdown 等结构化文本美化。',
  },
  {
    id: 'encode',
    name: '编码解码',
    icon: '🔐',
    theme: '--c-cyan',
    blurb: 'Base64 / URL / HEX / HTML 实体 / Unicode / JWT / QueryString。',
  },
  {
    id: 'crypto',
    name: '加密 Hash',
    icon: '#️⃣',
    theme: '--c-red',
    blurb: 'MD5 / SHA 家族 / HMAC / UUID 生成，基于 Web Crypto。',
  },
  {
    id: 'text',
    name: '文本处理',
    icon: '📝',
    theme: '--c-green',
    blurb: '大小写 / 对比 / 统计 / 排序 / 去重 / 正则 / 随机文本。',
  },
  {
    id: 'convert',
    name: '转换对照',
    icon: '🔄',
    theme: '--c-orange',
    blurb: '时间戳 / 进制 / 颜色 / 单位 / 编码表 / ASCII Art。',
  },
  {
    id: 'file',
    name: '文件图片',
    icon: '🖼️',
    theme: '--c-purple',
    blurb: '图片压缩 / 格式转换 / 裁剪 / 缩放 / 二维码 / 文件编码。',
  },
  {
    id: 'code',
    name: '代码',
    icon: '⌨️',
    theme: '--c-indigo',
    blurb: 'JS 运行 / AST 可视化 / 代码美化混淆 / 进制位运算。',
  },
];

// —— 派生查询（保持单一来源，避免散落硬编码）——

export const GROUP_IDS: readonly string[] = GROUPS.map((g) => g.id);

const GROUP_BY_ID = new Map<string, ToolGroup>(GROUPS.map((g) => [g.id, g]));

export function getGroup(id: string): ToolGroup | undefined {
  return GROUP_BY_ID.get(id);
}

/** 断言分组存在；缺失则抛错（供注册期校验）。 */
export function requireGroup(id: string): ToolGroup {
  const g = GROUP_BY_ID.get(id);
  if (!g) throw new Error(`Unknown tool group id: ${id}`);
  return g;
}
