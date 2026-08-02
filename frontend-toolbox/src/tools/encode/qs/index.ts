import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, el, kvRow, errorBox } from '../../../ui/components.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴查询串（?a=1&b=2 或 a=1&b=2）…', 6);
        input.value = '?name=前端&tags=js&tags=ts&active=true';

        const parse = (): void => {
          layout.outputArea.replaceChildren();
          const raw = input.value.trim();
          if (!raw) return;
          try {
            const qs = raw.startsWith('?') ? raw.slice(1) : raw;
            const params = new URLSearchParams(qs);
            // 分组为对象（同名合并成数组）
            const obj: Record<string, string | string[]> = {};
            for (const key of new Set(params.keys())) {
              const all = params.getAll(key);
              obj[key] = all.length > 1 ? all : all[0]!;
            }
            layout.outputArea.append(el('div', 'ftb-desc', '解析结果：'));
            for (const [k, v] of Object.entries(obj)) {
              layout.outputArea.append(kvRow(k, Array.isArray(v) ? v.join(', ') : v));
            }
            // JSON 形式
            const json = el('div');
            json.append(el('div', 'ftb-desc', 'JSON 形式：'));
            const pre = el('pre', 'ftb-codeblock-pre is-mono');
            const code = el('code');
            code.textContent = JSON.stringify(obj, null, 2);
            pre.append(code);
            const wrap = el('div', 'ftb-codeblock');
            wrap.append(pre);
            json.append(wrap);
            layout.outputArea.append(json);
          } catch (e) {
            layout.outputArea.append(errorBox('解析失败：' + (e as Error).message));
          }
        };

        input.addEventListener('input', parse);
        layout.inputArea.append(input);
        ctx.container.append(layout.container);
        parse();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
