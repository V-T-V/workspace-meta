import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el, kvRow } from '../../../ui/components.ts';
import { md5 } from '../../../lib/hash.ts';

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
        const update = (): void => {
          out.replaceChildren();
          const hash = md5(input.value);
          out.append(
            kvRow('MD5 (32 位)', hash),
            kvRow('MD5 (16 位)', hash.slice(8, 24)),
            kvRow('MD5 (Base64)', btoa(hash)),
            kvRow('长度', '128 位 / 16 字节'),
          );
        };
        input.addEventListener('input', update);
        layout.inputArea.append(input);
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
