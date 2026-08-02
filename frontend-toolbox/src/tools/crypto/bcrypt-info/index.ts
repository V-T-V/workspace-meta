import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, el, kvRow, errorBox } from '../../../ui/components.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let hashInput: HTMLInputElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        hashInput = input('粘贴 bcrypt 哈希 …', '$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy');

        const parse = (): void => {
          layout.outputArea.replaceChildren();
          const h = hashInput.value.trim();
          if (!h) return;
          // $2a$10$salt(22)hash(31)
          const m = h.match(/^\$(2[abxy])\$(\d{2})\$([A-Za-z0-9./]{22})([A-Za-z0-9./]{31})$/);
          if (!m) {
            layout.outputArea.append(errorBox('不是有效的 bcrypt 哈希格式（应为 $2X$NN$<22位盐><31位摘要>）'));
            return;
          }
          const [, version, cost, salt, digest] = m;
          layout.outputArea.append(
            kvRow('算法版本', `$${version}（2${version![1]}）`),
            kvRow('cost 因子', `${cost}（2^${cost} = ${Math.pow(2, Number(cost))} 轮）`),
            kvRow('盐值 (22位)', salt!),
            kvRow('摘要 (31位)', digest!),
            kvRow('总长度', `${h.length} 字符`),
            el(
              'div',
              'ftb-desc',
              '⚠ 本工具仅解析展示，不执行哈希计算或校验。生成 bcrypt 请在服务端使用专用库。',
            ),
          );
        };

        const btn = button('解析', parse);
        hashInput.addEventListener('input', parse);
        layout.inputArea.append(btn, hashInput);
        ctx.container.append(layout.container);
        parse();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
