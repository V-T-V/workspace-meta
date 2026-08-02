import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { el } from '../../../ui/components.ts';

const CONTROL_NAMES: Readonly<Record<number, string>> = {
  0: 'NUL', 1: 'SOH', 2: 'STX', 3: 'ETX', 4: 'EOT', 5: 'ENQ', 6: 'ACK', 7: 'BEL',
  8: 'BS', 9: 'TAB', 10: 'LF', 11: 'VT', 12: 'FF', 13: 'CR', 14: 'SO', 15: 'SI',
  16: 'DLE', 17: 'DC1', 18: 'DC2', 19: 'DC3', 20: 'DC4', 21: 'NAK', 22: 'SYN', 23: 'ETB',
  24: 'CAN', 25: 'EM', 26: 'SUB', 27: 'ESC', 28: 'FS', 29: 'GS', 30: 'RS', 31: 'US',
  127: 'DEL',
};

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const table = el('table');
        table.className = 'ftb-md';
        table.style.fontSize = '12px';

        const thead = el('thead');
        const headRow = el('tr');
        for (const h of ['十进制', '十六进制', '八进制', '字符', '名称/描述']) {
          headRow.append(el('th', undefined, h));
        }
        thead.append(headRow);

        const tbody = el('tbody');
        for (let i = 0; i <= 127; i++) {
          const tr = el('tr');
          const hex = i.toString(16).toUpperCase().padStart(2, '0');
          const oct = i.toString(8);
          const ctrl = CONTROL_NAMES[i];
          const ch = ctrl ? '·' : String.fromCharCode(i);
          const desc = ctrl ?? (i === 32 ? 'SP (空格)' : `可打印字符`);
          tr.append(
            el('td', undefined, String(i)),
            el('td', undefined, '0x' + hex),
            el('td', undefined, oct),
            el('td', undefined, ch),
            el('td', undefined, desc),
          );
          tbody.append(tr);
        }
        table.append(thead, tbody);

        const wrap = el('div', 'ftb-codeblock');
        wrap.style.maxHeight = '520px';
        wrap.style.overflow = 'auto';
        wrap.append(table);
        layout.inputArea.append(el('div', 'ftb-desc', 'ASCII 标准 0–127 字符对照表（含控制字符）：'));
        layout.outputArea.append(wrap);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
