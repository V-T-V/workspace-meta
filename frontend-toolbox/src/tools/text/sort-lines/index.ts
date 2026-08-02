import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, select, el } from '../../../ui/components.ts';
import { sortLines, type SortMode } from '../../../lib/text-utils.ts';

const MODES: Array<[SortMode, string]> = [
  ['asc', '升序 A→Z'],
  ['desc', '降序 Z→A'],
  ['length-asc', '长度 短→长'],
  ['length-desc', '长度 长→短'],
  ['reverse', '反转顺序'],
  ['shuffle', '随机打乱'],
];

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let sel: HTMLSelectElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('每行一条 …', 10);
        input.value = 'banana\napple\ncherry\ndate\nelderberry';
        sel = select(MODES as Array<[string, string]>, 'asc') as unknown as HTMLSelectElement;

        const out = el('pre', 'ftb-codeblock-pre is-mono');
        const wrap = el('div', 'ftb-codeblock');
        wrap.append(out);
        const update = (): void => {
          out.textContent = sortLines(input.value, sel.value as SortMode);
        };
        input.addEventListener('input', update);
        sel.addEventListener('change', update);
        layout.inputArea.append(sel, input);
        layout.outputArea.append(wrap);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
