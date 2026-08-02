import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el } from '../../../ui/components.ts';
import { renderMarkdownInline, renderMarkdown } from '../../../lib/markdown-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    let preview: HTMLElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('输入 Markdown …', 18);
        input.value = [
          '# 标题一',
          '',
          '这是一段 **加粗** 与 *斜体* 文本，含 `行内代码` 和 [链接](https://example.com)。',
          '',
          '## 列表',
          '',
          '- 项目 A',
          '- 项目 B',
          '- 项目 C',
          '',
          '## 代码块',
          '',
          '```js',
          'const x = 42;',
          'console.log(x);',
          '```',
          '',
          '> 引用文本。',
        ].join('\n');
        preview = el('div', 'ftb-md');
        preview.style.padding = '16px';
        preview.style.border = '1px solid var(--border)';
        preview.style.borderRadius = '8px';
        preview.style.background = 'var(--bg-elevated)';
        preview.style.lineHeight = '1.7';

        // 使用转义后的 innerHTML 设置（renderMarkdown 已转义）
        const update = (): void => {
          preview.innerHTML = renderMarkdown(input.value);
        };
        input.addEventListener('input', update);

        // 左右双栏
        const row = el('div', 'ftb-io-row');
        row.append(input, preview);
        layout.inputArea.append(row);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

// 引用以满足 isolatedModules（实际渲染函数在 lib）
void renderMarkdownInline;

export default tool;
