import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import {
  textarea,
  button,
  select,
  checkbox,
  toolbar,
  el,
} from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import {
  tryParseJSON,
  formatJSON,
  minifyJSON,
  statsJSON,
  escapeJSONString,
} from '../../../lib/json-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let output: ReturnType<typeof createCodeBlock>;
    let indentSel: HTMLSelectElement;
    let sortCb: HTMLInputElement;

    const run = (mode: 'format' | 'minify'): void => {
      const parsed = tryParseJSON(input.value);
      if (!parsed.ok) {
        output.container.replaceChildren();
        const err = el('div', 'ftb-error', '⚠ 解析失败：' + parsed.error);
        output.container.append(err);
        return;
      }
      try {
        let text: string;
        if (mode === 'minify') {
          text = minifyJSON(input.value);
        } else {
          const indent = Number(indentSel.value);
          text = formatJSON(input.value, {
            indent,
            sortKeys: sortCb.checked,
          });
        }
        output.setText(text);
      } catch (e) {
        output.container.replaceChildren();
        output.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
      }
    };

    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('在此粘贴 JSON …', 14);
        input.value = '{\n  "name": "前端工具箱",\n  "version": "0.1.0",\n  "tools": ["json","base64","hash"]\n}';

        indentSel = select([
          ['2', '2 空格'],
          ['4', '4 空格'],
          ['0', '压缩（无空格）'],
          ['1', '1 个 Tab'],
        ], '2');

        // 用 0 表示压缩但走 format 分支；minify 走独立按钮
        const { wrapper: sortWrap, input: sortInput } = checkbox('按 key 排序', false);
        sortCb = sortInput;

        const formatBtn = button('格式化', () => run('format'));
        const minifyBtn = button('压缩', () => run('minify'), 'ghost');
        const escapeBtn = button('转义字符串', () => {
          output.setText(escapeJSONString(input.value));
        }, 'ghost');
        const clearBtn = button('清空', () => {
          input.value = '';
          output.setText('');
        }, 'ghost');

        const bar = toolbar(indentSel, sortWrap, formatBtn, minifyBtn, escapeBtn, clearBtn);

        output = createCodeBlock({ copyable: true, lang: 'json' });

        // 统计区
        const statsBox = el('div', 'ftb-stat-grid');
        const updateStats = (): void => {
          statsBox.replaceChildren();
          const parsed = tryParseJSON(input.value);
          if (!parsed.ok) return;
          const s = statsJSON(parsed.value);
          const items: Array<[string, string]> = [
            ['类型', s.type],
            ['深度', String(s.depth)],
            ['键数', String(s.keys)],
            ['数组项', String(s.arrayItems)],
            ['体积', s.size + ' B'],
          ];
          for (const [k, v] of items) {
            const stat = el('div', 'ftb-stat');
            stat.append(el('div', 'ftb-stat-value', v), el('div', 'ftb-stat-label', k));
            statsBox.append(stat);
          }
        };

        input.addEventListener('input', updateStats);

        layout.inputArea.append(bar, input);
        layout.outputArea.append(statsBox, output.container);

        ctx.container.append(layout.container);

        // 初始格式化一次
        run('format');
        updateStats();
      },
    };
  },
};

export default tool;
