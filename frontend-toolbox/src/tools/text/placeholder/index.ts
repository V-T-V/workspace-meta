import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, select, checkbox, field, el } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { randomString } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const lenInp = input('', '16');
        lenInp.type = 'number';
        lenInp.min = '1';
        lenInp.max = '128';
        lenInp.style.width = '90px';

        const charsetSel = select(
          [
            ['alphanumeric', '字母+数字 (A-Za-z0-9)'],
            ['lower', '小写字母+数字'],
            ['hex', '十六进制 (0-9a-f)'],
            ['full', '字母+数字+符号'],
          ],
          'alphanumeric',
        );

        const { wrapper: symWrap, input: symCb } = checkbox('包含符号 !@#$%', false);

        const block = createCodeBlock({ copyable: true });

        const pickCharset = (): string => {
          switch (charsetSel.value) {
            case 'lower':
              return 'abcdefghijklmnopqrstuvwxyz0123456789';
            case 'hex':
              return '0123456789abcdef';
            case 'full':
              return 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
            default:
              return symCb.checked
                ? 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*'
                : 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
          }
        };

        const gen = (): void => {
          const len = Math.min(128, Math.max(1, Number(lenInp.value) || 16));
          block.setText(randomString(len, pickCharset()));
        };
        const btn = button('生成', gen);
        lenInp.addEventListener('input', gen);
        charsetSel.addEventListener('change', gen);
        symCb.addEventListener('change', gen);

        const tb = el('div', 'ftb-toolbar');
        tb.append(charsetSel, symWrap);
        layout.inputArea.append(field('长度', lenInp), tb, btn);
        layout.outputArea.append(block.container);
        ctx.container.append(layout.container);
        gen();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
