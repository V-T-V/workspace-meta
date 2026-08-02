import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el, kvRow } from '../../../ui/components.ts';
import { diffLines } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let a: HTMLTextAreaElement;
    let b: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        a = textarea('原文 …', 10);
        a.value = '第一行\n第二行\n第三行\n第四行';
        b = textarea('修改后 …', 10);
        b.value = '第一行\n第二行改\n第三行\n新增行\n第四行';

        const row = el('div', 'ftb-io-row');
        row.append(a, b);

        const stat = el('div');
        const diffBox = el('div', 'ftb-diff');

        const update = (): void => {
          const result = diffLines(a.value, b.value);
          diffBox.replaceChildren();
          let adds = 0,
            dels = 0;
          for (const line of result) {
            const div = el('div', `ftb-diff-line ftb-diff-line--${line.type}`, line.text || ' ');
            diffBox.append(div);
            if (line.type === 'add') adds++;
            else if (line.type === 'del') dels++;
          }
          stat.replaceChildren();
          stat.append(kvRow('新增', `+${adds}`), kvRow('删除', `-${dels}`), kvRow('总行数', String(result.length)));
        };
        a.addEventListener('input', update);
        b.addEventListener('input', update);

        layout.inputArea.append(row);
        layout.outputArea.append(stat, diffBox);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
