import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, select, el, kvRow } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import {
  parseCSV,
  prettyCSV,
  jsonToCSV,
  csvToJSON,
  detectDelimiter,
} from '../../../lib/csv-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let output: ReturnType<typeof createCodeBlock>;
    let modeSel: HTMLSelectElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴 CSV 或 JSON 数组 …', 10);
        input.value = 'name,age,city\nAlice,30,"Shanghai, China"\nBob,25,Beijing\n中文,28,"New York"';

        modeSel = select([
          ['pretty', '对齐美化'],
          ['to-json', 'CSV → JSON'],
          ['to-csv', 'JSON → CSV'],
          ['parse', '解析（标准CSV输出）'],
        ], 'pretty');

        output = createCodeBlock({ copyable: true });

        const run = (): void => {
          const text = input.value.trim();
          if (!text) { output.setText(''); return; }
          try {
            if (modeSel.value === 'to-csv') {
              output.setText(jsonToCSV(text));
            } else if (modeSel.value === 'to-json') {
              output.setText(csvToJSON(text));
            } else {
              const delim = detectDelimiter(text);
              const rows = parseCSV(text, delim);
              if (modeSel.value === 'pretty') {
                output.setText(prettyCSV(rows));
              } else {
                // parse: 重新标准输出
                output.setText(rows.map((r) => r.join(delim)).join('\n'));
              }
            }
          } catch (e) {
            output.container.replaceChildren();
            output.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const btn = button('转换', run);
        input.addEventListener('input', run);
        modeSel.addEventListener('change', run);

        // 检测分隔符信息
        const info = el('div');
        const updateInfo = (): void => {
          info.replaceChildren();
          const delim = detectDelimiter(input.value);
          const rows = parseCSV(input.value, delim);
          const cols = rows[0]?.length ?? 0;
          info.append(
            kvRow('检测分隔符', delim === '\t' ? 'Tab' : delim === ',' ? '逗号' : delim),
            kvRow('行数', String(rows.length)),
            kvRow('列数', String(cols)),
          );
        };
        input.addEventListener('input', updateInfo);

        const bar = el('div', 'ftb-toolbar');
        bar.append(modeSel, btn);

        layout.inputArea.append(bar, input);
        layout.outputArea.append(info, output.container);
        ctx.container.append(layout.container);
        run();
        updateInfo();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
