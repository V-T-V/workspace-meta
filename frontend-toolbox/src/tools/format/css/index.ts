import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, toolbar, el } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { formatCSS, minifyCSS } from '../../../lib/format-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let output: ReturnType<typeof createCodeBlock>;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴 CSS …');
        input.value = '.card{padding:14px;border-radius:8px;background:#fff;color:#333;box-shadow:0 2px 6px rgba(0,0,0,.1)}';
        output = createCodeBlock({ lang: 'css' });
        const beautify = button('美化', () => update('beautify'), 'primary');
        const minify = button('压缩', () => update('minify'), 'ghost');
        const clear = button('清空', () => { input.value = ''; output.setText(''); }, 'ghost');
        const update = (mode: 'beautify' | 'minify'): void => {
          try {
            output.setText(mode === 'beautify' ? formatCSS(input.value) : minifyCSS(input.value));
          } catch (e) {
            output.container.replaceChildren();
            output.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };
        layout.inputArea.append(toolbar(beautify, minify, clear), input);
        layout.outputArea.append(output.container);
        ctx.container.append(layout.container);
        update('beautify');
      },
    } satisfies ToolInstance;
  },
};

export default tool;
