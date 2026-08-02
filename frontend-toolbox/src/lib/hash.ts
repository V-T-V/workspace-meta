// =============================================================================
// Hash / 加密纯函数库 —— 基于 Web Crypto API（SubtleCrypto）
// 浏览器原生，免装 crypto-js。所有哈希返回小写 hex。
// Node 18+ 全局有 crypto.webcrypto，测试环境同样可用。
// =============================================================================

/** 字节序列 → 小写 hex 字符串。 */
export function bytesToHex(bytes: ArrayBuffer | Uint8Array): string {
  const view = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  return Array.from(view)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/** hex 字符串 → 字节序列。 */
export function hexToBytes(hex: string): Uint8Array {
  const clean = hex.replace(/[\s-]/g, '');
  const out = new Uint8Array(clean.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16);
  }
  return out;
}

/** 取全局 crypto（浏览器 crypto，Node 18+ webcrypto）。 */
function getCrypto(): Crypto {
  const g = globalThis as { crypto?: Crypto };
  if (g.crypto) return g.crypto;
  throw new Error('当前环境无 Web Crypto API');
}

/** 通用摘要：算法名 + 输入 → hex。支持 SHA-1/256/384/512。 */
export async function digest(
  algorithm: 'SHA-1' | 'SHA-256' | 'SHA-384' | 'SHA-512',
  input: string,
): Promise<string> {
  const data = new TextEncoder().encode(input);
  const buf = await getCrypto().subtle.digest(algorithm, data);
  return bytesToHex(buf);
}

export const sha1 = (input: string): Promise<string> => digest('SHA-1', input);
export const sha256 = (input: string): Promise<string> => digest('SHA-256', input);
export const sha384 = (input: string): Promise<string> => digest('SHA-384', input);
export const sha512 = (input: string): Promise<string> => digest('SHA-512', input);

// ---------- MD5（RFC 1321，纯 TS 实现）----------
// Web Crypto 不支持 MD5（已不安全）。这里实现标准版，仅用于校验/展示。
// 关键：所有字运算用无符号 32 位语义（>>> 0 归一化）。

export function md5(input: string): string {
  return md5Bytes(new TextEncoder().encode(input));
}

// 32 位无符号加法
const add32 = (a: number, b: number): number => (a + b) & 0xffffffff;

// 循环左移（无符号）
function rol(x: number, c: number): number {
  return ((x << c) | (x >>> (32 - c))) & 0xffffffff;
}

// 各轮非线性函数
function F(x: number, y: number, z: number): number {
  return (x & y) | (~x & z);
}
function G(x: number, y: number, z: number): number {
  return (x & z) | (y & ~z);
}
function H(x: number, y: number, z: number): number {
  return x ^ y ^ z;
}
function I(x: number, y: number, z: number): number {
  return y ^ (x | ~z);
}

function md5Bytes(msg: Uint8Array): string {
  const s = [
    7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9,
    14, 20, 5, 9, 14, 20, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 6, 10, 15,
    21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
  ];
  const K = [
    0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee, 0xf57c0faf, 0x4787c62a, 0xa8304613,
    0xfd469501, 0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be, 0x6b901122, 0xfd987193, 0xa679438e,
    0x49b40821, 0xf61e2562, 0xc040b340, 0x265e5a51, 0xe9b6c7aa, 0xd62f105d, 0x02441453, 0xd8a1e681,
    0xe7d3fbc8, 0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed, 0xa9e3e905, 0xfcefa3f8, 0x676f02d9,
    0x8d2a4c8a, 0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c, 0xa4beea44, 0x4bdecfa9, 0xf6bb4b60,
    0xbebfbc70, 0x289b7ec6, 0xeaa127fa, 0xd4ef3085, 0x04881d05, 0xd9d4d039, 0xe6db99e5, 0x1fa27cf8,
    0xc4ac5665, 0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039, 0x655b59c3, 0x8f0ccc92, 0xffeff47d,
    0x85845dd1, 0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1, 0xf7537e82, 0xbd3af235, 0x2ad7d2bb,
    0xeb86d391,
  ];

  // 预处理：填充至 length ≡ 56 (mod 64)，再追加 8 字节长度
  const origLen = msg.length;
  const bitLenHi = Math.floor((origLen * 8) / 0x100000000);
  const bitLenLo = (origLen * 8) & 0xffffffff;

  const withOneLen = origLen + 1;
  const padLen = (56 - (withOneLen % 64) + 64) % 64;
  const total = withOneLen + padLen + 8;
  const bytes = new Uint8Array(total);
  bytes.set(msg);
  bytes[origLen] = 0x80;
  // 末尾 8 字节 = 原始位长度（小端）
  const dv = new DataView(bytes.buffer);
  dv.setUint32(total - 8, bitLenLo >>> 0, true);
  dv.setUint32(total - 4, bitLenHi >>> 0, true);

  let a0 = 0x67452301;
  let b0 = 0xefcdab89;
  let c0 = 0x98badcfe;
  let d0 = 0x10325476;

  const M = new Int32Array(16);
  for (let off = 0; off < total; off += 64) {
    // 读入 16 个小端 32 位字
    for (let i = 0; i < 16; i++) {
      M[i] = dv.getInt32(off + i * 4, true);
    }
    let A = a0,
      B = b0,
      C = c0,
      D = d0;
    for (let i = 0; i < 64; i++) {
      let f: number;
      let g: number;
      if (i < 16) {
        f = F(B, C, D);
        g = i;
      } else if (i < 32) {
        f = G(B, C, D);
        g = (5 * i + 1) % 16;
      } else if (i < 48) {
        f = H(B, C, D);
        g = (3 * i + 5) % 16;
      } else {
        f = I(B, C, D);
        g = (7 * i) % 16;
      }
      // 注意：M[g]、K[i] 可能是负数（Int32），先 >>>0 转 unsigned 再加
      f = add32(add32(add32(f, A), (K[i]! >>> 0)), M[g]! >>> 0);
      A = D;
      D = C;
      C = B;
      B = add32(B, rol(f, s[i]!));
    }
    a0 = add32(a0, A);
    b0 = add32(b0, B);
    c0 = add32(c0, C);
    d0 = add32(d0, D);
  }

  const out = new Uint8Array(16);
  const ov = new DataView(out.buffer);
  ov.setUint32(0, a0 >>> 0, true);
  ov.setUint32(4, b0 >>> 0, true);
  ov.setUint32(8, c0 >>> 0, true);
  ov.setUint32(12, d0 >>> 0, true);
  return bytesToHex(out);
}

// ---------- HMAC ----------

/** HMAC，算法支持 SHA-1/256/384/512，返回 hex。 */
export async function hmac(
  algorithm: 'SHA-1' | 'SHA-256' | 'SHA-384' | 'SHA-512',
  message: string,
  key: string,
): Promise<string> {
  const crypto = getCrypto();
  const keyData = new TextEncoder().encode(key);
  const cryptoKey = await crypto.subtle.importKey(
    'raw',
    keyData,
    { name: 'HMAC', hash: algorithm },
    false,
    ['sign'],
  );
  const sig = await crypto.subtle.sign('HMAC', cryptoKey, new TextEncoder().encode(message));
  return bytesToHex(sig);
}

export const hmacSha1 = (msg: string, key: string) => hmac('SHA-1', msg, key);
export const hmacSha256 = (msg: string, key: string) => hmac('SHA-256', msg, key);
export const hmacSha512 = (msg: string, key: string) => hmac('SHA-512', msg, key);
