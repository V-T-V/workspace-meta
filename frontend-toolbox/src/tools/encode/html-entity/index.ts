import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { encodeHTML, decodeHTML } from '../../../lib/codec.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let plain: HTMLTextAreaElement;
    let encoded: HTMLTextAreaElement;
    let lock = false;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        plain = textarea('原文 …', 10);
        encoded = textarea('HTML 实体 …', 10);
        plain.value = '<div class="a">Tom & Jerry "x"</div>';

        const plainErr = el('div');
        const encErr = el('div');

        plain.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          plainErr.replaceChildren();
          encoded.value = encodeHTML(plain.value);
          lock = false;
        });
        encoded.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          encErr.replaceChildren();
          try {
            plain.value = decodeHTML(encoded.value);
          } catch (e) {
            encErr.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
          lock = false;
        });

        const row = el('div', 'ftb-io-row');
        const left = el('div'); left.append(el('div', 'ftb-desc', '原文'), plain, plainErr);
        const right = el('div'); right.append(el('div', 'ftb-desc', 'HTML 实体'), encoded, encErr);
        row.append(left, right);
        layout.inputArea.append(row);
        ctx.container.append(layout.container);
        plain.dispatchEvent(new Event('input'));
      },
    } satisfies ToolInstance;
  },
};

export default tool;
