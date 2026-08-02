// =============================================================================
// 代码块组件 —— 带复制按钮的只读文本展示
// =============================================================================

import { button } from './components.ts';
import { copyText } from './components.ts';
import { toastSuccess, toastError } from './toast.ts';

export interface CodeBlockOptions {
  /** 是否可复制（默认 true）。 */
  copyable?: boolean;
  /** 语言标签（显示在角落）。 */
  lang?: string;
  /** 是否等宽字体（默认 true）。 */
  mono?: boolean;
}

/** 创建一个带复制按钮的代码块。返回的容器带 setText 方法可复用。 */
export function createCodeBlock(opts: CodeBlockOptions = {}): {
  container: HTMLElement;
  setText: (text: string) => void;
} {
  const { copyable = true, lang, mono = true } = opts;
  const container = document.createElement('div');
  container.className = 'ftb-codeblock';

  const head = document.createElement('div');
  head.className = 'ftb-codeblock-head';
  if (lang) {
    const tag = document.createElement('span');
    tag.className = 'ftb-codeblock-lang';
    tag.textContent = lang;
    head.append(tag);
  }

  const pre = document.createElement('pre');
  pre.className = 'ftb-codeblock-pre' + (mono ? ' is-mono' : '');
  const code = document.createElement('code');
  pre.append(code);

  if (copyable) {
    const btn = button('复制', async () => {
      const text = code.textContent ?? '';
      const ok = await copyText(text);
      if (ok) toastSuccess('已复制');
      else toastError('复制失败');
    }, 'ghost');
    btn.classList.add('ftb-codeblock-copy');
    head.append(btn);
  }

  container.append(head, pre);

  return {
    container,
    setText(text: string): void {
      code.textContent = text;
    },
  };
}
