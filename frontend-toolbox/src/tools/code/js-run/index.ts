import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, el } from '../../../ui/components.ts';
import { transpileToES5, createRunner } from '../../../lib/js-run-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const codeArea = textarea('输入 JS 代码 …', 12);
        codeArea.value = [
          '// ES6+ 会被自动转译为 ES5 再执行',
          'const greet = (name) => `Hello, ${name}!`;',
          'const nums = [1, 2, 3].map(x => x * 2);',
          'console.log(greet("前端工具箱"));',
          'console.log(" doubled:", nums);',
        ].join('\n');

        const consoleBox = el('div', 'ftb-console-output');
        consoleBox.style.cssText = 'background:var(--code-bg);border:1px solid var(--border);border-radius:8px;padding:12px;min-height:120px;max-height:300px;overflow-y:auto;font-family:monospace;font-size:13px';

        const varBox = el('div');
        const statusText = el('span');
        statusText.style.cssText = 'font-size:12px;color:var(--text-muted)';

        const appendLine = (line: string, kind: 'log' | 'error' = 'log'): void => {
          const p = el('div');
          p.style.cssText = kind === 'error' ? 'color:var(--danger)' : 'color:var(--text)';
          p.textContent = kind === 'error' ? `✖ ${line}` : `› ${line}`;
          consoleBox.append(p);
          consoleBox.scrollTop = consoleBox.scrollHeight;
        };

        let runner: Awaited<ReturnType<typeof createRunner>> | null = null;

        // 运行
        const runBtn = button('▶️ 运行', async () => {
          consoleBox.replaceChildren();
          varBox.replaceChildren();
          statusText.textContent = '转译中…';
          const { code: es5, error } = await transpileToES5(codeArea.value);
          if (error) {
            appendLine('转译失败：' + error, 'error');
            statusText.textContent = '转译失败';
            return;
          }
          try {
            const r = await createRunner(es5, (line) => appendLine(line));
            runner = r;
            statusText.textContent = '运行中…';
            const result = r.run();
            if (result.error) appendLine(result.error, 'error');
            statusText.textContent = result.error ? '运行出错' : '运行完成';
            if (result.returnValue !== undefined && result.returnValue !== null) {
              appendLine('返回值: ' + String(result.returnValue));
            }
          } catch (e) {
            appendLine((e as Error).message, 'error');
            statusText.textContent = '出错';
          }
        });

        // 单步
        const stepBtn = button('⏭ 单步', async () => {
          if (!runner) {
            consoleBox.replaceChildren();
            const { code: es5, error } = await transpileToES5(codeArea.value);
            if (error) { appendLine('转译失败：' + error, 'error'); return; }
            runner = await createRunner(es5, (line) => appendLine(line));
          }
          const state = runner.step();
          statusText.textContent = state.done ? '执行完毕' : '已执行一步';
          varBox.replaceChildren();
          if (state.variables.length) {
            varBox.append(el('div', 'ftb-desc', '当前变量：'));
            for (const v of state.variables) {
              varBox.append(el('div', 'ftb-kv', `${v.name} = ${v.value}`));
            }
          }
        }, 'ghost');

        // 重置
        const resetBtn = button('🔄 重置', () => {
          runner = null;
          consoleBox.replaceChildren();
          varBox.replaceChildren();
          statusText.textContent = '';
        }, 'ghost');

        const clearBtn = button('🗑 清空', () => {
          codeArea.value = '';
          consoleBox.replaceChildren();
          varBox.replaceChildren();
          statusText.textContent = '';
        }, 'ghost');

        const tb = el('div', 'ftb-toolbar');
        tb.append(runBtn, stepBtn, resetBtn, clearBtn, statusText);

        const row = el('div', 'ftb-io-row');
        const left = el('div');
        left.append(el('div', 'ftb-desc', '代码：'), codeArea);
        const right = el('div');
        right.append(el('div', 'ftb-desc', 'Console 输出：'), consoleBox);
        row.append(left, right);

        layout.inputArea.append(tb, row);
        layout.outputArea.append(el('div', 'ftb-desc', '变量状态（单步时显示）：'), varBox);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};
export default tool;
