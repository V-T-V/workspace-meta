import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el, kvRow } from '../../../ui/components.ts';
import { encodeUnicodeEscape, decodeUnicodeEscape } from '../../../lib/codec.ts';

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
        encoded = textarea('Unicode 转义 …', 10);
        plain.value = '前端工具箱 🧰';

        const plainErr = el('div');
        const encErr = el('div');

        plain.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          plainErr.replaceChildren();
          encoded.value = encodeUnicodeEscape(plain.value);
          lock = false;
        });
        encoded.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          encErr.replaceChildren();
          try {
            plain.value = decodeUnicodeEscape(encoded.value);
          } catch (e) {
            encErr.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
          lock = false;
        });

        const row = el('div', 'ftb-io-row');
        const left = el('div'); left.append(el('div', 'ftb-desc', '原文'), plain, plainErr);
        const right = el('div'); right.append(el('div', 'ftb-desc', 'Unicode 转义'), encoded, encErr);
        row.append(left, right);
        layout.inputArea.append(row);

        // 码点表
        const cpBox = el('div');
        const updateCp = (): void => {
          cpBox.replaceChildren();
          cpBox.append(el('div', 'ftb-desc', '逐字符码点：'));
          for (const ch of plain.value) {
            const cp = ch.codePointAt(0)!;
            cpBox.append(
              kvRow(ch === ' ' ? '␣' : ch, `U+${cp.toString(16).toUpperCase().padStart(4, '0')} (${cp})`),
            );
          }
        };
        plain.addEventListener('input', updateCp);
        layout.outputArea.append(cpBox);

        ctx.container.append(layout.container);
        plain.dispatchEvent(new Event('input'));
      },
    } satisfies ToolInstance;
  },
};

export default tool;
