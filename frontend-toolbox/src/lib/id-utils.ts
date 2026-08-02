// =============================================================================
// ID 生成纯函数库 —— UUID v4 / NanoID 风格 / 雪花算法（简化） / ULID 风格
// =============================================================================

function getCrypto(): Crypto {
  const g = globalThis as { crypto?: Crypto };
  if (g.crypto) return g.crypto;
  throw new Error('当前环境无 Web Crypto API');
}

/** UUID v4（基于随机数）。 */
export function uuidV4(): string {
  const bytes = getCrypto().getRandomValues(new Uint8Array(16));
  // 设置 version (4) 和 variant (10xx)
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const h = (b: number) => b.toString(16).padStart(2, '0');
  return `${h(bytes[0]!)}${h(bytes[1]!)}${h(bytes[2]!)}${h(bytes[3]!)}-${h(bytes[4]!)}${h(
    bytes[5]!,
  )}-${h(bytes[6]!)}${h(bytes[7]!)}-${h(bytes[8]!)}${h(bytes[9]!)}-${h(bytes[10]!)}${h(
    bytes[11]!,
  )}${h(bytes[12]!)}${h(bytes[13]!)}${h(bytes[14]!)}${h(bytes[15]!)}`;
}

/** UUID v4 去掉横线（32 位）。 */
export function uuidCompact(): string {
  return uuidV4().replace(/-/g, '');
}

/** 生成多个 UUID。 */
export function uuids(count: number): string[] {
  return Array.from({ length: count }, () => uuidV4());
}

const NANOID_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';

/** NanoID 风格的短 ID。 */
export function nanoId(size = 21): string {
  const bytes = getCrypto().getRandomValues(new Uint8Array(size));
  let out = '';
  for (let i = 0; i < size; i++) out += NANOID_ALPHABET[bytes[i]! & 63];
  return out;
}

// ---------- 简化版雪花算法 ----------
// 结构：41bit 时间戳 + 10bit 机器 + 12bit 序列（与 Twitter Snowflake 对齐，单机简化）

const EPOCH = 1704067200000; // 2024-01-01 UTC

let snowflakeMachine = 1;
let snowflakeSeq = 0;
let snowflakeLast = -1;

export function setSnowflakeMachine(id: number): void {
  snowflakeMachine = id & 0x3ff;
}

/** 生成一个雪花 ID（BigInt 字符串）。 */
export function snowflake(): string {
  const now = Date.now() - EPOCH;
  if (now === snowflakeLast) {
    snowflakeSeq = (snowflakeSeq + 1) & 0xfff;
    if (snowflakeSeq === 0) {
      // 序列耗尽，等到下一毫秒
      while (Date.now() - EPOCH <= snowflakeLast) {
        /* spin */
      }
    }
  } else {
    snowflakeSeq = 0;
  }
  snowflakeLast = now;
  const id =
    (BigInt(now) << 22n) | (BigInt(snowflakeMachine) << 12n) | BigInt(snowflakeSeq);
  return id.toString();
}

// ---------- ULID 风格（Crockford Base32，时间排序）----------

const ULID_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

export function ulid(timestamp: number = Date.now()): string {
  const t = BigInt(timestamp);
  let timePart = '';
  for (let i = 9; i >= 0; i--) {
    timePart = ULID_ALPHABET[Number((t >> BigInt(i * 5)) & 31n)]! + timePart;
  }
  const random = getCrypto().getRandomValues(new Uint8Array(16));
  let randPart = '';
  for (let i = 0; i < 16; i++) randPart += ULID_ALPHABET[random[i]! & 31];
  return timePart + randPart;
}

// ---------- 简短随机令牌 ----------

export function randomToken(bytes = 32): string {
  const arr = getCrypto().getRandomValues(new Uint8Array(bytes));
  return Array.from(arr)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}
