// =============================================================================
// JSON 处理纯函数库 —— 格式化 / 压缩 / 转义 / 排序 / 路径取值
// =============================================================================

export interface JsonFormatOptions {
  /** 缩进空格数，0 表示压缩成一行。 */
  indent?: number;
  /** key 是否按字典序排序。 */
  sortKeys?: boolean;
  /** 是否保留非 ASCII 字符（不转成 \uXXXX）。 */
  asciiOnly?: boolean;
}

/** 安全 JSON 解析，失败返回 { error }。 */
export function tryParseJSON(input: string): { ok: true; value: unknown } | { ok: false; error: string } {
  try {
    return { ok: true, value: JSON.parse(input) };
  } catch (e) {
    return { ok: false, error: (e as Error).message };
  }
}

/** 递归对对象/数组的所有 object key 排序（数组顺序保持）。 */
export function sortObjectKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortObjectKeys);
  if (value && typeof value === 'object') {
    const obj = value as Record<string, unknown>;
    const sorted: Record<string, unknown> = {};
    for (const k of Object.keys(obj).sort()) sorted[k] = sortObjectKeys(obj[k]);
    return sorted;
  }
  return value;
}

/** 格式化 JSON 字符串。非法输入抛错。 */
export function formatJSON(input: string, opts: JsonFormatOptions = {}): string {
  const { indent = 2, sortKeys = false } = opts;
  let value = JSON.parse(input);
  if (sortKeys) value = sortObjectKeys(value);
  const text = JSON.stringify(value, null, indent);
  return text + (indent === 0 ? '' : '\n');
}

/** 压缩 JSON 为一行（无空格）。 */
export function minifyJSON(input: string): string {
  return JSON.stringify(JSON.parse(input));
}

/** 转义 JSON 字符串内容（用于把任意文本塞进 JSON 字符串值）。 */
export function escapeJSONString(input: string): string {
  return JSON.stringify(input);
}

/** 反转义 JSON 字符串（去掉外层引号，还原转义）。 */
export function unescapeJSONString(input: string): string {
  const trimmed = input.trim();
  const wrapped = trimmed.startsWith('"') && trimmed.endsWith('"') ? trimmed : `"${trimmed}"`;
  return JSON.parse(wrapped) as string;
}

/** 按 JSONPath 简化版（点号 + [index]）取值。 */
export function getByPath(value: unknown, path: string): unknown {
  if (!path) return value;
  const tokens = path
    .replace(/\[(\d+)\]/g, '.$1')
    .split('.')
    .filter(Boolean);
  let cur: unknown = value;
  for (const t of tokens) {
    if (cur == null) return undefined;
    cur = (cur as Record<string, unknown>)[t];
  }
  return cur;
}

/** 统计 JSON 结构：类型 / 深度 / key 数等。 */
export interface JsonStats {
  type: string;
  depth: number;
  keys: number;
  arrayItems: number;
  size: number;
}
export function statsJSON(value: unknown): JsonStats {
  let keys = 0;
  let arrayItems = 0;
  let depth = 0;
  function walk(v: unknown, d: number): void {
    depth = Math.max(depth, d);
    if (Array.isArray(v)) {
      arrayItems += v.length;
      for (const item of v) walk(item, d + 1);
    } else if (v && typeof v === 'object') {
      const obj = v as Record<string, unknown>;
      keys += Object.keys(obj).length;
      for (const k of Object.keys(obj)) walk(obj[k], d + 1);
    }
  }
  walk(value, 0);
  return {
    type: Array.isArray(value) ? 'array' : value === null ? 'null' : typeof value,
    depth,
    keys,
    arrayItems,
    size: JSON.stringify(value).length,
  };
}
