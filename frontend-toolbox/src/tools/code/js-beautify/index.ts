import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, el } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let inp: HTMLTextAreaElement;
    let out: ReturnType<typeof createCodeBlock>;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        inp = textarea('粘贴 JS 代码 …', 10);
        inp.value = 'const f=(a,b)=>a+b;const r=f(1,2);console.log(r)';
        out = createCodeBlock({ lang: 'js' });

        const beautify = async (): Promise<void> => {
          try {
            const [{ generate }, { parseCode }] = await Promise.all([
              import('@babel/generator'),
              import('../../../lib/ast-utils.ts'),
            ]);
            const ast = await parseCode(inp.value);
            const result = generate(ast, {
              retainLines: false,
              comments: true,
              jsescOption: { quotes: 'single' },
            });
            out.setText(result.code);
          } catch (e) {
            out.container.replaceChildren();
            out.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };
        const btn = button('美化', () => void beautify());
        layout.inputArea.append(btn, inp);
        layout.outputArea.append(out.container);
        ctx.container.append(layout.container);
        void beautify();
      },
    } satisfies ToolInstance;
  },
};
export default tool;
