// =============================================================================
// 格式化纯函数库 —— CSS / HTML / XML / SQL 的轻量美化（启发式，非完整解析器）
// 不引第三方库，适合工具展示；复杂输入可能不完美，但对日常粘贴足够。
// =============================================================================

// ---------- CSS ----------

export function formatCSS(input: string, indent = 2): string {
  const pad = ' '.repeat(indent);
  // 先压平：去换行、规整空格
  const flat = input
    .replace(/\/\*[\s\S]*?\*\//g, (m) => '\n' + m + '\n') // 保留注释
    .replace(/\s*\{\s*/g, ' {\n')
    .replace(/\s*;\s*/g, ';\n')
    .replace(/\s*:\s*/g, ': ')
    .replace(/\s*,\s*/g, ', ')
    .replace(/\s*\}\s*/g, '\n}\n')
    .replace(/\n{2,}/g, '\n')
    .trim();

  const lines = flat.split('\n');
  let depth = 0;
  const out: string[] = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed === '') continue;
    if (trimmed.endsWith('}')) depth = Math.max(0, depth - 1);
    out.push(pad.repeat(depth) + trimmed);
    if (trimmed.endsWith('{')) depth++;
  }
  return out.join('\n') + '\n';
}

export function minifyCSS(input: string): string {
  return input
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\s+/g, ' ')
    .replace(/\s*([{}:;,])\s*/g, '$1')
    .replace(/;}/g, '}')
    .trim();
}

// ---------- HTML / XML（基于标签的缩进）----------

export function formatHTML(input: string, indent = 2): string {
  const pad = ' '.repeat(indent);
  // 先规范化：标签外文本保留，标签独占行
  const tokens = input
    .replace(/>\s*</g, '><\n')
    .replace(/<(\/?)([a-zA-Z][\w-]*)([^>]*)>/g, '\n<$1$2$3>')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

  // 不缩进内容的标签（自闭合或预期单行）
  const inlineTags = new Set(['a', 'span', 'strong', 'em', 'b', 'i', 'code', 'small', 'br', 'img', 'input']);
  const voidTags = new Set(['br', 'img', 'input', 'meta', 'link', 'hr', 'area', 'base', 'col', 'embed', 'source']);

  let depth = 0;
  const out: string[] = [];
  for (const tok of tokens) {
    const closeMatch = tok.match(/^<\/([a-zA-Z][\w-]*)/);
    const openMatch = tok.match(/^<([a-zA-Z][\w-]*)([^>]*)>/);
    const selfClosed = /\/>$/.test(tok) || (openMatch && voidTags.has(openMatch[1]!.toLowerCase()));

    if (closeMatch) {
      depth = Math.max(0, depth - 1);
      out.push(pad.repeat(depth) + tok);
    } else if (selfClosed) {
      out.push(pad.repeat(depth) + tok);
    } else if (openMatch) {
      const tag = openMatch[1]!.toLowerCase();
      out.push(pad.repeat(depth) + tok);
      if (!inlineTags.has(tag)) depth++;
    } else {
      // 文本节点
      out.push(pad.repeat(depth) + tok);
    }
  }
  return out.join('\n') + '\n';
}

export const formatXML = (input: string, indent = 2): string => formatHTML(input, indent);

// ---------- SQL（关键字大写 + 缩进，启发式）----------

const SQL_KEYWORDS = [
  'SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'NOT', 'IN', 'LIKE', 'BETWEEN', 'IS', 'NULL',
  'JOIN', 'INNER', 'LEFT', 'RIGHT', 'OUTER', 'FULL', 'ON', 'UNION', 'ALL', 'GROUP BY',
  'ORDER BY', 'HAVING', 'LIMIT', 'OFFSET', 'INSERT INTO', 'VALUES', 'UPDATE', 'SET',
  'DELETE FROM', 'CREATE TABLE', 'DROP TABLE', 'ALTER TABLE', 'ADD', 'COLUMN',
  'AS', 'DISTINCT', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END', 'EXISTS',
];

export function formatSQL(input: string, indent = 2): string {
  const pad = ' '.repeat(indent);
  let s = input.replace(/\s+/g, ' ').trim();
  // 关键字大写
  for (const kw of SQL_KEYWORDS) {
    s = s.replace(new RegExp('\\b' + kw.replace(/ /g, '\\s+') + '\\b', 'gi'), kw);
  }
  // 在主要子句前换行
  const clauses = ['SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'JOIN', 'UNION', 'VALUES', 'SET'];
  for (const cl of clauses) {
    s = s.replace(new RegExp('\\s+' + cl.replace(/ /g, '\\s+') + '\\b', 'g'), '\n' + cl);
  }
  // SELECT 字段逗号后换行缩进
  s = s.split('\n').map((line) => {
    if (line.startsWith('SELECT')) {
      return line.replace(/,\s*/g, ',\n' + pad);
    }
    return line;
  }).join('\n');
  // 分号收尾
  if (!s.endsWith(';')) s += ';';
  return s + '\n';
}
