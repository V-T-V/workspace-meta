import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, select, el, kvRow } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { compressImage, type ImageFormat } from '../../../lib/image-utils.ts';
import { downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const maxWidthInp = input('', '1280');
        maxWidthInp.type = 'number';
        maxWidthInp.style.width = '90px';
        const qualityInp = input('', '0.7');
        qualityInp.type = 'number';
        qualityInp.step = '0.05';
        qualityInp.min = '0.1';
        qualityInp.max = '1';
        qualityInp.style.width = '90px';

        const formatSel = select(
          [
            ['image/jpeg', 'JPG（有损）'],
            ['image/png8', 'PNG 256色（缩小80%）'],
            ['image/png-best', 'PNG 无损优化'],
            ['image/webp', 'WebP'],
            ['image/png', 'PNG 原样'],
          ],
          'image/jpeg',
        );

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '选择或拖入图片（PNG / JPG / WebP）',
          onFiles: (files) => handle(files[0]!),
        });

        const result = el('div');
        let lastBlob: Blob | null = null;
        let lastName = 'compressed.jpg';

        const handle = async (file: File): Promise<void> => {
          result.replaceChildren();
          const maxW = Number(maxWidthInp.value) || undefined;
          const q = Number(qualityInp.value) || 0.7;
          const fmt = formatSel.value as ImageFormat;
          try {
            const { blob, width, height } = await compressImage(file, {
              maxWidth: maxW,
              quality: q,
              format: fmt,
            });
            lastBlob = blob;
            const ext = fmt.startsWith('image/png') ? 'png' : fmt === 'image/webp' ? 'webp' : 'jpg';
            lastName = file.name.replace(/\.(png|jpe?g|webp)$/i, '') + '-compressed.' + ext;

            const before = el('div');
            before.append(el('div', 'ftb-desc', '原图：'));
            const beforeImg = el('img', 'ftb-image-preview') as HTMLImageElement;
            beforeImg.src = URL.createObjectURL(file);
            before.append(beforeImg);
            before.append(kvRow('原始大小', formatBytes(file.size)));

            const after = el('div');
            after.append(el('div', 'ftb-desc', '压缩后：'));
            const afterImg = el('img', 'ftb-image-preview') as HTMLImageElement;
            afterImg.src = URL.createObjectURL(blob);
            after.append(afterImg);
            after.append(
              kvRow('压缩后大小', formatBytes(blob.size)),
              kvRow('节省', `${((1 - blob.size / file.size) * 100).toFixed(1)}%`),
              kvRow('输出尺寸', `${width} × ${height}`),
            );

            const dl = button('下载压缩图', () => {
              if (lastBlob) {
                downloadBlob(lastBlob, lastName);
                toastSuccess('已开始下载');
              }
            });

            const row = el('div', 'ftb-io-row');
            row.append(before, after);
            result.append(row, dl);
          } catch (e) {
            result.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const tb = el('div', 'ftb-toolbar');
        tb.append(
          Object.assign(el('label', undefined, '格式'), { style: 'font-size:13px;margin-right:4px' }),
          formatSel,
          Object.assign(el('label', undefined, '最大宽度'), { style: 'font-size:13px;margin:0 4px 0 12px' }),
          maxWidthInp,
          Object.assign(el('label', undefined, '质量'), { style: 'font-size:13px;margin:0 4px' }),
          qualityInp,
        );
        layout.inputArea.append(tb, drop.container);
        layout.outputArea.append(result);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
