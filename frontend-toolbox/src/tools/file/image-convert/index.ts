import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { select, input, button, el, kvRow } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { convertImage, type ImageFormat } from '../../../lib/image-utils.ts';
import { downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const fmtSel = select(
          [
            ['image/png', 'PNG'],
            ['image/jpeg', 'JPG'],
            ['image/webp', 'WebP'],
          ],
          'image/webp',
        );
        const qualityInp = input('', '0.9');
        qualityInp.type = 'number';
        qualityInp.step = '0.05';
        qualityInp.min = '0.1';
        qualityInp.max = '1';
        qualityInp.style.width = '90px';

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '选择或拖入图片',
          onFiles: (files) => handle(files[0]!),
        });
        const result = el('div');

        const handle = async (file: File): Promise<void> => {
          result.replaceChildren();
          try {
            const fmt = fmtSel.value as ImageFormat;
            const q = Number(qualityInp.value) || 0.9;
            const blob = await convertImage(file, fmt, q);
            const ext = fmt === 'image/png' ? 'png' : fmt === 'image/jpeg' ? 'jpg' : 'webp';
            const name = file.name.replace(/\.(png|jpe?g|webp)$/i, '') + '.' + ext;

            const img = el('img', 'ftb-image-preview') as HTMLImageElement;
            img.src = URL.createObjectURL(blob);
            result.append(
              img,
              kvRow('原格式', file.type + ' · ' + formatBytes(file.size)),
              kvRow('新格式', fmt + ' · ' + formatBytes(blob.size)),
              kvRow('变化', `${((blob.size / file.size - 1) * 100).toFixed(1)}%`),
              button('下载', () => {
                downloadBlob(blob, name);
                toastSuccess('已开始下载');
              }),
            );
          } catch (e) {
            result.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const tb = el('div', 'ftb-toolbar');
        tb.append(fmtSel, qualityInp);
        layout.inputArea.append(tb, drop.container);
        layout.outputArea.append(result);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
