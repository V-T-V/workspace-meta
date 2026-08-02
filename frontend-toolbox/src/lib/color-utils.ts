// =============================================================================
// 颜色处理纯函数库 —— HEX / RGB / HSL 互转 + 明暗判断 + 调色
// =============================================================================

export interface RGB {
  r: number;
  g: number;
  b: number;
}

export interface HSL {
  h: number;
  s: number;
  l: number;
}

const clamp = (n: number, min: number, max: number): number => Math.min(max, Math.max(min, n));

// ---------- HEX ----------

export function parseHex(hex: string): RGB {
  const clean = hex.trim().replace(/^#/, '');
  let r: number, g: number, b: number;
  if (clean.length === 3) {
    r = parseInt(clean[0]! + clean[0]!, 16);
    g = parseInt(clean[1]! + clean[1]!, 16);
    b = parseInt(clean[2]! + clean[2]!, 16);
  } else if (clean.length === 6 || clean.length === 8) {
    r = parseInt(clean.slice(0, 2), 16);
    g = parseInt(clean.slice(2, 4), 16);
    b = parseInt(clean.slice(4, 6), 16);
  } else {
    throw new Error(`无效 HEX：${hex}`);
  }
  if ([r, g, b].some((n) => Number.isNaN(n))) throw new Error(`无效 HEX：${hex}`);
  return { r, g, b };
}

export function toHex({ r, g, b }: RGB, withAlpha = false, a = 1): string {
  const h = (n: number) => clamp(Math.round(n), 0, 255).toString(16).padStart(2, '0');
  const base = `#${h(r)}${h(g)}${h(b)}`;
  if (!withAlpha) return base;
  return base + h(a * 255);
}

// ---------- RGB ----------

export function parseRgb(input: string): RGB {
  const m = input.match(/rgba?\(\s*(\d+)\s*,?\s*(\d+)\s*,?\s*(\d+)/);
  if (!m) throw new Error(`无效 RGB：${input}`);
  return { r: +m[1]!, g: +m[2]!, b: +m[3]! };
}

export function toRgbString({ r, g, b }: RGB): string {
  return `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`;
}

// ---------- HSL ----------

export function rgbToHsl({ r, g, b }: RGB): HSL {
  const rn = r / 255,
    gn = g / 255,
    bn = b / 255;
  const max = Math.max(rn, gn, bn);
  const min = Math.min(rn, gn, bn);
  const l = (max + min) / 2;
  let h = 0;
  let s = 0;
  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    if (max === rn) h = ((gn - bn) / d + (gn < bn ? 6 : 0)) * 60;
    else if (max === gn) h = ((bn - rn) / d + 2) * 60;
    else h = ((rn - gn) / d + 4) * 60;
  }
  return { h: Math.round(h), s: Math.round(s * 100), l: Math.round(l * 100) };
}

export function hslToRgb({ h, s, l }: HSL): RGB {
  const hn = ((h % 360) + 360) % 360 / 360;
  const sn = clamp(s, 0, 100) / 100;
  const ln = clamp(l, 0, 100) / 100;
  if (sn === 0) {
    const v = Math.round(ln * 255);
    return { r: v, g: v, b: v };
  }
  const q = ln < 0.5 ? ln * (1 + sn) : ln + sn - ln * sn;
  const p = 2 * ln - q;
  const hue = (t: number): number => {
    const tt = (t + 1) % 1;
    if (tt < 1 / 6) return p + (q - p) * 6 * tt;
    if (tt < 1 / 2) return q;
    if (tt < 2 / 3) return p + (q - p) * (2 / 3 - tt) * 6;
    return p;
  };
  return {
    r: Math.round(hue(hn + 1 / 3) * 255),
    g: Math.round(hue(hn) * 255),
    b: Math.round(hue(hn - 1 / 3) * 255),
  };
}

// ---------- 综合解析 ----------

export function parseAny(input: string): RGB {
  const s = input.trim();
  if (s.startsWith('#')) return parseHex(s);
  if (s.startsWith('rgb')) return parseRgb(s);
  if (s.startsWith('hsl')) {
    const m = s.match(/hsla?\(\s*(\d+)\s*,?\s*(\d+)%?\s*,?\s*(\d+)%?/);
    if (!m) throw new Error(`无效 HSL：${input}`);
    return hslToRgb({ h: +m[1]!, s: +m[2]!, l: +m[3]! });
  }
  throw new Error(`无法识别的颜色格式：${input}`);
}

export interface ColorFull {
  hex: string;
  rgb: string;
  hsl: string;
  rgbObj: RGB;
  hslObj: HSL;
}

export function describeAll(input: string): ColorFull {
  const rgbObj = parseAny(input);
  const hslObj = rgbToHsl(rgbObj);
  return {
    hex: toHex(rgbObj),
    rgb: toRgbString(rgbObj),
    hsl: `hsl(${hslObj.h}, ${hslObj.s}%, ${hslObj.l}%)`,
    rgbObj,
    hslObj,
  };
}

/** 判断颜色是浅色还是深色（基于相对亮度，用于选前景色）。 */
export function isLight({ r, g, b }: RGB): boolean {
  // 简化的感知亮度公式
  const luma = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luma > 0.5;
}

/** 互补色。 */
export function complement(rgb: RGB): RGB {
  return { r: 255 - rgb.r, g: 255 - rgb.g, b: 255 - rgb.b };
}

/** 颜色变亮/变暗（amount: -100~100）。 */
export function adjustLightness(rgb: RGB, amount: number): RGB {
  const hsl = rgbToHsl(rgb);
  hsl.l = clamp(hsl.l + amount, 0, 100);
  return hslToRgb(hsl);
}

/** 生成 n 个均匀色相的配色（类似色相环取样）。 */
export function hueWheel(base: RGB, n: number): RGB[] {
  const hsl = rgbToHsl(base);
  const out: RGB[] = [];
  for (let i = 0; i < n; i++) {
    out.push(hslToRgb({ h: (hsl.h + (360 / n) * i) % 360, s: hsl.s, l: hsl.l }));
  }
  return out;
}
