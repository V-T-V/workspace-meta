import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, input, select, field, el, kvRow } from '../../../ui/components.ts';
import { hmac } from '../../../lib/hash.ts';

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let msgInput: HTMLTextAreaElement;
    let keyInput: HTMLInputElement;
    let algoSel: HTMLSelectElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        msgInput = textarea('消息 …', 6);
        msgInput.value = 'POST /api/order\nbody={"amount":100}';
        keyInput = input('密钥 …', 'my-secret-key');
        algoSel = select(
          [
            ['SHA-256', 'SHA-256'],
            ['SHA-1', 'SHA-1'],
            ['SHA-512', 'SHA-512'],
          ],
          'SHA-256',
        );

        const out = el('div');
        const update = async (): Promise<void> => {
          out.replaceChildren();
          const sig = await hmac(
            algoSel.value as 'SHA-256' | 'SHA-1' | 'SHA-512',
            msgInput.value,
            keyInput.value,
          );
          out.append(
            kvRow(`HMAC-${algoSel.value}`, sig),
            kvRow('Base64', btoa(sig)),
          );
        };
        let timer = 0;
        const schedule = (): void => {
          clearTimeout(timer);
          timer = window.setTimeout(() => void update(), 150);
        };
        msgInput.addEventListener('input', schedule);
        keyInput.addEventListener('input', schedule);
        algoSel.addEventListener('change', schedule);

        layout.inputArea.append(
          field('算法', algoSel),
          field('密钥', keyInput),
          el('div', 'ftb-desc', '消息：'),
          msgInput,
        );
        layout.outputArea.append(out);
        ctx.container.append(layout.container);
        void update();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
