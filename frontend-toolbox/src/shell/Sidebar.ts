// =============================================================================
// 侧栏 —— 分组折叠导航 + 顶部搜索过滤
// 默认所有组折叠，进入工具时自动展开所属组，搜索时跨组过滤。
// =============================================================================

import type { ToolMeta } from '../types.ts';
import { GROUPS } from '../taxonomy.ts';
import { metasByGroup } from '../core/registry.ts';
import { el } from '../ui/components.ts';
import { goTool } from '../core/router.ts';

export interface SidebarHandle {
  setActive: (id: string | null) => void;
}

/** 渲染侧栏导航。 */
export function renderSidebar(host: HTMLElement, _allMetas: readonly ToolMeta[]): SidebarHandle {
  host.replaceChildren();

  // ---- 搜索框 ----
  const searchBox = el('div', 'ftb-sidebar-search');
  const searchInput = el('input') as HTMLInputElement;
  searchInput.type = 'search';
  searchInput.placeholder = '过滤工具…';
  searchInput.setAttribute('aria-label', '过滤工具');
  searchBox.append(searchInput);
  host.append(searchBox);

  // ---- 分组导航容器 ----
  const navHost = el('div', 'ftb-sidebar-nav');
  host.append(navHost);

  const groupEls: HTMLElement[] = [];
  const navItems: HTMLDivElement[] = [];

  /** 渲染分组列表（可被搜索驱动重建）。 */
  const renderGroups = (filter: string): void => {
    navHost.replaceChildren();
    groupEls.length = 0;
    // navItems 不清空（setActive 需稳定引用，但重建后引用会变，故也清空）
    navItems.length = 0;

    const q = filter.trim().toLowerCase();

    for (const group of GROUPS) {
      let items = metasByGroup(group.id);
      if (q) {
        items = items.filter(
          (m) =>
            m.title.toLowerCase().includes(q) ||
            m.summary.toLowerCase().includes(q) ||
            (m.keywords ?? []).some((k) => k.toLowerCase().includes(q)),
        );
      }
      if (items.length === 0) continue;

      const groupEl = el('div', 'ftb-nav-group');
      // 搜索时强制展开，否则默认折叠
      if (!q) groupEl.classList.add('is-collapsed');

      const head = el('div', 'ftb-nav-group-head');
      head.append(
        el('span', 'ftb-nav-group-icon', group.icon),
        el('span', undefined, group.name),
        el('span', 'ftb-nav-group-count', String(items.length)),
      );
      head.title = group.blurb;

      const itemsWrap = el('div', 'ftb-nav-group-items');
      for (const meta of items) {
        const item = el('div', 'ftb-nav-item');
        item.dataset.toolId = meta.id;
        item.dataset.groupId = meta.groupId;
        item.append(
          el('span', 'ftb-nav-item-icon', meta.icon ?? '🔧'),
          el('span', undefined, meta.title),
        );
        item.addEventListener('click', () => goTool(meta.id));
        itemsWrap.append(item);
        navItems.push(item);
      }

      head.addEventListener('click', () => groupEl.classList.toggle('is-collapsed'));

      groupEl.append(head, itemsWrap);
      navHost.append(groupEl);
      groupEls.push(groupEl);
    }

    if (groupEls.length === 0) {
      navHost.append(el('div', 'ftb-sidebar-empty', '无匹配工具'));
    }
  };

  renderGroups('');

  searchInput.addEventListener('input', () => {
    renderGroups(searchInput.value);
  });

  // 点击首页/品牌时清空搜索
  searchInput.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      searchInput.value = '';
      renderGroups('');
      searchInput.blur();
    } else if (e.key === 'Enter') {
      const first = navItems[0];
      if (first) {
        const id = first.dataset.toolId;
        if (id) goTool(id);
        searchInput.value = '';
        renderGroups('');
        searchInput.blur();
      }
    }
  });

  return {
    setActive(id) {
      for (const it of navItems) {
        const active = it.dataset.toolId === id;
        it.classList.toggle('is-active', active);
        // 进入工具时自动展开所属组
        if (active) {
          let parent: HTMLElement | null = it.parentElement;
          while (parent && !parent.classList.contains('ftb-nav-group')) {
            parent = parent.parentElement;
          }
          parent?.classList.remove('is-collapsed');
        }
      }
    },
  };
}
