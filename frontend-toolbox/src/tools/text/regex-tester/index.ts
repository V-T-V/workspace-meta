import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, el, kvRow, errorBox, checkbox } from '../../../ui/components.ts';
import { testRegex } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let patternInp: HTMLInputElement;
    let flagsInp: HTMLInputElement;
    let textArea: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        patternInp = input('正则（不含斜杠）', '\\d+');
        flagsInp = input('标志位', 'g');
        const { wrapper: gWrap, input: gCb } = checkbox('全局 g', true);
        const { wrapper: iWrap, input: iCb } = checkbox('忽略大小写 i', false);
        const { wrapper: mWrap, input: mCb } = checkbox('多行 m', false);

        textArea = document.createElement('textarea');
        textArea.className = 'ftb-textarea';
        textArea.rows = 8;
        textArea.value = '订单 12345 于 2024 年 1 月发货，金额 678.90 元，编号 999。';
        textArea.placeholder = '测试文本 …';

        const out = el('div');

        const buildFlags = (): string => {
          const f = new Set<string>();
          if (gCb.checked) f.add('g');
          if (iCb.checked) f.add('i');
          if (mCb.checked) f.add('m');
          return Array.from(f).join('');
        };

        const update = (): void => {
          out.replaceChildren();
          const flags = buildFlags();
          flagsInp.value = flags;
          const result = testRegex(patternInp.value, flags, textArea.value);
          if (result.error) {
            out.append(errorBox('正则错误：' + result.error));
            return;
          }
          if (result.matches.length === 0) {
            out.append(el('div', 'ftb-desc', '（无匹配）'));
            return;
          }
          out.append(kvRow('匹配数', String(result.matches.length)));
          for (const m of result.matches) {
            const card = el('div', 'ftb-stat');
            card.style.textAlign = 'left';
            const val = el('div', 'ftb-stat-value');
            val.style.fontSize = '14px';
            val.style.color = 'var(--success)';
            val.textContent = `「${m.match}」 @ ${m.index}`;
            card.append(val);
            if (m.groups.length) {
              const g = el('div', 'ftb-stat-label', '捕获组：' + m.groups.map((x, i) => `$${i + 1}="${x}"`).join('  '));
              card.append(g);
            }
            out.append(card);
          }
        };

        patternInp.addEventListener('input', update);
        textArea.addEventListener('input', update);
        gCb.addEventListener('change', update);
        iCb.addEventListener('change', update);
        mCb.addEventListener('change', update);

        const tb = el('div', 'ftb-toolbar');
        tb.append(patternInp, gWrap, iWrap, mWrap);
        layout.inputArea.append(tb, textArea, el('div', 'ftb-desc', `当前标志：`), flagsInp);
        flagsInp.readOnly = true;
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
