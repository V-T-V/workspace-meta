// =============================================================================
// 工具注册表 · 自动发现（复用 algorithms-atlas 模式）
//   - 元数据（meta.ts）静态收集：打进首包用于导航/搜索
//   - 工具实现（index.ts）懒加载：按需分块，不进首包
// 新增工具只需在 src/tools/<组>/<工具>/ 下建 meta.ts + index.ts。
// =============================================================================

import type { Tool, ToolMeta } from '../types.ts';
import { requireGroup } from '../taxonomy.ts';

// —— 同步元数据登记：eager 加载 meta.ts（纯数据，首包友好）——
// 注意：用 `import: 'default'` 时，glob 的 value 直接是 default 导出本身。
const META_MODULES = import.meta.glob('../tools/*/*/meta.ts', {
  eager: true,
  import: 'default',
}) as Record<string, ToolMeta>;

export const TOOL_METAS: readonly ToolMeta[] = Object.values(META_MODULES)
  // 校验：groupId 必须存在；缺失则跳过并警告（开发期可见）
  .filter((meta) => {
    try {
      requireGroup(meta.groupId);
      return true;
    } catch {
      console.warn(`[registry] 工具 ${meta.id} 的 groupId=${meta.groupId} 不存在，已忽略`);
      return false;
    }
  })
  .sort((a, b) => {
    // 先按组顺序，组内按标题
    const ga = requireGroup(a.groupId);
    const gb = requireGroup(b.groupId);
    const gi = GROUP_ORDER.indexOf(ga.id) - GROUP_ORDER.indexOf(gb.id);
    if (gi !== 0) return gi;
    return a.title.localeCompare(b.title, 'zh');
  });

import { GROUP_IDS as GROUP_ORDER } from '../taxonomy.ts';

const META_BY_ID = new Map<string, ToolMeta>(TOOL_METAS.map((m) => [m.id, m]));

export function findMeta(id: string): ToolMeta | undefined {
  return META_BY_ID.get(id);
}

export function hasTool(id: string): boolean {
  return META_BY_ID.has(id);
}

export function metasByGroup(groupId: string): ToolMeta[] {
  return TOOL_METAS.filter((m) => m.groupId === groupId);
}

// —— 懒加载工具实现：id → () => Promise<Tool> ——
const TOOL_LAZY = import.meta.glob('../tools/*/*/index.ts') as Record<
  string,
  () => Promise<{ default: Tool }>
>;

const FACTORIES = (() => {
  const m = new Map<string, () => Promise<Tool>>();
  for (const [path, loader] of Object.entries(TOOL_LAZY)) {
    const id = path.split('/').slice(-2, -1)[0];
    if (!id) continue;
    m.set(id, async () => (await loader()).default);
  }
  return m;
})();

export async function loadTool(id: string): Promise<Tool | undefined> {
  const factory = FACTORIES.get(id);
  if (!factory) return undefined;
  try {
    return await factory();
  } catch (e) {
    console.error(`[registry] 加载工具 ${id} 失败：`, e);
    return undefined;
  }
}

/** 工具总数。 */
export const TOOL_COUNT: number = TOOL_METAS.length;
