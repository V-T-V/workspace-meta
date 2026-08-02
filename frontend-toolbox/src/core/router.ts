// =============================================================================
// 极简 hash 路由器（零依赖，基于 hashchange）
//   #/            → 首页（搜索 + 宫格）
//   #/tool/<id>   → 单个工具
// 复用 algorithms-atlas 的范式。
// =============================================================================

export interface Route {
  /** 'home' | 'tool' */
  view: 'home' | 'tool';
  /** view === 'tool' 时为工具 id。 */
  id?: string;
}

const listeners = new Set<(r: Route) => void>();

function parse(): Route {
  const hash = (window.location.hash || '').replace(/^#\/?/, '').trim();
  if (!hash) return { view: 'home' };
  const m = hash.match(/^tool\/(.+)$/);
  if (m) return { view: 'tool', id: m[1] };
  // 兼容直接写 id 的旧链接：仅当匹配到某工具时才算，否则回首页
  return { view: 'home' };
}

/** 当前路由。 */
export function currentRoute(): Route {
  return parse();
}

/** 跳转首页。 */
export function goHome(): void {
  window.location.hash = '#/';
}

/** 跳转到指定工具。 */
export function goTool(id: string): void {
  window.location.hash = `#/tool/${id}`;
}

/** 订阅路由变化，返回取消订阅函数。注册后立即触发一次当前路由。 */
export function onRoute(fn: (r: Route) => void): () => void {
  listeners.add(fn);
  // 立即触发一次，确保首屏渲染（无论 initRouter 是否先调用）
  fn(parse());
  return () => listeners.delete(fn);
}

function emit(): void {
  const r = parse();
  for (const fn of listeners) fn(r);
}

/** 初始化：监听 hashchange。 */
export function initRouter(): void {
  window.addEventListener('hashchange', emit);
}
