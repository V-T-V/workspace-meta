import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { button, select, field, input, copyText } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';
import { toastSuccess } from '../../../ui/toast.ts';
import { uuidV4, nanoId, snowflake, ulid, randomToken } from '../../../lib/id-utils.ts';

type IdKind = 'uuid' | 'nanoid' | 'snowflake' | 'ulid' | 'token';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        const kindSel = select(
          [
            ['uuid', 'UUID v4'],
            ['nanoid', 'NanoID (21 位)'],
            ['snowflake', '雪花 ID'],
            ['ulid', 'ULID'],
            ['token', '随机令牌 (32 字节 hex)'],
          ] as Array<[string, string]>,
          'uuid',
        );
        const countInp = input('', '10');
        countInp.type = 'number';
        countInp.min = '1';
        countInp.max = '1000';
        countInp.style.width = '90px';

        const codeBlock = createCodeBlock({ lang: '结果', copyable: true });

        const gen = (): void => {
          const kind = kindSel.value as IdKind;
          const n = Math.min(1000, Math.max(1, Number(countInp.value) || 1));
          const lines: string[] = [];
          for (let i = 0; i < n; i++) {
            switch (kind) {
              case 'uuid':
                lines.push(uuidV4());
                break;
              case 'nanoid':
                lines.push(nanoId(21));
                break;
              case 'snowflake':
                lines.push(snowflake());
                break;
              case 'ulid':
                lines.push(ulid());
                break;
              case 'token':
                lines.push(randomToken(32));
                break;
            }
          }
          codeBlock.setText(lines.join('\n'));
        };

        const genBtn = button('生成', gen);
        const copyBtn = button('复制全部', async () => {
          const ok = await copyText(codeBlock.container.querySelector('code')?.textContent ?? '');
          if (ok) toastSuccess('已复制全部');
        }, 'ghost');

        kindSel.addEventListener('change', gen);
        countInp.addEventListener('input', gen);

        layout.inputArea.append(field('类型', kindSel), field('数量', countInp), genBtn, copyBtn);
        layout.outputArea.append(codeBlock.container);
        ctx.container.append(layout.container);
        gen();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
