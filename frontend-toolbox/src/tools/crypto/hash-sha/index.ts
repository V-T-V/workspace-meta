import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el, kvRow } from '../../../ui/components.ts';
import { sha1, sha256, sha384, sha512 } from '../../../lib/hash.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('输入文本 …', 8);
        input.value = 'Hello, 前端工具箱！';

        const out = el('div');
        const update = async (): Promise<void> => {
          out.replaceChildren();
          const text = input.value;
          const [s1, s256, s384, s512] = await Promise.all([
            sha1(text),
            sha256(text),
            sha384(text),
            sha512(text),
          ]);
          out.append(
            kvRow('SHA-1   (160位)', s1),
            kvRow('SHA-256 (256位)', s256),
            kvRow('SHA-384 (384位)', s384),
            kvRow('SHA-512 (512位)', s512),
          );
        };
        let timer = 0;
        input.addEventListener('input', () => {
          clearTimeout(timer);
          timer = window.setTimeout(() => void update(), 150);
        });
        layout.inputArea.append(input);
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        void update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
