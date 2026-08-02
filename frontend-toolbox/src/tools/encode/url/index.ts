import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, select, el, toolbar } from '../../../ui/components.ts';
import { encodeURL, decodeURL, encodeURLComponentAll } from '../../../lib/codec.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let plain: HTMLTextAreaElement;
    let encoded: HTMLTextAreaElement;
    let modeSel: HTMLSelectElement;
    let lock = false;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        plain = textarea('原文 …', 10);
        encoded = textarea('URL 编码 …', 10);
        plain.value = 'https://example.com/搜索?q=前端 工具';
        modeSel = select([
          ['standard', '标准（保留字母数字与常见符号）'],
          ['all', '全字符编码（每个字符都转义）'],
        ], 'standard');

        const plainErr = el('div');
        const encErr = el('div');

        const encode = (s: string): string =>
          modeSel.value === 'all' ? encodeURLComponentAll(s) : encodeURL(s);

        plain.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          plainErr.replaceChildren();
          encoded.value = encode(plain.value);
          lock = false;
        });
        encoded.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          encErr.replaceChildren();
          try {
            plain.value = decodeURL(encoded.value);
          } catch (e) {
            encErr.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
          lock = false;
        });
        modeSel.addEventListener('change', () => plain.dispatchEvent(new Event('input')));

        const row = el('div', 'ftb-io-row');
        const left = el('div'); left.append(el('div', 'ftb-desc', '原文'), plain, plainErr);
        const right = el('div'); right.append(el('div', 'ftb-desc', 'URL 编码'), encoded, encErr);
        row.append(left, right);
        layout.inputArea.append(toolbar(modeSel), row);
        ctx.container.append(layout.container);
        plain.dispatchEvent(new Event('input'));
      },
    } satisfies ToolInstance;
  },
};

export default tool;
