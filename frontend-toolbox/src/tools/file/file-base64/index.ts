import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, el, kvRow, errorBox } from '../../../ui/components.ts';
import { createFileDrop, formatBytes } from '../../../ui/file-drop.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { copyText, downloadBlob } from '../../../ui/components.ts';
import { toastSuccess } from '../../../ui/toast.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);

        // 编码：文件 → base64
        const encDrop = createFileDrop({
          hint: '选择文件 → 生成 Base64',
          onFiles: (files) => encode(files[0]!),
        });
        const encOut = createCodeBlock({ lang: 'Data URL', copyable: false });
        const encInfo = el('div');
        const encode = (file: File): void => {
          const reader = new FileReader();
          reader.onload = (): void => {
            const result = reader.result as string;
            encOut.setText(result);
            encInfo.replaceChildren();
            encInfo.append(
              kvRow('文件名', file.name),
              kvRow('类型', file.type || '未知'),
              kvRow('大小', formatBytes(file.size)),
              kvRow('Base64 长度', String(result.length)),
              button('复制', async () => {
                if (await copyText(result)) toastSuccess('已复制');
              }),
            );
          };
          reader.onerror = (): void => encInfo.append(errorBox('读取失败'));
          reader.readAsDataURL(file);
        };

        // 解码：base64 → 文件
        const decInput = textarea('粘贴 Data URL 或纯 Base64 …', 8);
        const decOut = el('div');
        const decode = (): void => {
          decOut.replaceChildren();
          let dataUrl = decInput.value.trim();
          if (!dataUrl) return;
          // 若无前缀，默认当作 octet-stream
          if (!dataUrl.startsWith('data:')) {
            dataUrl = 'data:application/octet-stream;base64,' + dataUrl;
          }
          try {
            const m = dataUrl.match(/^data:([^;]+);base64,(.*)$/s);
            if (!m) {
              decOut.append(errorBox('格式无效，应为 data:<mime>;base64,<data>'));
              return;
            }
            const mime = m[1]!;
            const b64 = m[2]!;
            const binary = atob(b64);
            const bytes = new Uint8Array(binary.length);
            for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
            const blob = new Blob([bytes], { type: mime });
            const ext = mime.split('/')[1]?.split(';')[0] ?? 'bin';
            decOut.append(
              kvRow('MIME', mime),
              kvRow('大小', formatBytes(blob.size)),
              button('下载文件', () => {
                downloadBlob(blob, 'decoded.' + ext);
                toastSuccess('已开始下载');
              }),
            );
          } catch (e) {
            decOut.append(errorBox('解码失败：' + (e as Error).message));
          }
        };
        const decBtn = button('解码', decode);

        const encodeSection = el('div');
        encodeSection.append(
          el('div', 'ftb-desc', '① 文件转 Base64：'),
          encDrop.container,
          encInfo,
          encOut.container,
        );
        const decodeSection = el('div');
        decodeSection.style.marginTop = '24px';
        decodeSection.append(el('div', 'ftb-desc', '② Base64 转文件：'), decBtn, decInput, decOut);

        layout.inputArea.append(encodeSection, decodeSection);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
