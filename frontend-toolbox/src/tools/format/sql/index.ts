import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, toolbar, el } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { formatSQL } from '../../../lib/format-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let output: ReturnType<typeof createCodeBlock>;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴 SQL …');
        input.value = 'select id,name,email from users where age>18 and status="active" order by name';
        output = createCodeBlock({ lang: 'sql' });
        const run = (): void => {
          try {
            output.setText(formatSQL(input.value));
          } catch (e) {
            output.container.replaceChildren();
            output.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };
        const btn = button('美化', run);
        const clear = button('清空', () => { input.value = ''; output.setText(''); }, 'ghost');
        input.addEventListener('input', run);
        layout.inputArea.append(toolbar(btn, clear), input);
        layout.outputArea.append(output.container);
        ctx.container.append(layout.container);
        run();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
