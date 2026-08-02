// =============================================================================
// 通用可折叠树组件 —— 用于 AST 可视化等场景
// =============================================================================

import { el } from './components.ts';

export interface TreeNodeData {
  label: string;
  /** 节点类型标签（显示在 label 旁，可选）。 */
  typeTag?: string;
  /** 点击回调。 */
  onClick?: () => void;
  /** 子节点。 */
  children?: TreeNodeData[];
  /** 是否默认展开。 */
  expanded?: boolean;
  /** 额外数据（用于高亮等）。 */
  data?: unknown;
}

export interface TreeViewHandle {
  container: HTMLElement;
  /** 高亮指定节点。 */
  highlight: (data: unknown) => void;
}

/** 创建一棵可折叠树。 */
export function createTreeView(root: TreeNodeData): TreeViewHandle {
  const container = el('div', 'ftb-tree');
  let highlightCallback: ((data: unknown) => void) | null = null;

  function renderNode(node: TreeNodeData, depth: number): HTMLElement {
    const wrap = el('div', 'ftb-tree-node');
    wrap.dataset.depth = String(depth);

    const row = el('div', 'ftb-tree-row');
    row.style.paddingLeft = `${depth * 16 + 8}px`;

    const hasChildren = node.children && node.children.length > 0;
    const toggle = el('span', 'ftb-tree-toggle');
    toggle.textContent = hasChildren ? (node.expanded ? '▼' : '▶') : '·';

    const labelEl = el('span', 'ftb-tree-label', node.label);
    if (node.typeTag) {
      const tag = el('span', 'ftb-tree-tag', node.typeTag);
      row.append(toggle, tag, labelEl);
    } else {
      row.append(toggle, labelEl);
    }

    if (node.onClick) {
      row.style.cursor = 'pointer';
      row.addEventListener('click', (e) => {
        if (e.target === toggle && hasChildren) return;
        node.onClick?.();
        highlightCallback?.(node.data);
      });
      row.addEventListener('mouseenter', () => row.classList.add('is-hover'));
      row.addEventListener('mouseleave', () => row.classList.remove('is-hover'));
    }

    let childWrap: HTMLElement | null = null;
    if (hasChildren) {
      childWrap = el('div', 'ftb-tree-children');
      for (const child of node.children!) {
        childWrap.append(renderNode(child, depth + 1));
      }
      if (!node.expanded) childWrap.style.display = 'none';

      toggle.style.cursor = 'pointer';
      toggle.addEventListener('click', (e) => {
        e.stopPropagation();
        const collapsed = childWrap!.style.display === 'none';
        childWrap!.style.display = collapsed ? '' : 'none';
        toggle.textContent = collapsed ? '▼' : '▶';
      });
    }

    wrap.append(row);
    if (childWrap) wrap.append(childWrap);
    return wrap;
  }

  container.append(renderNode(root, 0));

  return {
    container,
    highlight(data) {
      highlightCallback = null; // 清旧
      container.querySelectorAll('.ftb-tree-row.is-active').forEach((r) => r.classList.remove('is-active'));
      // 找到匹配 data 的行
      const rows = container.querySelectorAll('.ftb-tree-row');
      rows.forEach((r) => {
        const nodeEl = r.parentElement;
        if (nodeEl?.dataset.nodeData === String(data)) {
          r.classList.add('is-active');
        }
      });
    },
  };
}
