// =============================================================================
// 外壳渲染 —— 顶栏（品牌 + 全局搜索 + 主题切换）+ 侧栏 + 主区
// =============================================================================

import type { ToolMeta } from '../types.ts';
import { el } from '../ui/components.ts';
import { initTheme, toggleTheme, currentTheme } from '../core/theme.ts';
import { renderSidebar, type SidebarHandle } from './Sidebar.ts';
import { search } from '../core/search.ts';
import { goHome, goTool } from '../core/router.ts';

export interface ShellHandle {
  /** 主区容器（工具/首页挂载点）。 */
  main: HTMLElement;
  /** 高亮指定工具（侧栏 + URL 同步）。 */
  setActiveTool: (id: string | null) => void;
  /** 刷新侧栏（工具元数据变化时）。 */
  refreshSidebar: (metas: readonly ToolMeta[]) => void;
}

/** 渲染外壳并挂到 #app。 */
export function renderShell(
  app: HTMLElement,
  allMetas: readonly ToolMeta[],
): ShellHandle {
  initTheme();

  // 清空启动占位
  app.replaceChildren();

  const shell = el('div', 'ftb-shell');

  // ---------- 顶栏 ----------
  const topbar = el('div', 'ftb-topbar');

  const brand = el('div', 'ftb-brand');
  brand.append(
    el('span', 'ftb-brand-icon', '🧰'),
    el('span', 'ftb-brand-name', '前端工具箱'),
    el('span', 'ftb-brand-sub', 'Frontend Toolbox'),
  );
  brand.addEventListener('click', () => goHome());

  // 全局搜索
  const searchBox = el('div', 'ftb-search');
  const searchInput = el('input') as HTMLInputElement;
  searchInput.type = 'search';
  searchInput.placeholder = '搜索工具（如 JSON、Base64、时间戳）…';
  searchInput.setAttribute('aria-label', '搜索工具');
  const searchResults = el('div', 'ftb-search-results');
  searchBox.append(searchInput, searchResults);

  const renderSearchResults = (query: string): void => {
    if (!query.trim()) {
      searchResults.classList.remove('is-open');
      searchResults.replaceChildren();
      return;
    }
    const results = search(allMetas, query).slice(0, 8);
    searchResults.replaceChildren();
    for (const r of results) {
      const item = el('div', 'ftb-search-item');
      const text = el('div', 'ftb-search-item-text');
      text.append(
        Object.assign(el('div', 'ftb-search-item-title'), { textContent: r.meta.title }),
        Object.assign(el('div', 'ftb-search-item-summary'), { textContent: r.meta.summary }),
      );
      item.append(el('span', 'ftb-search-item-icon', r.meta.icon ?? '🔧'), text);
      item.addEventListener('click', () => {
        goTool(r.meta.id);
        searchInput.value = '';
        searchResults.classList.remove('is-open');
        searchInput.blur();
      });
      searchResults.append(item);
    }
    searchResults.classList.toggle('is-open', results.length > 0);
  };

  searchInput.addEventListener('input', () => renderSearchResults(searchInput.value));
  searchInput.addEventListener('focus', () => {
    if (searchInput.value) renderSearchResults(searchInput.value);
  });
  document.addEventListener('click', (e: MouseEvent) => {
    if (!searchBox.contains(e.target as Node)) {
      searchResults.classList.remove('is-open');
    }
  });
  searchInput.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      const first = search(allMetas, searchInput.value)[0];
      if (first) {
        goTool(first.meta.id);
        searchInput.value = '';
        searchResults.classList.remove('is-open');
        searchInput.blur();
      }
    } else if (e.key === 'Escape') {
      searchInput.value = '';
      searchResults.classList.remove('is-open');
      searchInput.blur();
    }
  });

  // 主题切换
  const actions = el('div', 'ftb-topbar-actions');
  const themeBtn = el('button', 'ftb-btn ftb-btn--ghost', currentTheme() === 'dark' ? '☀️' : '🌙');
  themeBtn.title = '切换深色/浅色主题';
  themeBtn.addEventListener('click', () => {
    const next = toggleTheme();
    themeBtn.textContent = next === 'dark' ? '☀️' : '🌙';
  });
  actions.append(themeBtn);

  topbar.append(brand, searchBox, actions);

  // ---------- 主体 ----------
  const body = el('div', 'ftb-body');
  const sidebarHost = el('aside', 'ftb-sidebar');
  const main = el('main', 'ftb-main');
  body.append(sidebarHost, main);

  shell.append(topbar, body);
  app.append(shell);

  const sidebar: SidebarHandle = renderSidebar(sidebarHost, allMetas);

  return {
    main,
    setActiveTool(id) {
      sidebar.setActive(id);
    },
    refreshSidebar(metas) {
      sidebarHost.replaceChildren();
      renderSidebar(sidebarHost, metas);
    },
  };
}
