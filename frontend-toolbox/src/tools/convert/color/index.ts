import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, el, kvRow, errorBox } from '../../../ui/components.ts';
import { describeAll, toHex, isLight, complement, hueWheel, adjustLightness } from '../../../lib/color-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let inp: HTMLInputElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        inp = input('输入颜色（#hex / rgb() / hsl()）', '#2563eb');

        const err = el('div');
        const preview = el('div', 'ftb-color-preview');
        const swatch = el('div', 'ftb-color-swatch');
        const previewText = el('div');
        preview.append(swatch, previewText);

        const kvs = el('div');
        const palette = el('div');
        const paletteBox = el('div', 'ftb-color-palette');
        palette.append(el('div', 'ftb-desc', '色相环 6 配色：'), paletteBox);

        const update = (): void => {
          err.replaceChildren();
          try {
            const c = describeAll(inp.value);
            swatch.style.background = c.hex;
            previewText.replaceChildren(
              kvRow('HEX', c.hex),
              kvRow('RGB', c.rgb),
              kvRow('HSL', c.hsl),
              kvRow('是否浅色', isLight(c.rgbObj) ? '是（建议深色前景）' : '否（建议浅色前景）'),
              kvRow('互补色', toHex(complement(c.rgbObj))),
              kvRow('变亮 10%', toHex(adjustLightness(c.rgbObj, 10))),
              kvRow('变暗 10%', toHex(adjustLightness(c.rgbObj, -10))),
            );
            kvs.replaceChildren(previewText);
            paletteBox.replaceChildren();
            for (const wheel of hueWheel(c.rgbObj, 6)) {
              const item = el('div', 'ftb-color-palette-item');
              item.style.background = toHex(wheel);
              item.title = toHex(wheel);
              item.addEventListener('click', () => {
                inp.value = toHex(wheel);
                update();
              });
              paletteBox.append(item);
            }
          } catch (e) {
            kvs.replaceChildren();
            paletteBox.replaceChildren();
            err.append(errorBox((e as Error).message));
          }
        };
        inp.addEventListener('input', update);
        layout.inputArea.append(inp);
        layout.outputArea.append(err, preview, kvs, palette);
        ctx.container.append(layout.container);
        update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
