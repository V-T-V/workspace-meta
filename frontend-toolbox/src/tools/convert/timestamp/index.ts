import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, el, kvRow, errorBox } from '../../../ui/components.ts';
import { infoFromTimestamp, infoFromDate, now, relativeTime } from '../../../lib/time-utils.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let tsInput: HTMLInputElement;
    let dateInput: HTMLInputElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        tsInput = input('Unix 时间戳（秒或毫秒）', String(Math.floor(Date.now() / 1000)));
        dateInput = input('日期字符串（如 2024-01-01 12:00:00）', '');
        dateInput.type = 'text';

        const out = el('div');
        const err = el('div');

        const renderInfo = (info: ReturnType<typeof infoFromTimestamp>): void => {
          err.replaceChildren();
          out.replaceChildren();
          out.append(
            kvRow('Unix (秒)', String(info.unixSeconds)),
            kvRow('Unix (毫秒)', String(info.unixMillis)),
            kvRow('ISO 8601', info.iso8601),
            kvRow('本地时间', info.local),
            kvRow('UTC 时间', info.utc),
            kvRow('RFC 2822', info.rfc2822),
            kvRow('年/月/日', `${info.year}-${String(info.month).padStart(2, '0')}-${String(info.day).padStart(2, '0')}`),
            kvRow('时:分:秒', `${String(info.hours).padStart(2, '0')}:${String(info.minutes).padStart(2, '0')}:${String(info.seconds).padStart(2, '0')}`),
            kvRow('星期', info.weekday),
            kvRow('距今', relativeTime(info.date)),
          );
        };

        const fromTs = (): void => {
          const v = tsInput.value.trim();
          if (!v) return;
          const n = Number(v);
          if (Number.isNaN(n)) {
            err.replaceChildren();
            err.append(errorBox('请输入数字'));
            out.replaceChildren();
            return;
          }
          try {
            renderInfo(infoFromTimestamp(n));
          } catch (e) {
            err.replaceChildren();
            err.append(errorBox((e as Error).message));
          }
        };

        const fromDate = (): void => {
          const v = dateInput.value.trim();
          if (!v) return;
          try {
            renderInfo(infoFromDate(v));
            tsInput.value = String(infoFromDate(v).unixSeconds);
          } catch (e) {
            err.replaceChildren();
            err.append(errorBox((e as Error).message));
          }
        };

        tsInput.addEventListener('input', fromTs);
        dateInput.addEventListener('input', fromDate);

        const nowBtn = button('当前时间', () => {
          const info = now();
          tsInput.value = String(info.unixSeconds);
          renderInfo(info);
        });

        layout.inputArea.append(nowBtn, tsInput, el('div', 'ftb-desc', '或输入日期：'), dateInput);
        layout.outputArea.append(err, out);
        ctx.container.append(layout.container);
        fromTs();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
