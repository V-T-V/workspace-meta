// =============================================================================
// localStorage 封装 —— 偏好/最近使用的工具，带命名空间与防御性兜底
// =============================================================================

const PREFIX = 'ftb:';

/** 安全读取并 JSON 解析；失败或无则返回 fallback。 */
export function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(PREFIX + key);
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

/** 安全写入 JSON。 */
export function writeJSON<T>(key: string, value: T): void {
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify(value));
  } catch {
    // 容量满或隐私模式：静默忽略
  }
}

/** 删除。 */
export function remove(key: string): void {
  try {
    localStorage.removeItem(PREFIX + key);
  } catch {
    /* noop */
  }
}

// —— 最近使用工具（最多 N 个，去重，最新在前）——

const RECENT_KEY = 'recent-tools';
const RECENT_MAX = 8;

export function pushRecent(toolId: string): void {
  const list = readJSON<string[]>(RECENT_KEY, []).filter((id) => id !== toolId);
  list.unshift(toolId);
  writeJSON(RECENT_KEY, list.slice(0, RECENT_MAX));
}

export function getRecent(): string[] {
  return readJSON<string[]>(RECENT_KEY, []);
}
