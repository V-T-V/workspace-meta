import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, el, kvRow, field } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { cropImage, loadImage } from '../../../lib/image-utils.ts';
import { downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const xInp = input('', '0');
        const yInp = input('', '0');
        const wInp = input('', '200');
        const hInp = input('', '200');
        for (const inp of [xInp, yInp, wInp, hInp]) {
          inp.type = 'number';
          inp.style.width = '90px';
        }

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '选择或拖入图片',
          onFiles: (files) => handle(files[0]!),
        });
        const result = el('div');
        let currentFile: File | null = null;

        const handle = async (file: File): Promise<void> => {
          currentFile = file;
          const img = await loadImage(file);
          // 默认裁剪区为中间 200x200
          const cx = Math.max(0, Math.floor((img.naturalWidth - 200) / 2));
          const cy = Math.max(0, Math.floor((img.naturalHeight - 200) / 2));
          xInp.value = String(cx);
          yInp.value = String(cy);
          wInp.value = '200';
          hInp.value = '200';
          result.replaceChildren();
          result.append(kvRow('原图尺寸', `${img.naturalWidth} × ${img.naturalHeight}`));
          if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
        };

        const doCrop = async (): Promise<void> => {
          if (!currentFile) return;
          const rect = {
            x: Number(xInp.value),
            y: Number(yInp.value),
            width: Number(wInp.value),
            height: Number(hInp.value),
          };
          if (!rect.width || !rect.height) return;
          try {
            const blob = await cropImage(currentFile, rect);
            const img = el('img', 'ftb-image-preview') as HTMLImageElement;
            img.src = URL.createObjectURL(blob);
            result.replaceChildren(
              kvRow('裁剪结果', `${rect.width} × ${rect.height} · ${formatBytes(blob.size)}`),
              img,
              button('下载', () => {
                downloadBlob(blob, 'cropped.png');
                toastSuccess('已开始下载');
              }),
            );
          } catch (e) {
            result.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const tb = el('div', 'ftb-toolbar');
        tb.append(
          field('X', xInp),
          field('Y', yInp),
          field('宽', wInp),
          field('高', hInp),
          button('裁剪', doCrop),
        );
        layout.inputArea.append(tb, drop.container);
        layout.outputArea.append(result);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
