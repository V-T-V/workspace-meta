import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, select, el, kvRow, errorBox } from '../../../ui/components.ts';

const BASES: Array<[number, string]> = [
  [2, '二进制 (2)'],
  [8, '八进制 (8)'],
  [10, '十进制 (10)'],
  [16, '十六进制 (16)'],
];

const DIGITS = '0123456789abcdefghijklmnopqrstuvwxyz';

function toBase(value: bigint, base: number): string {
  if (value === 0n) return '0';
  const neg = value < 0n;
  let n = neg ? -value : value;
  let out = '';
  while (n > 0n) {
    out = DIGITS[Number(n % BigInt(base))] + out;
    n /= BigInt(base);
  }
  return neg ? '-' + out : out;
}

function fromBase(text: string, base: number): bigint {
  const neg = text.startsWith('-');
  const s = neg ? text.slice(1) : text;
  let result = 0n;
  const b = BigInt(base);
  for (const ch of s.toLowerCase()) {
    const v = DIGITS.indexOf(ch);
    if (v < 0 || v >= base) throw new Error(`字符「${ch}」在基 ${base} 中无效`);
    result = result * b + BigInt(v);
  }
  return neg ? -result : result;
}

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let valueInp: HTMLInputElement;
    let fromSel: HTMLSelectElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        valueInp = input('输入数值', '255');
        fromSel = select(
          BASES.map(([v, l]) => [String(v), l] as [string, string]),
          '10',
        );

        const out = el('div');
        const err = el('div');

        const update = (): void => {
          err.replaceChildren();
          const v = valueInp.value.trim();
          if (!v) {
            out.replaceChildren();
            return;
          }
          try {
            const from = Number(fromSel.value);
            const n = fromBase(v, from);
            out.replaceChildren();
            for (const [base, label] of BASES) {
              out.append(kvRow(label, toBase(n, base)));
            }
          } catch (e) {
            out.replaceChildren();
            err.append(errorBox('转换失败：' + (e as Error).message));
          }
        };
        valueInp.addEventListener('input', update);
        fromSel.addEventListener('change', update);
        layout.inputArea.append(fromSel, valueInp);
        layout.outputArea.append(err, out);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
