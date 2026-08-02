// =============================================================================
// CSV 处理纯函数库 —— 解析 / 序列化 / 对齐美化 / JSON 互转
// 遵循 RFC 4180：逗号分隔，双引号转义，CRLF 换行。
// =============================================================================

/** 解析 CSV 文本为二维数组。 */
export function parseCSV(input: string, delimiter = ','): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let inQuotes = false;
  const len = input.length;

  for (let i = 0; i < len; i++) {
    const ch = input[i]!;
    if (inQuotes) {
      if (ch === '"') {
        if (input[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
    } else {
      if (ch === '"') {
        inQuotes = true;
      } else if (ch === delimiter) {
        row.push(field);
        field = '';
      } else if (ch === '\r') {
        // CR 或 CRLF
        row.push(field);
        field = '';
        rows.push(row);
        row = [];
        if (input[i + 1] === '\n') i++;
      } else if (ch === '\n') {
        row.push(field);
        field = '';
        rows.push(row);
        row = [];
      } else {
        field += ch;
      }
    }
  }
  // 最后一个字段
  if (field !== '' || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows;
}

/** 把一个字段序列化为 CSV 安全值（含逗号/引号/换行时加引号转义）。 */
function escapeField(value: string, delimiter = ','): string {
  if (value === '') return '';
  if (value.includes(delimiter) || value.includes('"') || value.includes('\n') || value.includes('\r')) {
    return '"' + value.replace(/"/g, '""') + '"';
  }
  return value;
}

/** 序列化二维数组为 CSV 文本。 */
export function stringifyCSV(rows: string[][], delimiter = ','): string {
  return rows.map((row) => row.map((f) => escapeField(f, delimiter)).join(delimiter)).join('\r\n');
}

/** 对齐美化 CSV：按列对齐，方便人眼阅读。 */
export function prettyCSV(rows: string[][]): string {
  if (!rows.length) return '';
  const colCount = Math.max(...rows.map((r) => r.length));
  const widths: number[] = [];
  for (let c = 0; c < colCount; c++) {
    widths[c] = Math.max(...rows.map((r) => (r[c] ?? '').length));
  }
  return rows
    .map((row) => row.map((f, c) => (f ?? '').padEnd(widths[c]!)).join('  '))
    .join('\n');
}

/** CSV → 对象数组（第一行为表头）。 */
export function csvToObjects(rows: string[][]): Record<string, string>[] {
  if (rows.length < 2) return [];
  const headers = rows[0]!;
  return rows.slice(1).map((row) => {
    const obj: Record<string, string> = {};
    for (let i = 0; i < headers.length; i++) {
      obj[headers[i]!] = row[i] ?? '';
    }
    return obj;
  });
}

/** 对象数组 → CSV（含表头）。 */
export function objectsToCSV(items: Record<string, unknown>[]): string {
  if (!items.length) return '';
  const headers = Object.keys(items[0]!);
  const rows: string[][] = [headers];
  for (const item of items) {
    rows.push(headers.map((h) => String(item[h] ?? '')));
  }
  return stringifyCSV(rows);
}

/** JSON 数组 → CSV。 */
export function jsonToCSV(jsonText: string): string {
  const data = JSON.parse(jsonText);
  if (!Array.isArray(data)) throw new Error('JSON 必须是数组');
  return objectsToCSV(data as Record<string, unknown>[]);
}

/** CSV → JSON 数组（第一行为表头）。 */
export function csvToJSON(csvText: string): string {
  const rows = parseCSV(csvText);
  const objects = csvToObjects(rows);
  return JSON.stringify(objects, null, 2);
}

/** 自动检测分隔符（逗号 / 分号 / 制表符）。 */
export function detectDelimiter(input: string): string {
  const firstLine = input.split(/\r?\n/)[0] ?? '';
  const counts: Record<string, number> = { ',': 0, ';': 0, '\t': 0, '|': 0 };
  for (const ch of firstLine) {
    if (ch in counts) counts[ch]!++;
  }
  let best = ',';
  let max = 0;
  for (const [d, c] of Object.entries(counts)) {
    if (c > max) { max = c; best = d; }
  }
  return best;
}
