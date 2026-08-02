import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { textarea, button, el, kvRow, errorBox } from '../../../ui/components.ts';
import { createCodeBlock } from '../../../ui/code-block.ts';

/** Base64URL 解码为字符串。 */
function b64urlDecode(s: string): string {
  const pad = s.replace(/-/g, '+').replace(/_/g, '/');
  const padded = pad + '='.repeat((4 - (pad.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

const tool: Tool = {
  meta,
  create(): ToolInstance {
    let input: HTMLTextAreaElement;
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);
        input = textarea('粘贴 JWT …', 6);
        input.value =
          'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IuWAh+eOh+WQjeensCIsImlhdCI6MTcwMDAwMDAwMCwiZXhwIjoxOTAwMDAwMDAwfQ.signaturepart';

        const parse = (): void => {
          layout.outputArea.replaceChildren();
          const token = input.value.trim();
          if (!token) return;
          const parts = token.split('.');
          if (parts.length < 2) {
            layout.outputArea.append(errorBox('JWT 至少需要 2 段（header.payload）'));
            return;
          }
          try {
            const header = JSON.parse(b64urlDecode(parts[0]!));
            const payload = JSON.parse(b64urlDecode(parts[1]!));

            const headerBlock = createCodeBlock({ lang: 'Header', copyable: true });
            headerBlock.setText(JSON.stringify(header, null, 2));
            const payloadBlock = createCodeBlock({ lang: 'Payload', copyable: true });
            payloadBlock.setText(JSON.stringify(payload, null, 2));

            layout.outputArea.append(
              el('div', 'ftb-desc', 'Header：'),
              headerBlock.container,
              el('div', 'ftb-desc', 'Payload：'),
              payloadBlock.container,
            );

            // 关键声明解读
            const claims = el('div');
            claims.append(el('div', 'ftb-desc', '关键声明：'));
            const now = Math.floor(Date.now() / 1000);
            const iat = payload.iat as number | undefined;
            const exp = payload.exp as number | undefined;
            if (iat) claims.append(kvRow('iat (签发)', `${iat} · ${new Date(iat * 1000).toLocaleString()}`));
            if (exp) {
              const expired = exp < now;
              claims.append(
                kvRow(
                  'exp (过期)',
                  `${exp} · ${new Date(exp * 1000).toLocaleString()} ${expired ? '· ❌ 已过期' : '· ✅ 有效'}`,
                ),
              );
            }
            if (payload.sub) claims.append(kvRow('sub', String(payload.sub)));
            if (payload.iss) claims.append(kvRow('iss', String(payload.iss)));
            if (payload.aud) claims.append(kvRow('aud', String(payload.aud)));

            if (parts[2]) {
              claims.append(kvRow('signature', parts[2]));
              claims.append(
                el(
                  'div',
                  'ftb-desc',
                  '⚠ 注意：本工具仅解码展示，不验证签名真伪。',
                ),
              );
            }
            layout.outputArea.append(claims);
          } catch (e) {
            layout.outputArea.append(errorBox('解析失败：' + (e as Error).message));
          }
        };

        const btn = button('解析', parse);
        input.addEventListener('input', parse);
        layout.inputArea.append(btn, input);
        ctx.container.append(layout.container);
        parse();
      },
    } satisfies ToolInstance;
  },
};

export default tool;
