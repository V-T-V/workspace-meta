// =============================================================================
// 主题切换 —— 深色/浅色，跟随系统 + 手动覆盖，localStorage 持久化
// =============================================================================

export type ThemeMode = 'light' | 'dark' | 'auto';

const STORAGE_KEY = 'ftb-theme';
const ATTR = 'data-theme';

function resolveAuto(): 'light' | 'dark' {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function apply(mode: ThemeMode): 'light' | 'dark' {
  const effective = mode === 'auto' ? resolveAuto() : mode;
  document.documentElement.setAttribute(ATTR, effective);
  return effective;
}

/** 初始化主题，读取存储偏好（默认 auto）。返回当前模式。 */
export function initTheme(): ThemeMode {
  const stored = (localStorage.getItem(STORAGE_KEY) as ThemeMode | null) ?? 'auto';
  apply(stored);
  // 系统主题变化时，若为 auto 则跟随
  window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', () => {
    const cur = (localStorage.getItem(STORAGE_KEY) as ThemeMode | null) ?? 'auto';
    if (cur === 'auto') apply('auto');
  });
  return stored;
}

/** 设置主题。 */
export function setTheme(mode: ThemeMode): void {
  localStorage.setItem(STORAGE_KEY, mode);
  apply(mode);
}

/** 切换深/浅（在两者间翻转，不影响 auto 语义）。 */
export function toggleTheme(): 'light' | 'dark' {
  const cur = document.documentElement.getAttribute(ATTR);
  const next = cur === 'dark' ? 'light' : 'dark';
  setTheme(next);
  return next;
}

/** 当前生效的主题（light/dark）。 */
export function currentTheme(): 'light' | 'dark' {
  return (document.documentElement.getAttribute(ATTR) as 'light' | 'dark') ?? resolveAuto();
}

/** 当前设置的模式（含 auto）。 */
export function currentMode(): ThemeMode {
  return (localStorage.getItem(STORAGE_KEY) as ThemeMode | null) ?? 'auto';
}
