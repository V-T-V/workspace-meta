import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { select, description } from '../../../ui/components.ts';
import { createFileDrop } from '../../../ui/file-drop.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';

// ASCII 字符，从亮到暗（空格最亮）
const RAMP = ' .:-=+*#%@';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const widthSel = select(
          [
            ['60', '窄 (60 字符)'],
            ['100', '中 (100 字符)'],
            ['140', '宽 (140 字符)'],
            ['200', '超宽 (200 字符)'],
          ],
          '100',
        );

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '点击选择或拖入图片',
          onFiles: (files) => handle(files[0]!),
        });

        const block = createCodeBlock({ copyable: true });
        block.container.querySelector('pre')?.style.setProperty('line-height', '1.05');
        block.container.querySelector('pre')?.style.setProperty('letter-spacing', '0');

        const handle = (file: File): void => {
          const url = URL.createObjectURL(file);
          const img = new Image();
          img.onload = (): void => {
            const maxW = Number(widthSel.value);
            const ratio = (img.naturalHeight / img.naturalWidth) * 0.5; // 字符高宽比校正
            const w = Math.min(maxW, img.naturalWidth);
            const h = Math.floor(w * ratio);
            const canvas = document.createElement('canvas');
            canvas.width = w;
            canvas.height = h;
            const c = canvas.getContext('2d');
            if (!c) return;
            c.drawImage(img, 0, 0, w, h);
            const data = c.getImageData(0, 0, w, h).data;
            let out = '';
            for (let y = 0; y < h; y++) {
              for (let x = 0; x < w; x++) {
                const i = (y * w + x) * 4;
                const r = data[i]!,
                  g = data[i + 1]!,
                  b = data[i + 2]!,
                  a = data[i + 3]!;
                const luma = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
                const alpha = a / 255;
                const idx = Math.floor((1 - luma * alpha) * (RAMP.length - 1));
                out += RAMP[Math.max(0, Math.min(RAMP.length - 1, idx))];
              }
              out += '\n';
            }
            block.setText(out);
            URL.revokeObjectURL(url);
          };
          img.src = url;
        };

        widthSel.addEventListener('change', () => {
          /* 需重新选择图片触发 */
        });

        layout.inputArea.append(widthSel, drop.container);
        layout.outputArea.append(description('转换结果（等宽字体查看效果最佳）：'), block.container);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
