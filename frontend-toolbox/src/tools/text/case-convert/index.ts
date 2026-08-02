import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { convertCase, type CaseMode } from '../../../lib/text-utils.ts';

const MODES: Array<[CaseMode, string]> = [
  ['upper', 'UPPER CASE'],
  ['lower', 'lower case'],
  ['title', 'Title Case'],
  ['sentence', 'Sentence case'],
  ['capitalize', 'Capitalize Each'],
  ['camel', 'camelCase'],
  ['pascal', 'PascalCase'],
  ['snake', 'snake_case'],
  ['kebab', 'kebab-case'],
  ['constant', 'CONSTANT_CASE'],
];

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('输入文本 …', 8);
        input.value = 'hello world from 前端工具箱';

        // 全量展示所有模式结果，无需选择器
        const grid = el('div', 'ftb-stat-grid');
        grid.style.gridTemplateColumns = 'repeat(auto-fill, minmax(260px, 1fr))';
        const update = (): void => {
          grid.replaceChildren();
          for (const [mode, label] of MODES) {
            const card = el('div', 'ftb-stat');
            card.style.textAlign = 'left';
            const val = el('div', 'ftb-stat-value');
            val.style.fontSize = '14px';
            val.style.wordBreak = 'break-all';
            val.textContent = convertCase(input.value, mode);
            const lab = el('div', 'ftb-stat-label', label);
            card.append(val, lab);
            grid.append(card);
          }
        };
        input.addEventListener('input', update);
        layout.inputArea.append(input);
        layout.outputArea.append(grid);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
