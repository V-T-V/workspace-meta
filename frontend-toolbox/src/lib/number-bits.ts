// =============================================================================
// 数字进制 / 位运算 / IEEE 754 可视化纯函数库
// 全部无 DOM 依赖，可在 node --test 下直接测试。
// =============================================================================

/** 把一个 32 位无符号整数转为 32 位二进制字符串（含前导零，每 4 位空格分组）。 */
export function toBinary32(n: number): string {
  const u = n >>> 0;
  const bits = u.toString(2).padStart(32, '0');
  // 每 8 位分组
  return bits.match(/.{1,8}/g)!.join(' ');
}

/** 把一个字节（0-255）转为 8 位二进制。 */
export function toBinary8(n: number): string {
  return (n & 0xff).toString(2).padStart(8, '0');
}

/** 各进制字符串表示。 */
export function radixStrings(n: number): {
  dec: string;
  bin: string;
  oct: string;
  hex: string;
} {
  const u = n >>> 0;
  return {
    dec: n.toString(),
    bin: u.toString(2),
    oct: u.toString(8),
    hex: u.toString(16).toUpperCase(),
  };
}

/** 补码表示（针对负数）。返回 32 位补码的二进制。 */
export function twosComplement(n: number): string {
  return toBinary32(n);
}

// ---------- IEEE 754 浮点 ----------

export interface IEEE754 {
  sign: number; // 0 或 1
  exponent: number; // 实际指数值（含偏移）
  exponentBits: string;
  mantissaBits: string;
  fullBits: string; // 完整 64 位
  isZero: boolean;
  isInfinity: boolean;
  isNaN: boolean;
  isDenormal: boolean;
  description: string;
}

/** 把数字解析为 IEEE 754 双精度（64 位）内存布局。 */
export function parseIEEE754(n: number): IEEE754 {
  const buf = new ArrayBuffer(8);
  new Float64Array(buf)[0] = n;
  const bits = new BigUint64Array(buf)[0]!;
  const bitsStr = bits.toString(2).padStart(64, '0');

  const sign = Number(bits >> 63n);
  const exponent = Number((bits >> 52n) & 0x7ffn);
  const mantissa = bits & 0xfffffffffffffn;

  const expBits = bitsStr.slice(1, 12);
  const mantissaBits = bitsStr.slice(12);

  const isZero = exponent === 0 && mantissa === 0n;
  const isInfinity = exponent === 0x7ff && mantissa === 0n;
  const isNaN = exponent === 0x7ff && mantissa !== 0n;
  const isDenormal = exponent === 0 && mantissa !== 0n;

  let description: string;
  if (isNaN) description = 'NaN（非数字）';
  else if (isInfinity) description = `${sign === 0 ? '+' : '-'}Infinity（无穷）`;
  else if (isZero) description = `${sign === 0 ? '+' : '-'}零`;
  else if (isDenormal) description = '非规格化数（极小）';
  else {
    const realExp = exponent - 1023;
    description = `规格化数：(-1)^${sign} × 1.${mantissaBits.slice(0, 8)}… × 2^${realExp}`;
  }

  return {
    sign,
    exponent,
    exponentBits: expBits,
    mantissaBits,
    fullBits: bitsStr,
    isZero,
    isInfinity,
    isNaN,
    isDenormal,
    description,
  };
}

/** 把 64 位二进制串分组格式化（1-11-52）。 */
export function formatIEEE754Bits(bits: string): string {
  return `${bits[0]} ${bits.slice(1, 12)} ${bits.slice(12)}`;
}

// ---------- 位运算 ----------

export interface BitOpResult {
  expression: string;
  result: number;
  resultBinary: string;
}

/** 对两个数执行所有位运算并返回结果。 */
export function bitOperations(a: number, b: number): BitOpResult[] {
  return [
    { expression: `a & b`, result: a & b, resultBinary: toBinary32(a & b) },
    { expression: `a | b`, result: a | b, resultBinary: toBinary32(a | b) },
    { expression: `a ^ b`, result: a ^ b, resultBinary: toBinary32(a ^ b) },
    { expression: `~a`, result: ~a, resultBinary: toBinary32(~a) },
    { expression: `~b`, result: ~b, resultBinary: toBinary32(~b) },
    { expression: `a << 1`, result: a << 1, resultBinary: toBinary32(a << 1) },
    { expression: `a >> 1`, result: a >> 1, resultBinary: toBinary32(a >> 1) },
    { expression: `a >>> 1`, result: a >>> 1, resultBinary: toBinary32(a >>> 1) },
  ];
}

/** 移位运算结果（支持任意移位量）。 */
export function shiftOperations(a: number, bits: number): BitOpResult[] {
  const b = Math.max(0, Math.min(31, bits | 0));
  return [
    { expression: `a << ${b}`, result: a << b, resultBinary: toBinary32(a << b) },
    { expression: `a >> ${b}`, result: a >> b, resultBinary: toBinary32(a >> b) },
    { expression: `a >>> ${b}`, result: a >>> b, resultBinary: toBinary32(a >>> b) },
  ];
}

/** 把布尔值数组转为数字（用于位掩码可视化）。 */
export function bitsToNumber(bits: boolean[]): number {
  let n = 0;
  for (let i = 0; i < bits.length; i++) {
    if (bits[i]) n |= 1 << i;
  }
  return n >>> 0;
}

/** 把数字转为布尔位数组（低位在前，指定位数）。 */
export function numberToBits(n: number, width = 32): boolean[] {
  const u = n >>> 0;
  const bits: boolean[] = [];
  for (let i = 0; i < width; i++) {
    bits.push(((u >> i) & 1) === 1);
  }
  return bits;
}
