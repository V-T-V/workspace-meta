import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { countText } from '../../../lib/text-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴文本 …', 12);
        input.value = '前端工具箱\n一个纯前端的开发者工具集合。\n包含 JSON、编码、文本等工具。';

        const grid = el('div', 'ftb-stat-grid');
        const order: Array<[keyof ReturnType<typeof countText>, string]> = [
          ['characters', '字符数'],
          ['charactersNoSpaces', '字符(无空格)'],
          ['words', '单词/词组'],
          ['lines', '行数'],
          ['paragraphs', '段落'],
          ['sentences', '句子'],
          ['bytes', '字节(UTF-8)'],
        ];
        const update = (): void => {
          const s = countText(input.value);
          grid.replaceChildren();
          for (const [key, label] of order) {
            const card = el('div', 'ftb-stat');
            card.append(el('div', 'ftb-stat-value', String(s[key])), el('div', 'ftb-stat-label', label));
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
