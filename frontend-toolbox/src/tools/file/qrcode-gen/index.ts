import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, select, input, button, el } from '../../../ui/components.ts';
import { downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';
import QRCode from 'qrcode';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const textArea = textarea('输入要编码的文本或链接 …', 6);
        textArea.value = 'https://github.com/前端工具箱';
        const levelSel = select(
          [
            ['L', 'L 低 (7%)'],
            ['M', 'M 中 (15%)'],
            ['Q', 'Q 较高 (25%)'],
            ['H', 'H 高 (30%)'],
          ],
          'M',
        );
        const sizeInp = input('', '320');
        sizeInp.type = 'number';
        sizeInp.step = '16';
        sizeInp.style.width = '90px';

        const canvas = document.createElement('canvas');
        canvas.className = 'ftb-qr-canvas';

        const update = async (): Promise<void> => {
          const text = textArea.value;
          if (!text) {
            canvas.width = 0;
            canvas.height = 0;
            return;
          }
          try {
            const size = Number(sizeInp.value) || 256;
            await QRCode.toCanvas(canvas, text, {
              width: size,
              errorCorrectionLevel: levelSel.value as 'L' | 'M' | 'Q' | 'H',
              margin: 2,
              color: { dark: '#000000', light: '#ffffff' },
            });
          } catch {
            /* ignore */
          }
        };
        textArea.addEventListener('input', () => void update());
        levelSel.addEventListener('change', () => void update());
        sizeInp.addEventListener('input', () => void update());

        const dlBtn = button('下载 PNG', () => {
          canvas.toBlob((blob) => {
            if (blob) {
              downloadBlob(blob, 'qrcode.png');
              toastSuccess('已开始下载');
            }
          }, 'image/png');
        });

        const tb = el('div', 'ftb-toolbar');
        tb.append(levelSel, sizeInp, dlBtn);
        layout.inputArea.append(tb, textArea);
        layout.outputArea.append(canvas);
        ctx.container.append(layout.container);
        void update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
