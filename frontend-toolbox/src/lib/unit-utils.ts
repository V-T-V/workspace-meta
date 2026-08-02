// =============================================================================
// 单位换算纯函数库 —— 长度 / 重量 / 数据 / 面积 / 时间 / 温度
// 每类以基准单位为枢纽：toBase / fromBase。
// =============================================================================

export type Category = 'length' | 'weight' | 'data' | 'area' | 'time' | 'temperature';

export interface UnitDef {
  id: string;
  name: string;
  /** 换算到基准单位的因子（温度特殊处理，见 toBase/fromBase）。 */
  factor: number;
}

export const UNITS: Record<Category, UnitDef[]> = {
  length: [
    { id: 'mm', name: '毫米', factor: 0.001 },
    { id: 'cm', name: '厘米', factor: 0.01 },
    { id: 'm', name: '米', factor: 1 },
    { id: 'km', name: '千米', factor: 1000 },
    { id: 'in', name: '英寸', factor: 0.0254 },
    { id: 'ft', name: '英尺', factor: 0.3048 },
    { id: 'mi', name: '英里', factor: 1609.344 },
  ],
  weight: [
    { id: 'mg', name: '毫克', factor: 0.001 },
    { id: 'g', name: '克', factor: 1 },
    { id: 'kg', name: '千克', factor: 1000 },
    { id: 't', name: '吨', factor: 1_000_000 },
    { id: 'lb', name: '磅', factor: 453.592 },
    { id: 'oz', name: '盎司', factor: 28.3495 },
  ],
  data: [
    { id: 'B', name: '字节', factor: 1 },
    { id: 'KB', name: '千字节', factor: 1024 },
    { id: 'MB', name: '兆字节', factor: 1024 ** 2 },
    { id: 'GB', name: '吉字节', factor: 1024 ** 3 },
    { id: 'TB', name: '太字节', factor: 1024 ** 4 },
    { id: 'bit', name: '比特', factor: 0.125 },
  ],
  area: [
    { id: 'mm2', name: '平方毫米', factor: 0.000001 },
    { id: 'cm2', name: '平方厘米', factor: 0.0001 },
    { id: 'm2', name: '平方米', factor: 1 },
    { id: 'km2', name: '平方千米', factor: 1_000_000 },
    { id: 'ha', name: '公顷', factor: 10_000 },
    { id: 'acre', name: '英亩', factor: 4046.86 },
    { id: 'mu', name: '亩', factor: 666.667 },
  ],
  time: [
    { id: 'ms', name: '毫秒', factor: 0.001 },
    { id: 's', name: '秒', factor: 1 },
    { id: 'min', name: '分钟', factor: 60 },
    { id: 'h', name: '小时', factor: 3600 },
    { id: 'd', name: '天', factor: 86400 },
    { id: 'week', name: '周', factor: 604800 },
  ],
  // 温度用独立函数，factor 不使用
  temperature: [
    { id: 'C', name: '摄氏度', factor: 1 },
    { id: 'F', name: '华氏度', factor: 1 },
    { id: 'K', name: '开尔文', factor: 1 },
  ],
};

/** 换算。 */
export function convert(
  category: Category,
  value: number,
  fromId: string,
  toId: string,
): number {
  if (category === 'temperature') {
    return convertTemperature(value, fromId as 'C' | 'F' | 'K', toId as 'C' | 'F' | 'K');
  }
  const units = UNITS[category];
  const from = units.find((u) => u.id === fromId);
  const to = units.find((u) => u.id === toId);
  if (!from || !to) throw new Error(`未知单位：${fromId} 或 ${toId}`);
  const base = value * from.factor;
  return base / to.factor;
}

function convertTemperature(value: number, from: 'C' | 'F' | 'K', to: 'C' | 'F' | 'K'): number {
  // 先转摄氏
  let c: number;
  switch (from) {
    case 'C':
      c = value;
      break;
    case 'F':
      c = (value - 32) * (5 / 9);
      break;
    case 'K':
      c = value - 273.15;
      break;
  }
  switch (to) {
    case 'C':
      return c;
    case 'F':
      return c * (9 / 5) + 32;
    case 'K':
      return c + 273.15;
  }
}

/** 格式化为友好数字（去尾零）。 */
export function formatNumber(n: number, max = 6): string {
  if (!Number.isFinite(n)) return String(n);
  return parseFloat(n.toFixed(max)).toString();
}
