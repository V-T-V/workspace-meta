import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { encodeHex, decodeHex, encodeBinary } from '../../../lib/codec.ts';

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
        encoded = textarea('HEX …', 10);
        plain.value = '前端工具箱';

        const plainErr = el('div');
        const encErr = el('div');

        plain.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          plainErr.replaceChildren();
          encoded.value = encodeHex(plain.value);
          lock = false;
        });
        encoded.addEventListener('input', () => {
          if (lock) return;
          lock = true;
          encErr.replaceChildren();
          try {
            plain.value = decodeHex(encoded.value);
          } catch (e) {
            encErr.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
          lock = false;
        });

        const row = el('div', 'ftb-io-row');
        const left = el('div'); left.append(el('div', 'ftb-desc', '原文'), plain, plainErr);
        const right = el('div'); right.append(el('div', 'ftb-desc', 'HEX（UTF-8 字节）'), encoded, encErr);
        row.append(left, right);
        layout.inputArea.append(row);

        // 附赠：二进制视图
        const binOut = el('div', 'ftb-codeblock');
        binOut.style.marginTop = '12px';
        const binHead = el('div', 'ftb-codeblock-head', '', );
        binHead.append(el('span', 'ftb-codeblock-lang', '二进制（只读）'));
        binOut.append(binHead);
        const binPre = el('pre', 'ftb-codeblock-pre is-mono');
        const binCode = el('code');
        binPre.append(binCode);
        binOut.append(binPre);
        plain.addEventListener('input', () => {
          binCode.textContent = encodeBinary(plain.value);
        });
        layout.outputArea.append(el('div', 'ftb-desc', '字节二进制视图：'), binOut);

        ctx.container.append(layout.container);
        plain.dispatchEvent(new Event('input'));
      },
    } satisfies ToolInstance;
  },
};

export default tool;
