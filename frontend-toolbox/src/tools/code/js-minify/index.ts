import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, checkbox, el, kvRow } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let inp: HTMLTextAreaElement;
    let out: ReturnType<typeof createCodeBlock>;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        inp = textarea('粘贴 JS 代码 …', 10);
        inp.value = [
          '// 计算两数之和',
          'function calculateSum(firstNumber, secondNumber) {',
          '  const result = firstNumber + secondNumber;',
          '  return result;',
          '}',
          'const total = calculateSum(10, 20);',
          'console.log("总和:", total);',
        ].join('\n');
        out = createCodeBlock({ lang: 'js' });

        const { wrapper: mangleWrap, input: mangleCb } = checkbox('变量名混淆', true);
        const stats = el('div');

        const minify = async (): Promise<void> => {
          try {
            const Babel = await import('@babel/standalone');
            const original = inp.value;
            const result = Babel.transform(original, {
              presets: [['env', { modules: false }]],
              comments: false,
              minified: true,
              compact: true,
              shouldPrintComment: () => false,
              // 基础混淆：压缩模式已自动缩短部分，这里额外用 mangle
              babelrc: false,
              configFile: false,
            });
            let code = result.code ?? '';
            // 简单变量名混淆：替换局部变量（保守，仅替换 var/let/const 声明的标识符）
            if (mangleCb.checked && code) {
              code = basicMangle(code);
            }
            out.setText(code);
            stats.replaceChildren();
            stats.append(
              kvRow('原始大小', original.length + ' 字符'),
              kvRow('压缩后', code.length + ' 字符'),
              kvRow('压缩率', ((1 - code.length / original.length) * 100).toFixed(1) + '%'),
            );
          } catch (e) {
            out.container.replaceChildren();
            out.container.append(el('div', 'ftb-error', '⚠ ' + (e as Error).message));
          }
        };

        const btn = button('🗜️ 压缩', () => void minify());
        const tb = el('div', 'ftb-toolbar');
        tb.append(btn, mangleWrap);
        layout.inputArea.append(tb, inp);
        layout.outputArea.append(stats, out.container);
        ctx.container.append(layout.container);
        void minify();
      },
    } satisfies ToolInstance;
  },
};

/** 基础变量名混淆：把 var/let/const/function 声明的标识符替换为 _0x开头短名。 */
function basicMangle(code: string): string {
  // 收集声明的标识符名
  const declared = new Set<string>();
  const declRe = /\b(?:var|let|const|function)\s+([a-zA-Z_$][\w$]*)/g;
  let m: RegExpExecArray | null;
  while ((m = declRe.exec(code)) !== null) {
    const name = m[1]!;
    // 跳过过短或保留字
    if (name.length > 2 && !JS_RESERVED.has(name)) declared.add(name);
  }
  // 生成映射
  const names = Array.from(declared);
  const map = new Map<string, string>();
  names.forEach((n, i) => map.set(n, '_0x' + i.toString(36)));
  // 替换（词边界）
  let result = code;
  for (const [orig, repl] of map) {
    result = result.replace(new RegExp(`\\b${orig}\\b`, 'g'), repl);
  }
  return result;
}

const JS_RESERVED = new Set([
  'break', 'case', 'catch', 'class', 'const', 'continue', 'debugger', 'default',
  'delete', 'do', 'else', 'export', 'extends', 'finally', 'for', 'function',
  'if', 'import', 'in', 'instanceof', 'new', 'return', 'super', 'switch',
  'this', 'throw', 'try', 'typeof', 'var', 'void', 'while', 'with', 'yield',
  'null', 'true', 'false', 'undefined', 'let', 'static',
]);

export default tool;
