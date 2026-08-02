import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, toolbar, el } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { formatHTML } from '../../../lib/format-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let output: ReturnType<typeof createCodeBlock>;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴 HTML 片段 …');
        input.value = '<div class="wrap"><h1>标题</h1><p>段落<span>行内</span></p><ul><li>A</li><li>B</li></ul></div>';
        output = createCodeBlock({ lang: 'html' });
        const run = (): void => {
          try {
            output.setText(formatHTML(input.value, 2));
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
