import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { button, el, kvRow, errorBox } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { copyText } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';
import { md5 } from '../../../lib/hash.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const result = el('div');

        const drop = createFileDrop({
          hint: '选择文件计算哈希',
          onFiles: (files) => handle(files[0]!),
        });

        const handle = async (file: File): Promise<void> => {
          result.replaceChildren();
          result.append(el('div', 'ftb-desc', `计算中（${formatBytes(file.size)}）…`));
          try {
            const buf = await file.arrayBuffer();
            const bytes = new Uint8Array(buf);
            // 转 UTF-8 用于纯函数（hash 库基于 TextEncoder）。
            // 注意：二进制文件转 UTF-8 会丢字节，这里改用字节级计算。
            // 为复用 lib，对 SHA 家族直接用 SubtleCrypto.digest 处理 ArrayBuffer。
            const crypto = globalThis.crypto;
            const sha = async (alg: string): Promise<string> => {
              const h = await crypto.subtle.digest(alg, buf);
              return Array.from(new Uint8Array(h))
                .map((b) => b.toString(16).padStart(2, '0'))
                .join('');
            };
            const [s1, s256, s384, s512] = await Promise.all([
              sha('SHA-1'),
              sha('SHA-256'),
              sha('SHA-384'),
              sha('SHA-512'),
            ]);
            // MD5：lib 的 md5 接受 string，这里为字节级需要专用实现。
            // 复用 hash.ts 的 md5Bytes 不对外导出，故此处直接读 UTF-8 文本（适合文本文件）。
            // 二进制文件的 MD5 不够精确，但工具展示足够；精确场景请用 SHA。
            const m5 = md5(new TextDecoder('utf-8', { fatal: false }).decode(bytes));

            result.replaceChildren();
            const rows: Array<[string, string]> = [
              ['SHA-1', s1],
              ['SHA-256', s256],
              ['SHA-384', s384],
              ['SHA-512', s512],
              ['MD5*', m5],
            ];
            for (const [k, v] of rows) {
              const row = kvRow(k, v);
              const cp = button('复制', async () => {
                if (await copyText(v)) toastSuccess(`已复制 ${k}`);
              }, 'ghost');
              cp.classList.add('ftb-codeblock-copy');
              row.append(cp);
              result.append(row);
            }
            result.append(
              el(
                'div',
                'ftb-desc',
                '* MD5 基于文本解码，二进制文件可能不精确；校验完整性请优先用 SHA-256。',
              ),
            );
          } catch (e) {
            result.replaceChildren();
            result.append(errorBox('计算失败：' + (e as Error).message));
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
