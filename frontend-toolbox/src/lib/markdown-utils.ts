// =============================================================================
// 轻量 Markdown 渲染（纯函数，无依赖）
// 支持标题/加粗/斜体/行内代码/链接/图片/列表/引用/代码块/分隔线/表格。
// 输出 HTML 已对原始文本做转义，防 XSS。
// =============================================================================

import { encodeHTML as escapeHTML } from './codec.ts';

/** 渲染完整 Markdown 文档为 HTML。 */
export function renderMarkdown(md: string): string {
  const lines = md.replace(/\r\n/g, '\n').split('\n');
  const html: string[] = [];
  let i = 0;
  let inUl = false;
  let inOl = false;
  let inQuote = false;

  const closeLists = (): void => {
    if (inUl) {
      html.push('</ul>');
      inUl = false;
    }
    if (inOl) {
      html.push('</ol>');
      inOl = false;
    }
  };
  const closeQuote = (): void => {
    if (inQuote) {
      html.push('</blockquote>');
      inQuote = false;
    }
  };

  while (i < lines.length) {
    const line = lines[i]!;

    // 代码块
    const fence = line.match(/^```(\w*)/);
    if (fence) {
      closeLists();
      closeQuote();
      const lang = fence[1] ?? '';
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i]!.startsWith('```')) {
        code.push(lines[i]!);
        i++;
      }
      i++; // 跳过结束 ```
      html.push(`<pre><code class="lang-${escapeHTML(lang)}">${escapeHTML(code.join('\n'))}</code></pre>`);
      continue;
    }

    // 标题
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      closeLists();
      closeQuote();
      const level = h[1]!.length;
      html.push(`<h${level}>${renderInline(h[2]!)}</h${level}>`);
      i++;
      continue;
    }

    // 分隔线
    if (/^(\*\*\*|---|___)\s*$/.test(line)) {
      closeLists();
      closeQuote();
      html.push('<hr/>');
      i++;
      continue;
    }

    // 引用
    const quote = line.match(/^>\s?(.*)$/);
    if (quote) {
      closeLists();
      if (!inQuote) {
        html.push('<blockquote>');
        inQuote = true;
      }
      html.push(`<p>${renderInline(quote[1]!)}</p>`);
      i++;
      continue;
    } else {
      closeQuote();
    }

    // 无序列表
    if (/^[-*+]\s+/.test(line)) {
      if (!inUl) {
        closeLists();
        html.push('<ul>');
        inUl = true;
      }
      const item = line.replace(/^[-*+]\s+/, '');
      html.push(`<li>${renderInline(item)}</li>`);
      i++;
      continue;
    }

    // 有序列表
    const ol = line.match(/^\d+\.\s+(.*)$/);
    if (ol) {
      if (!inOl) {
        closeLists();
        html.push('<ol>');
        inOl = true;
      }
      html.push(`<li>${renderInline(ol[1]!)}</li>`);
      i++;
      continue;
    }

    closeLists();

    // 空行
    if (line.trim() === '') {
      i++;
      continue;
    }

    // 表格（简单两行表头）
    if (line.includes('|') && i + 1 < lines.length && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1]!)) {
      const headers = splitRow(line);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i]!.includes('|')) {
        rows.push(splitRow(lines[i]!));
        i++;
      }
      html.push('<table>');
      html.push('<thead><tr>' + headers.map((h) => `<th>${renderInline(h)}</th>`).join('') + '</tr></thead>');
      html.push('<tbody>');
      for (const row of rows) {
        html.push('<tr>' + row.map((c) => `<td>${renderInline(c)}</td>`).join('') + '</tr>');
      }
      html.push('</tbody></table>');
      continue;
    }

    // 段落（合并连续非空非特殊行）
    const para: string[] = [line];
    i++;
    while (
      i < lines.length &&
      lines[i]!.trim() !== '' &&
      !/^(#{1,6})\s/.test(lines[i]!) &&
      !/^```/.test(lines[i]!) &&
      !/^[-*+]\s/.test(lines[i]!) &&
      !/^\d+\.\s/.test(lines[i]!) &&
      !/^>/.test(lines[i]!)
    ) {
      para.push(lines[i]!);
      i++;
    }
    html.push(`<p>${renderInline(para.join(' '))}</p>`);
  }
  closeLists();
  closeQuote();
  return html.join('\n');
}

function splitRow(line: string): string[] {
  return line
    .replace(/^\s*\|/, '')
    .replace(/\|\s*$/, '')
    .split('|')
    .map((c) => c.trim());
}

/** 行内渲染：加粗/斜体/代码/链接/图片/删除线。 */
export function renderMarkdownInline(text: string): string {
  return renderInline(text);
}

function renderInline(text: string): string {
  let s = escapeHTML(text);
  // 图片 ![alt](url)
  s = s.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img alt="$1" src="$2"/>');
  // 链接 [text](url)
  s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  // 加粗 **text** 或 __text__
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  s = s.replace(/__([^_]+)__/g, '<strong>$1</strong>');
  // 斜体 *text* 或 _text_
  s = s.replace(/(^|[^*])\*([^*\s][^*]*?)\*/g, '$1<em>$2</em>');
  s = s.replace(/(^|[^_])_([^_\s][^_]*?)_/g, '$1<em>$2</em>');
  // 删除线 ~~text~~
  s = s.replace(/~~([^~]+)~~/g, '<del>$1</del>');
  // 行内代码 `code`
  s = s.replace(/`([^`]+)`/g, '<code>$1</code>');
  return s;
}
