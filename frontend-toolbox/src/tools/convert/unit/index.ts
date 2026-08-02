import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, select, el, kvRow } from '../../../ui/components.ts';
import { UNITS, convert, formatNumber, type Category } from '../../../lib/unit-utils.ts';

const CATS: Array<[Category, string]> = [
  ['length', '长度'],
  ['weight', '重量'],
  ['data', '数据存储'],
  ['area', '面积'],
  ['time', '时间'],
  ['temperature', '温度'],
];

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const catSel = select(CATS as Array<[string, string]>, 'length') as unknown as HTMLSelectElement;
        const valInp = input('', '1');
        valInp.type = 'number';
        valInp.step = 'any';

        const rebuildUnits = (): void => {
          const cat = catSel.value as Category;
          const units = UNITS[cat];
          fromSel.replaceChildren();
          toSel.replaceChildren();
          for (const u of units) {
            const o1 = document.createElement('option');
            o1.value = u.id;
            o1.textContent = `${u.name} (${u.id})`;
            const o2 = o1.cloneNode(true) as HTMLOptionElement;
            fromSel.append(o1);
            toSel.append(o2);
          }
          fromSel.value = units[0]!.id;
          toSel.value = units[1]?.id ?? units[0]!.id;
          update();
        };

        const fromSel = document.createElement('select');
        fromSel.className = 'ftb-select';
        const toSel = document.createElement('select');
        toSel.className = 'ftb-select';

        const out = el('div');

        const update = (): void => {
          out.replaceChildren();
          const cat = catSel.value as Category;
          const v = Number(valInp.value);
          if (Number.isNaN(v)) return;
          // 主换算
          try {
            const result = convert(cat, v, fromSel.value, toSel.value);
            const toUnit = UNITS[cat].find((u) => u.id === toSel.value);
            out.append(kvRow('结果', `${formatNumber(result)} ${toUnit?.name ?? ''}`));
          } catch (e) {
            out.append(kvRow('错误', (e as Error).message));
          }
          // 全部对照
          out.append(el('div', 'ftb-desc', '全部对照：'));
          for (const u of UNITS[cat]) {
            try {
              out.append(kvRow(u.name, formatNumber(convert(cat, v, fromSel.value, u.id)) + ' ' + u.id));
            } catch {
              /* skip */
            }
          }
        };

        catSel.addEventListener('change', rebuildUnits);
        valInp.addEventListener('input', update);
        fromSel.addEventListener('change', update);
        toSel.addEventListener('change', update);

        const tb = el('div', 'ftb-toolbar');
        tb.append(catSel, valInp, fromSel, el('span', undefined, '→'), toSel);
        layout.inputArea.append(tb);
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        rebuildUnits();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
