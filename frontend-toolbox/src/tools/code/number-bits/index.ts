import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, el, kvRow } from '../../../ui/components.ts';
import {
  toBinary32,
  radixStrings,
  parseIEEE754,
  formatIEEE754Bits,
  bitOperations,
} from '../../../lib/number-bits.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let inp: HTMLInputElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        inp = input('输入数字（整数或小数）', '42');
        inp.type = 'number';
        inp.step = 'any';

        const out = el('div');
        const update = (): void => {
          out.replaceChildren();
          const raw = inp.value.trim();
          const n = Number(raw);
          if (raw === '' || Number.isNaN(n)) {
            out.append(el('div', 'ftb-desc', '请输入有效数字'));
            return;
          }

          // 进制
          const sec1 = el('div');
          const r = radixStrings(n);
          sec1.append(el('div', 'ftb-desc', '各进制：'));
          sec1.append(
            kvRow('十进制', r.dec),
            kvRow('二进制', r.bin || '0'),
            kvRow('八进制', r.oct || '0'),
            kvRow('十六进制', r.hex || '0'),
          );

          // 32 位补码
          const sec2 = el('div');
          sec2.append(el('div', 'ftb-desc', '32 位补码：'));
          const bitsBox = el('div', 'ftb-bits-display');
          bitsBox.textContent = toBinary32(n | 0);
          sec2.append(bitsBox);

          // IEEE 754
          const sec3 = el('div');
          sec3.append(el('div', 'ftb-desc', 'IEEE 754 双精度（64 位）：'));
          const f = parseIEEE754(n);
          const ieeeBox = el('div', 'ftb-bits-display');
          ieeeBox.textContent = formatIEEE754Bits(f.fullBits);
          const ieeeParts = el('div', 'ftb-ieee-parts');
          ieeeParts.append(
            el('span', 'ftb-ieee-sign', `符号位: ${f.sign} (${f.sign === 0 ? '+' : '-'})`),
            el('span', 'ftb-ieee-exp', `指数: ${f.exponent} (${f.exponentBits})`),
            el('span', 'ftb-ieee-mant', `尾数: ${f.mantissaBits.slice(0, 20)}…`),
          );
          sec3.append(ieeeBox, ieeeParts, el('div', 'ftb-desc', f.description));

          // 位运算（需两个操作数）
          const sec4 = el('div');
          sec4.append(el('div', 'ftb-desc', '位运算（a = 该数, b = 10）：'));
          const ops = bitOperations(n | 0, 10);
          for (const op of ops) {
            const row = kvRow(op.expression, `${op.result}  ( ${op.resultBinary} )`);
            sec4.append(row);
          }

          out.append(sec1, sec2, sec3, sec4);
        };

        inp.addEventListener('input', update);
        layout.inputArea.append(inp);
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};
export default tool;
