// =============================================================================
// 编解码纯函数库 —— Base64 / URL / HEX / HTML 实体 / Unicode 转义
// 全部无 DOM 依赖，可在 node --test 下直接测试。
// =============================================================================

// ---------- Base64（支持 UTF-8，处理浏览器/Node 兼容）----------

/** UTF-8 字符串 → Base64。 */
export function encodeBase64(input: string): string {
  const bytes = new TextEncoder().encode(input);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

/** Base64 → UTF-8 字符串。非法输入抛错。 */
export function decodeBase64(input: string): string {
  const binary = atob(input.trim());
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

// ---------- URL 编解码 ----------

export function encodeURL(input: string): string {
  return encodeURIComponent(input);
}

export function decodeURL(input: string): string {
  return decodeURIComponent(input.replace(/\+/g, ' '));
}

/** 完整 URL 编码（含 / ? : @ & = # 等保留字符）。 */
export function encodeURLComponentAll(input: string): string {
  return Array.from(input)
    .map((ch) => '%' + ch.charCodeAt(0).toString(16).toUpperCase().padStart(2, '0'))
    .join('');
}

// ---------- HEX 编解码 ----------

/** UTF-8 字符串 → HEX（大写，每字节两位）。 */
export function encodeHex(input: string): string {
  const bytes = new TextEncoder().encode(input);
  return Array.from(bytes)
    .map((b) => b.toString(16).toUpperCase().padStart(2, '0'))
    .join('');
}

/** HEX → UTF-8 字符串。允许空格/0x 前缀。 */
export function decodeHex(input: string): string {
  const clean = input.replace(/0x/gi, '').replace(/[\s_-]/g, '');
  if (!/^[0-9a-fA-F]*$/.test(clean) || clean.length % 2 !== 0) {
    throw new Error('HEX 格式无效：需为偶数位十六进制字符');
  }
  const bytes = new Uint8Array(clean.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16);
  }
  return new TextDecoder().decode(bytes);
}

// ---------- HTML 实体编解码 ----------

const HTML_NAMED: Readonly<Record<string, string>> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
  '`': '&#96;',
};

export function encodeHTML(input: string): string {
  return input.replace(/[&<>"'`]/g, (ch) => HTML_NAMED[ch] ?? ch);
}

export function decodeHTML(input: string): string {
  const named: Readonly<Record<string, string>> = {
    amp: '&',
    lt: '<',
    gt: '>',
    quot: '"',
    apos: "'",
    nbsp: '\u00a0',
    copy: '\u00a9',
    reg: '\u00ae',
    trade: '\u2122',
    hellip: '\u2026',
    mdash: '\u2014',
    ndash: '\u2013',
    lsquo: '\u2018',
    rsquo: '\u2019',
    ldquo: '\u201c',
    rdquo: '\u201d',
  };
  return input
    .replace(/&(#x?[0-9a-fA-F]+|[a-zA-Z]+);/g, (full, body: string) => {
      if (body.startsWith('#')) {
        const isHex = body[1] === 'x' || body[1] === 'X';
        const code = parseInt(body.slice(isHex ? 2 : 1), isHex ? 16 : 10);
        if (Number.isNaN(code)) return full;
        return String.fromCodePoint(code);
      }
      return named[body] ?? full;
    });
}

// ---------- Unicode 转义（\uXXXX / \u{XXXXXX}） ----------

export function encodeUnicodeEscape(input: string): string {
  let out = '';
  for (const ch of input) {
    const cp = ch.codePointAt(0)!;
    if (cp <= 0xffff) {
      out += '\\u' + cp.toString(16).toUpperCase().padStart(4, '0');
    } else {
      out += '\\u{' + cp.toString(16).toUpperCase() + '}';
    }
  }
  return out;
}

export function decodeUnicodeEscape(input: string): string {
  return input
    .replace(/\\u\{([0-9a-fA-F]+)\}/g, (_, hex: string) =>
      String.fromCodePoint(parseInt(hex, 16)),
    )
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, hex: string) =>
      String.fromCharCode(parseInt(hex, 16)),
    );
}

// ---------- 二进制 / 八进制文本 ----------

/** UTF-8 字符串 → 二进制文本（每字符 8 位空格分隔）。 */
export function encodeBinary(input: string): string {
  return Array.from(new TextEncoder().encode(input))
    .map((b) => b.toString(2).padStart(8, '0'))
    .join(' ');
}

/** 二进制文本 → UTF-8 字符串。 */
export function decodeBinary(input: string): string {
  const bits = input.trim().split(/[\s,]+/).filter(Boolean);
  const bytes = new Uint8Array(bits.length);
  for (let i = 0; i < bits.length; i++) {
    const n = parseInt(bits[i]!, 2);
    if (Number.isNaN(n) || n < 0 || n > 255) {
      throw new Error('二进制格式无效：每组应为 8 位 0/1');
    }
    bytes[i] = n;
  }
  return new TextDecoder().decode(bytes);
}
