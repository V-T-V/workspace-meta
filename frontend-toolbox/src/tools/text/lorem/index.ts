import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, field } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { lorem } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const paraInp = input('', '3');
        paraInp.type = 'number';
        paraInp.min = '1';
        paraInp.max = '20';
        paraInp.style.width = '90px';
        const sentInp = input('', '5');
        sentInp.type = 'number';
        sentInp.min = '1';
        sentInp.max = '20';
        sentInp.style.width = '90px';

        const block = createCodeBlock({ copyable: true });

        const gen = (): void => {
          const p = Math.min(20, Math.max(1, Number(paraInp.value) || 1));
          const s = Math.min(20, Math.max(1, Number(sentInp.value) || 1));
          block.setText(lorem(p, s));
        };
        const btn = button('生成', gen);
        paraInp.addEventListener('input', gen);
        sentInp.addEventListener('input', gen);

        layout.inputArea.append(field('段落数', paraInp), field('每段句数', sentInp), btn);
        layout.outputArea.append(block.container);
        ctx.container.append(layout.container);
        gen();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
