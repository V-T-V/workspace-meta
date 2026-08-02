import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { encodeBase64, decodeBase64 } from '../../../lib/codec.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let plain: HTMLTextAreaElement;
    let encoded: HTMLTextAreaElement;
    let lock = false; // 防循环
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        plain = textarea('明文 …', 10);
        encoded = textarea('Base64 …', 10);
        plain.value = 'Hello, 前端工具箱！';

        const plainErr = el('div');
        const encErr = el('div');

        plain.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          plainErr.replaceChildren();
          try {
            encoded.value = encodeBase64(plain.value);
          } catch (e) {
            plainErr.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
          lock = false;
        });
        encoded.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          encErr.replaceChildren();
          try {
            plain.value = decodeBase64(encoded.value);
          } catch (e) {
            encErr.append(el('div', 'ftb-error', '⚠ 解码失败：' + (e as Error).message));
          }
          lock = false;
        });

        const row = el('div', 'ftb-io-row');
        const left = el('div'); left.append(el('div', 'ftb-desc', '明文（UTF-8）'), plain, plainErr);
        const right = el('div'); right.append(el('div', 'ftb-desc', 'Base64'), encoded, encErr);
        row.append(left, right);
        layout.inputArea.append(row);
        ctx.container.append(layout.container);
        plain.dispatchEvent(new Event('input'));
      },
    } satisfies ToolInstance;
  },
};

export default tool;
