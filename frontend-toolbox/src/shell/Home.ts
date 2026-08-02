// =============================================================================
// 首页 —— 大搜索框 + 分组宫格 + 最近使用
// 作为导航中枢，一眼看到所有分类，快速直达。
// =============================================================================

import type { ToolMeta } from '../types.ts';
import { GROUPS, getGroup } from '../taxonomy.ts';
import { TOOL_COUNT, findMeta } from '../core/registry.ts';
import { el } from '../ui/components.ts';
import { goTool } from '../core/router.ts';
import { getRecent } from '../core/storage.ts';
import { search } from '../core/search.ts';

export function renderHome(host: HTMLElement, allMetas: readonly ToolMeta[]): void {
  host.replaceChildren();

  const home = el('div', 'ftb-home');

  // ---- Hero + 大搜索框 ----
  const hero = el('div', 'ftb-home-hero');
  hero.append(
    el('h1', undefined, '🧰 前端工具箱'),
    el('p', undefined, `${TOOL_COUNT} 个纯前端开发者工具 · 数据不离开浏览器 · 可双击打开`),
  );

  const searchBox = el('div', 'ftb-home-search');
  const searchInput = el('input') as HTMLInputElement;
  searchInput.type = 'search';
  searchInput.placeholder = '🔍 搜索工具名称、关键词、拼音…（如 JSON、编码、时间戳）';
  searchInput.setAttribute('aria-label', '搜索工具');
  const searchResults = el('div', 'ftb-home-search-results');
  searchBox.append(searchInput, searchResults);
  hero.append(searchBox);
  home.append(hero);

  // ---- 搜索逻辑 ----
  const renderSearchResults = (query: string): void => {
    searchResults.replaceChildren();
    if (!query.trim()) {
      searchResults.classList.remove('is-open');
      return;
    }
    const results = search(allMetas, query).slice(0, 10);
    for (const r of results) {
      const item = el('div', 'ftb-home-search-item');
      const left = el('div', 'ftb-home-search-item-main');
      const itemText = el('div', 'ftb-home-search-item-text');
      itemText.append(
        Object.assign(el('div', 'ftb-home-search-item-title'), { textContent: r.meta.title }),
        Object.assign(el('div', 'ftb-home-search-item-summary'), { textContent: r.meta.summary }),
      );
      left.append(
        el('span', 'ftb-home-search-item-icon', r.meta.icon ?? '🔧'),
        itemText,
      );
      const groupName = getGroup(r.meta.groupId)?.name ?? '';
      left.append(el('span', 'ftb-home-search-item-group', groupName));
      item.append(left);
      item.addEventListener('click', () => {
        goTool(r.meta.id);
        searchInput.value = '';
        searchResults.classList.remove('is-open');
      });
      searchResults.append(item);
    }
    searchResults.classList.toggle('is-open', results.length > 0);
  };

  searchInput.addEventListener('input', () => renderSearchResults(searchInput.value));
  searchInput.addEventListener('focus', () => {
    if (searchInput.value) renderSearchResults(searchInput.value);
  });
  searchInput.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      const first = search(allMetas, searchInput.value)[0];
      if (first) goTool(first.meta.id);
    } else if (e.key === 'Escape') {
      searchInput.value = '';
      searchResults.classList.remove('is-open');
    }
  });
  // 点击外部关闭
  document.addEventListener('click', (e: MouseEvent) => {
    if (!searchBox.contains(e.target as Node)) {
      searchResults.classList.remove('is-open');
    }
  });

  // ---- 最近使用 ----
  const recentIds = getRecent()
    .map((id) => findMeta(id))
    .filter((m): m is ToolMeta => m !== undefined);
  if (recentIds.length) {
    home.append(renderRecentSection(recentIds));
  }

  // ---- 分组宫格 ----
  const groupGrid = el('div', 'ftb-home-groupgrid');
  for (const group of GROUPS) {
    const metas = allMetas.filter((m) => m.groupId === group.id);
    if (!metas.length) continue;

    const card = el('div', 'ftb-group-card');
    card.dataset.groupId = group.id;

    const head = el('div', 'ftb-group-card-head');
    const titleWrap = el('div', 'ftb-group-card-title-wrap');
    titleWrap.append(
      el('div', 'ftb-group-card-title', group.name),
      el('div', 'ftb-group-card-count', `${metas.length} 个工具`),
    );
    head.append(
      el('span', 'ftb-group-card-icon', group.icon),
      titleWrap,
    );

    const blurb = el('div', 'ftb-group-card-blurb', group.blurb);
    const accent = el('div', 'ftb-group-card-accent');
    accent.style.background = `var(${group.theme})`;

    // 代表性图标（前 6 个工具的 icon）
    const icons = el('div', 'ftb-group-card-icons');
    for (const m of metas.slice(0, 6)) {
      icons.append(el('span', 'ftb-group-card-miniicon', m.icon ?? '🔧'));
    }

    card.append(accent, head, blurb, icons);
    // 点击组卡片 → 滚动到该组工具列表
    card.addEventListener('click', () => {
      const target = home.querySelector(`[data-group-tools="${group.id}"]`);
      target?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });

    groupGrid.append(card);
  }
  home.append(groupGrid);

  // ---- 各组工具详细列表 ----
  for (const group of GROUPS) {
    const metas = allMetas.filter((m) => m.groupId === group.id);
    if (!metas.length) continue;
    home.append(renderToolListSection(group, metas));
  }

  host.append(home);
}

/** 渲染最近使用区。 */
function renderRecentSection(metas: readonly ToolMeta[]): HTMLElement {
  const section = el('div', 'ftb-home-section');
  const head = el('div', 'ftb-home-section-head');
  head.append(
    el('span', 'ftb-home-section-icon', '🕒'),
    el('span', undefined, '最近使用'),
  );
  section.append(head);
  section.append(renderToolGrid(metas, undefined));
  return section;
}

/** 渲染一组工具的详细宫格（带锚点）。 */
function renderToolListSection(
  group: { id: string; name: string; icon: string; theme: string },
  metas: readonly ToolMeta[],
): HTMLElement {
  const section = el('div', 'ftb-home-section');
  section.dataset.groupTools = group.id;
  section.id = `group-${group.id}`;

  const head = el('div', 'ftb-home-section-head');
  head.append(
    el('span', 'ftb-home-section-icon', group.icon),
    el('span', undefined, group.name),
    el('a', 'ftb-home-back-top', '↑ 顶部'),
  );
  // 回顶部
  head.querySelector('.ftb-home-back-top')?.addEventListener('click', (e: Event) => {
    e.preventDefault();
    document.querySelector('.ftb-main')?.scrollTo({ top: 0, behavior: 'smooth' });
  });

  section.append(head);
  section.append(renderToolGrid(metas, group.theme));
  return section;
}

/** 渲染工具卡片宫格。 */
function renderToolGrid(metas: readonly ToolMeta[], themeVar?: string): HTMLElement {
  const grid = el('div', 'ftb-home-grid');
  for (const meta of metas) {
    const group = getGroup(meta.groupId);
    const card = el('div', 'ftb-card');
    const cardHead = el('div', 'ftb-card-head');
    cardHead.append(
      el('span', 'ftb-card-icon', meta.icon ?? '🔧'),
      el('span', 'ftb-card-title', meta.title),
    );
    card.append(cardHead, el('div', 'ftb-card-summary', meta.summary));
    const accent = el('div', 'ftb-card-accent');
    accent.style.background = `var(${themeVar ?? group?.theme ?? '--c-gray'})`;
    card.append(accent);

    card.addEventListener('click', () => goTool(meta.id));
    grid.append(card);
  }
  return grid;
}
