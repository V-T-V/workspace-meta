import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { button, el, errorBox, kvRow } from '../../../ui/components.ts';
import { createFileDrop } from '../../../ui/file-drop.ts';
import { copyText } from '../../../ui/components.ts';
import { toastSuccess, toastError } from '../../../ui/toast.ts';

// BarcodeDetector 类型（部分浏览器支持，Safari 较完整，Chrome 需 flags 或实验）
interface BarcodeDetectorLike {
  detect(source: ImageBitmap | CanvasImageSource): Promise<Array<{ rawValue: string; format: string }>>;
}

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const result = el('div');

        const drop = createFileDrop({
          accept: 'image/*',
          hint: '选择或拖入二维码图片',
          onFiles: (files) => handle(files[0]!),
        });

        const getDetector = (): BarcodeDetectorLike | null => {
          const G = globalThis as { BarcodeDetector?: new (opts: { formats: string[] }) => BarcodeDetectorLike };
          if (typeof G.BarcodeDetector === 'function') {
            try {
              return new G.BarcodeDetector({ formats: ['qr_code'] });
            } catch {
              return null;
            }
          }
          return null;
        };

        const handle = async (file: File): Promise<void> => {
          result.replaceChildren();
          const detector = getDetector();
          if (!detector) {
            result.append(
              errorBox(
                '当前浏览器不支持 BarcodeDetector API。建议使用 Chrome/Edge 或最新版 Safari，或在「二维码生成」工具反向使用。',
              ),
            );
            return;
          }
          try {
            const bitmap = await createImageBitmap(file);
            const codes = await detector.detect(bitmap);
            bitmap.close();
            if (codes.length === 0) {
              result.append(errorBox('未识别到二维码，请确认图片清晰且完整。'));
              return;
            }
            for (const code of codes) {
              result.append(
                kvRow('识别结果', code.rawValue),
                kvRow('格式', code.format),
                button('复制结果', async () => {
                  if (await copyText(code.rawValue)) toastSuccess('已复制');
                  else toastError('复制失败');
                }),
              );
              // 预览图片
              const img = el('img', 'ftb-image-preview') as HTMLImageElement;
              img.src = URL.createObjectURL(file);
              img.style.maxHeight = '200px';
              result.append(img);
            }
          } catch (e) {
            result.append(errorBox('识别失败：' + (e as Error).message));
          }
        };

        layout.inputArea.append(drop.container);
        layout.outputArea.append(result);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
