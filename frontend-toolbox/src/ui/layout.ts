// =============================================================================
// 工具页通用布局 —— 面包屑 + 标题 + 描述 + 输入区 + 输出区
// 大多数「输入 → 处理 → 输出」型工具的骨架，省去每个工具重复搭结构。
// =============================================================================

import type { ToolMeta } from '../types.ts';
import { el, description } from './components.ts';
import { getGroup } from '../taxonomy.ts';
import { goHome } from '../core/router.ts';

export interface ToolLayout {
  container: HTMLElement;
  /** 输入区容器（供工具往里塞控件）。 */
  inputArea: HTMLElement;
  /** 输出区容器（供工具往里塞结果）。 */
  outputArea: HTMLElement;
}

/** 创建工具页骨架（含面包屑导航）。 */
export function createToolLayout(meta: ToolMeta): ToolLayout {
  const container = el('div', 'ftb-tool');

  // ---- 面包屑 ----
  const group = getGroup(meta.groupId);
  const breadcrumb = el('div', 'ftb-breadcrumb');
  const homeLink = el('a', 'ftb-breadcrumb-item', '首页');
  homeLink.href = '#/';
  homeLink.addEventListener('click', (e: Event) => {
    e.preventDefault();
    goHome();
  });
  breadcrumb.append(homeLink);
  if (group) {
    breadcrumb.append(el('span', 'ftb-breadcrumb-sep', '/'));
    breadcrumb.append(el('span', 'ftb-breadcrumb-item ftb-breadcrumb-group', `${group.icon} ${group.name}`));
  }
  breadcrumb.append(el('span', 'ftb-breadcrumb-sep', '/'));
  breadcrumb.append(el('span', 'ftb-breadcrumb-current', meta.title));
  container.append(breadcrumb);

  // ---- 标题 ----
  const head = el('div', 'ftb-tool-head');
  const titleRow = el('div', 'ftb-tool-titlerow');
  if (meta.icon) {
    const icon = el('span', 'ftb-tool-icon', meta.icon);
    titleRow.append(icon);
  }
  titleRow.append(el('h2', 'ftb-tool-title', meta.title));
  head.append(titleRow);
  if (meta.summary) head.append(description(meta.summary));
  container.append(head);

  const inputArea = el('div', 'ftb-tool-input');
  const outputArea = el('div', 'ftb-tool-output');
  container.append(inputArea, outputArea);

  return { container, inputArea, outputArea };
}

/** 创建「输入 → 输出」双栏布局（左右并列）。 */
export function createSplitLayout(meta: ToolMeta): ToolLayout {
  const layout = createToolLayout(meta);
  layout.container.querySelector('.ftb-tool-input')?.classList.add('is-split');
  return layout;
}
