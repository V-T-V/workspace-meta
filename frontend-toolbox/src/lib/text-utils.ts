// =============================================================================
// 文本处理纯函数库 —— 大小写 / 统计 / 排序 / 去重 / 行操作
// =============================================================================

/** 大小写转换目标。 */
export type CaseMode =
  | 'upper'
  | 'lower'
  | 'title'
  | 'sentence'
  | 'camel'
  | 'pascal'
  | 'snake'
  | 'kebab'
  | 'constant'
  | 'capitalize';

export function convertCase(input: string, mode: CaseMode): string {
  switch (mode) {
    case 'upper':
      return input.toUpperCase();
    case 'lower':
      return input.toLowerCase();
    case 'capitalize':
      return input.replace(/(^\s*\S|[.!?]\s+\S)/g, (m) => m.toUpperCase());
    case 'title':
      return input.replace(/\b\w/g, (m) => m.toUpperCase());
    case 'sentence':
      return input
        .toLowerCase()
        .replace(/(^\s*\S|[.!?]\s+\S)/g, (m) => m.toUpperCase());
    case 'camel':
      return toCamel(input);
    case 'pascal':
      return capitalizeFirst(toCamel(input));
    case 'snake':
      return splitWords(input).join('_').toLowerCase();
    case 'kebab':
      return splitWords(input).join('-').toLowerCase();
    case 'constant':
      return splitWords(input).join('_').toUpperCase();
    default:
      return input;
  }
}

function splitWords(input: string): string[] {
  return input
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2') // camelCase → camel Case
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .split(' ')
    .filter(Boolean);
}

function toCamel(input: string): string {
  const words = splitWords(input).map((w) => w.toLowerCase());
  return words
    .map((w, i) => (i === 0 ? w : capitalizeFirst(w)))
    .join('');
}

function capitalizeFirst(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

// ---------- 统计 ----------

export interface TextStats {
  characters: number;
  charactersNoSpaces: number;
  words: number;
  lines: number;
  paragraphs: number;
  sentences: number;
  bytes: number;
}

export function countText(input: string): TextStats {
  const characters = input.length;
  const charactersNoSpaces = input.replace(/\s/g, '').length;
  const words = (input.trim().match(/\S+/g) ?? []).length;
  const lines = input === '' ? 0 : input.split(/\r\n|\r|\n/).length;
  const paragraphs = input
    .split(/\n\s*\n/)
    .map((p) => p.trim())
    .filter(Boolean).length;
  const sentences = (input.match(/[^.!?。！？]+[.!?。！？]+/g) ?? []).length;
  const bytes = new TextEncoder().encode(input).length;
  return { characters, charactersNoSpaces, words, lines, paragraphs, sentences, bytes };
}

// ---------- 行操作 ----------

export type SortMode = 'asc' | 'desc' | 'length-asc' | 'length-desc' | 'shuffle' | 'reverse';

export function sortLines(input: string, mode: SortMode): string {
  const lines = input.split(/\r\n|\r|\n/);
  switch (mode) {
    case 'asc':
      lines.sort((a, b) => a.localeCompare(b));
      break;
    case 'desc':
      lines.sort((a, b) => b.localeCompare(a));
      break;
    case 'length-asc':
      lines.sort((a, b) => a.length - b.length);
      break;
    case 'length-desc':
      lines.sort((a, b) => b.length - a.length);
      break;
    case 'reverse':
      lines.reverse();
      break;
    case 'shuffle':
      shuffleInPlace(lines);
      break;
  }
  return lines.join('\n');
}

function shuffleInPlace(arr: string[]): void {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j]!, arr[i]!];
  }
}

export function dedupLines(input: string, caseSensitive = true, trim = false): {
  output: string;
  removed: number;
} {
  const rawLines = input.split(/\r\n|\r|\n/);
  const seen = new Set<string>();
  const out: string[] = [];
  for (const line of rawLines) {
    const key = trim ? line.trim() : line;
    const norm = caseSensitive ? key : key.toLowerCase();
    if (seen.has(norm)) continue;
    seen.add(norm);
    out.push(line);
  }
  return { output: out.join('\n'), removed: rawLines.length - out.length };
}

/** 移除空行（含纯空白行）。 */
export function removeEmptyLines(input: string): string {
  return input
    .split(/\r\n|\r|\n/)
    .filter((l) => l.trim() !== '')
    .join('\n');
}

/** 移除每行首尾空白。 */
export function trimLines(input: string): string {
  return input
    .split(/\r\n|\r|\n/)
    .map((l) => l.trim())
    .join('\n');
}

// ---------- 正则测试 ----------

export interface RegexTestResult {
  matches: Array<{ match: string; index: number; groups: string[] }>;
  error?: string;
}

export function testRegex(
  pattern: string,
  flags: string,
  input: string,
): RegexTestResult {
  try {
    const re = new RegExp(pattern, flags);
    const result: RegexTestResult = { matches: [] };
    if (flags.includes('g')) {
      let m: RegExpExecArray | null;
      re.lastIndex = 0;
      while ((m = re.exec(input)) !== null) {
        result.matches.push({
          match: m[0],
          index: m.index,
          groups: m.slice(1),
        });
        if (m.index === re.lastIndex) re.lastIndex++;
      }
    } else {
      const m = re.exec(input);
      if (m) result.matches.push({ match: m[0], index: m.index, groups: m.slice(1) });
    }
    return result;
  } catch (e) {
    return { matches: [], error: (e as Error).message };
  }
}

// ---------- 文本差异（简化 LCS） ----------

export interface DiffLine {
  type: 'equal' | 'add' | 'del';
  text: string;
}

export function diffLines(a: string, b: string): DiffLine[] {
  const la = a.split(/\r\n|\r|\n/);
  const lb = b.split(/\r\n|\r|\n/);
  // LCS dp 表
  const m = la.length,
    n = lb.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      dp[i]![j] =
        la[i] === lb[j] ? dp[i + 1]![j + 1]! + 1 : Math.max(dp[i + 1]![j]!, dp[i]![j + 1]!);
    }
  }
  const out: DiffLine[] = [];
  let i = 0,
    j = 0;
  while (i < m && j < n) {
    if (la[i] === lb[j]) {
      out.push({ type: 'equal', text: la[i]! });
      i++;
      j++;
    } else if (dp[i + 1]![j]! >= dp[i]![j + 1]!) {
      out.push({ type: 'del', text: la[i]! });
      i++;
    } else {
      out.push({ type: 'add', text: lb[j]! });
      j++;
    }
  }
  while (i < m) out.push({ type: 'del', text: la[i++]! });
  while (j < n) out.push({ type: 'add', text: lb[j++]! });
  return out;
}

// ---------- 随机文本 / Lorem ----------

const LOREM_WORDS =
  'lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua enim ad minim veniam quis nostrud exercitation ullamco laboris nisi aliquip ex ea commodo consequat duis aute irure in reprehenderit voluptate velit esse cillum eu fugiat nulla pariatur excepteur sint occaecat cupidatat non proident sunt culpa qui officia deserunt mollit anim id est laborum'.split(
    ' ',
  );

export function lorem(paragraphs: number, sentencesPerPara = 4): string {
  const out: string[] = [];
  for (let p = 0; p < paragraphs; p++) {
    const ss: string[] = [];
    for (let s = 0; s < sentencesPerPara; s++) {
      const len = 8 + Math.floor(Math.random() * 12);
      const words: string[] = [];
      for (let i = 0; i < len; i++) {
        words.push(LOREM_WORDS[Math.floor(Math.random() * LOREM_WORDS.length)]!);
      }
      let sentence = words.join(' ');
      sentence = sentence.charAt(0).toUpperCase() + sentence.slice(1) + '.';
      ss.push(sentence);
    }
    out.push(ss.join(' '));
  }
  return out.join('\n\n');
}

export function randomString(
  length: number,
  charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789',
): string {
  let out = '';
  const arr = new Uint32Array(length);
  crypto.getRandomValues(arr);
  for (let i = 0; i < length; i++) out += charset[arr[i]! % charset.length];
  return out;
}
