// =============================================================================
// UI 原语 —— 跨工具复用的小组件工厂（纯 DOM，无框架）
// 全部返回 HTMLElement，命名空间前缀 ftb- 避免冲突。
// =============================================================================

/** 创建带 class 的元素简写。 */
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

/** 创建按钮。 */
export function button(
  label: string,
  onClick: () => void,
  variant: 'primary' | 'ghost' | 'danger' = 'primary',
): HTMLButtonElement {
  const btn = el('button', `ftb-btn ftb-btn--${variant}`, label);
  btn.type = 'button';
  btn.addEventListener('click', onClick);
  return btn;
}

/** 创建多行文本输入框。 */
export function textarea(
  placeholder = '',
  rows = 10,
): HTMLTextAreaElement {
  const ta = el('textarea', 'ftb-textarea');
  ta.placeholder = placeholder;
  ta.rows = rows;
  ta.spellcheck = false;
  return ta;
}

/** 创建单行输入框。 */
export function input(placeholder = '', value = ''): HTMLInputElement {
  const inp = el('input', 'ftb-input');
  inp.placeholder = placeholder;
  inp.value = value;
  return inp;
}

/** 创建下拉选择。options: [value,label][]。 */
export function select(options: Array<[string, string]>, selected?: string): HTMLSelectElement {
  const sel = el('select', 'ftb-select');
  for (const [value, label] of options) {
    const opt = el('option', undefined, label);
    opt.value = value;
    if (value === selected) opt.selected = true;
    sel.append(opt);
  }
  return sel;
}

/** 创建复选框 + 标签组合。 */
export function checkbox(label: string, checked = false): {
  wrapper: HTMLLabelElement;
  input: HTMLInputElement;
} {
  const wrapper = el('label', 'ftb-checkbox');
  const inp = el('input') as HTMLInputElement;
  inp.type = 'checkbox';
  inp.checked = checked;
  const span = el('span', undefined, label);
  wrapper.append(inp, span);
  return { wrapper, input: inp };
}

/** 创建字段行（label + control）。 */
export function field(label: string, control: HTMLElement): HTMLDivElement {
  const row = el('div', 'ftb-field');
  row.append(el('label', 'ftb-field-label', label), control);
  return row;
}

/** 创建工具栏（横向排列按钮/选择）。 */
export function toolbar(...children: HTMLElement[]): HTMLDivElement {
  const bar = el('div', 'ftb-toolbar');
  bar.append(...children);
  return bar;
}

/** 创建键值对展示行。 */
export function kvRow(key: string, value: string): HTMLDivElement {
  const row = el('div', 'ftb-kv');
  row.append(el('span', 'ftb-kv-key', key), el('span', 'ftb-kv-val', value));
  return row;
}

/** 创建描述段落。 */
export function description(text: string): HTMLParagraphElement {
  return el('p', 'ftb-desc', text);
}

/** 创建错误提示框。 */
export function errorBox(text: string): HTMLDivElement {
  return el('div', 'ftb-error', '⚠ ' + text);
}

/** 复制文本到剪贴板，返回是否成功。 */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // 降级方案
    try {
      const ta = el('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.append(ta);
      ta.select();
      const ok = document.execCommand('copy');
      ta.remove();
      return ok;
    } catch {
      return false;
    }
  }
}

/** 触发下载（Blob → 文件）。 */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = el('a');
  a.href = url;
  a.download = filename;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
