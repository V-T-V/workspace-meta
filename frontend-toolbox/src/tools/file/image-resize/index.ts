import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, checkbox, button, el, kvRow } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { resizeImage, loadImage, scaleFit } from '../../../lib/image-utils.ts';
import { downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const wInp = input('', '800');
        wInp.type = 'number';
        wInp.style.width = '90px';
        const hInp = input('', '600');
        hInp.type = 'number';
        hInp.style.width = '90px';
        const { wrapper: ratioWrap, input: ratioCb } = checkbox('保持比例', true);

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '选择或拖入图片',
          onFiles: (files) => handle(files[0]!),
        });
        const result = el('div');
        let currentFile: File | null = null;
        let natW = 0,
          natH = 0;

        const handle = async (file: File): Promise<void> => {
          currentFile = file;
          const img = await loadImage(file);
          natW = img.naturalWidth;
          natH = img.naturalHeight;
          if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
          result.replaceChildren();
          result.append(kvRow('原图尺寸', `${natW} × ${natH} · ${formatBytes(file.size)}`));
        };

        const doResize = async (): Promise<void> => {
          if (!currentFile) return;
          let w = Number(wInp.value);
          let h = Number(hInp.value);
          if (!w || !h) return;
          if (ratioCb.checked) {
            const fit = scaleFit(natW || w, natH || h, w, h);
            w = fit.width;
            h = fit.height;
          }
          try {
            const blob = await resizeImage(currentFile, w, h);
            const img = el('img', 'ftb-image-preview') as HTMLImageElement;
            img.src = URL.createObjectURL(blob);
            result.replaceChildren(
              kvRow('输出', `${w} × ${h} · ${formatBytes(blob.size)}`),
              img,
              button('下载', () => {
                downloadBlob(blob, 'resized.png');
                toastSuccess('已开始下载');
              }),
            );
          } catch (e) {
            result.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const tb = el('div', 'ftb-toolbar');
        tb.append(
          Object.assign(el('label', undefined, '宽'), { style: 'font-size:13px' }),
          wInp,
          Object.assign(el('label', undefined, '高'), { style: 'font-size:13px' }),
          hInp,
          ratioWrap,
          button('缩放', doResize),
        );
        layout.inputArea.append(tb, drop.container);
        layout.outputArea.append(result);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
