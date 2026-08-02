import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, checkbox, el, kvRow } from '../../../ui/components.ts';
import { dedupLines, removeEmptyLines, trimLines } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let caseCb: HTMLInputElement;
    let trimCb: HTMLInputElement;
    let emptyCb: HTMLInputElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('每行一条 …', 10);
        input.value = 'apple\nApple\nbanana\n apple \nbanana\ncherry';
        const { wrapper: w1, input: i1 } = checkbox('忽略大小写', false);
        const { wrapper: w2, input: i2 } = checkbox('比较时去首尾空白', true);
        const { wrapper: w3, input: i3 } = checkbox('同时移除空行', false);
        caseCb = i1;
        trimCb = i2;
        emptyCb = i3;

        const out = el('pre', 'ftb-codeblock-pre is-mono');
        const wrap = el('div', 'ftb-codeblock');
        wrap.append(out);
        const stat = el('div');

        const update = (): void => {
          let text = input.value;
          if (emptyCb.checked) text = removeEmptyLines(text);
          text = trimLines(text);
          const { output, removed } = dedupLines(text, caseCb.checked, trimCb.checked);
          out.textContent = output;
          stat.replaceChildren();
          const total = input.value.split('\n').length;
          stat.append(
            kvRow('原始行数', String(total)),
            kvRow('移除重复', String(removed)),
            kvRow('剩余行数', String(output.split('\n').length)),
          );
        };
        input.addEventListener('input', update);
        caseCb.addEventListener('change', update);
        trimCb.addEventListener('change', update);
        emptyCb.addEventListener('change', update);

        layout.inputArea.replaceChildren();
        const tb = el('div', 'ftb-toolbar');
        tb.append(w1, w2, w3);
        layout.inputArea.append(tb, input);
        layout.outputArea.append(stat, wrap);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
