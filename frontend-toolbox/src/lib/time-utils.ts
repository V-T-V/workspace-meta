// =============================================================================
// 时间处理纯函数库 —— 时间戳 / 时区 / 格式化 / 相对时间
// =============================================================================

export interface TimestampInfo {
  unixSeconds: number;
  unixMillis: number;
  iso8601: string;
  rfc2822: string;
  local: string;
  utc: string;
  date: Date;
  year: number;
  month: number;
  day: number;
  hours: number;
  minutes: number;
  seconds: number;
  weekday: string;
}

/** 从秒/毫秒时间戳构造信息（自动判断数量级）。 */
export function infoFromTimestamp(ts: number): TimestampInfo {
  // 10 位 → 秒，13 位 → 毫秒；统一成毫秒
  const millis = ts.toString().length <= 10 ? ts * 1000 : ts;
  const date = new Date(millis);
  return describe(date, millis);
}

/** 从日期字符串/Date 构造信息。 */
export function infoFromDate(input: Date | string): TimestampInfo {
  const date = input instanceof Date ? input : new Date(input);
  return describe(date, date.getTime());
}

function describe(date: Date, millis: number): TimestampInfo {
  const pad = (n: number, w = 2) => n.toString().padStart(w, '0');
  const local = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
  const utc = `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(
    date.getUTCDate(),
  )} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`;
  const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  return {
    unixSeconds: Math.floor(millis / 1000),
    unixMillis: millis,
    iso8601: date.toISOString(),
    rfc2822: date.toString(),
    local,
    utc,
    date,
    year: date.getFullYear(),
    month: date.getMonth() + 1,
    day: date.getDate(),
    hours: date.getHours(),
    minutes: date.getMinutes(),
    seconds: date.getSeconds(),
    weekday: weekdays[date.getDay()] ?? '',
  };
}

/** 当前时间戳。 */
export function now(): TimestampInfo {
  return infoFromTimestamp(Date.now());
}

/** 把日期字符串解析为时间戳（毫秒）。解析失败抛错。 */
export function parseToTimestamp(input: string): number {
  const ms = Date.parse(input);
  if (Number.isNaN(ms)) throw new Error(`无法解析为日期：${input}`);
  return ms;
}

/** 相对时间（如「3 分钟前」「2 小时后」）。 */
export function relativeTime(from: Date, to: Date = new Date()): string {
  const diff = to.getTime() - from.getTime();
  const abs = Math.abs(diff);
  const sign = diff >= 0 ? '前' : '后';
  const units: [number, string][] = [
    [60 * 1000, '秒'],
    [60 * 60 * 1000, '分钟'],
    [24 * 60 * 60 * 1000, '小时'],
    [30 * 24 * 60 * 60 * 1000, '天'],
    [365 * 24 * 60 * 60 * 1000, '月'],
    [Number.MAX_SAFE_INTEGER, '年'],
  ];
  let prev = 1;
  for (const [threshold, label] of units) {
    if (abs < threshold) {
      const v = Math.floor(abs / prev);
      return `${v} ${label}${sign}`;
    }
    prev = threshold;
  }
  return '刚刚';
}

/** 简单格式化：YYYY-MM-DD HH:mm:ss。 */
export function format(date: Date, fmt = 'YYYY-MM-DD HH:mm:ss'): string {
  const pad = (n: number, w = 2) => n.toString().padStart(w, '0');
  return fmt
    .replace('YYYY', date.getFullYear().toString())
    .replace('MM', pad(date.getMonth() + 1))
    .replace('DD', pad(date.getDate()))
    .replace('HH', pad(date.getHours()))
    .replace('mm', pad(date.getMinutes()))
    .replace('ss', pad(date.getSeconds()));
}

/** 计算两个日期差，返回 {days, hours, minutes, seconds}。 */
export function diffBetween(a: Date, b: Date): {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
  totalMs: number;
} {
  const totalMs = Math.abs(a.getTime() - b.getTime());
  const seconds = Math.floor(totalMs / 1000) % 60;
  const minutes = Math.floor(totalMs / (1000 * 60)) % 60;
  const hours = Math.floor(totalMs / (1000 * 60 * 60)) % 24;
  const days = Math.floor(totalMs / (1000 * 60 * 60 * 24));
  return { days, hours, minutes, seconds, totalMs };
}
