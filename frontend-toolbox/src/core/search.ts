// =============================================================================
// 工具搜索 —— 简单的标题/关键词/拼音首字母模糊匹配
// 纯函数，输入 metas 列表 + 查询，返回排序后的结果。
// =============================================================================

import type { ToolMeta } from '../types.ts';

export interface SearchResult {
  meta: ToolMeta;
  /** 命中得分，越高越靠前。 */
  score: number;
}

/** 简单拼音首字母表（仅覆盖常见字，缺失则按汉字本身匹配）。 */
const PINYIN_INITIAL: Readonly<Record<string, string>> = {
  格式化: 'gsh', 编码: 'bm', 解码: 'jm', 加密: 'jm', 哈希: 'hx',
  时间: 'sj', 戳: 'c', 转换: 'zh', 颜色: 'ys', 进制: 'jz',
  单位: 'dw', 图片: 'tp', 压缩: 'ys', 裁剪: 'cj', 二维码: 'ewm',
  文本: 'wb', 文字: 'wz', 统计: 'tj', 排序: 'px', 去重: 'qc',
  正则: 'zz', 随机: 'sj', 生成: 'sc', 字符: 'zf', 字符串: 'zfc',
  对比: 'db', 差异: 'cy', 预览: 'yl', 美化: 'mh',
};

function pinyinInitials(s: string): string {
  // 滑窗匹配 2-3 字的常见词首字母
  let out = '';
  for (const [word, ini] of Object.entries(PINYIN_INITIAL)) {
    if (s.includes(word)) out += ini;
  }
  return out;
}

/** 单个 meta 的可搜索文本（小写拼接）。 */
function searchableText(meta: ToolMeta): string {
  return [
    meta.title,
    meta.summary,
    ...(meta.keywords ?? []),
    pinyinInitials(meta.title),
  ]
    .join(' ')
    .toLowerCase();
}

export function search(metas: readonly ToolMeta[], query: string): SearchResult[] {
  const q = query.trim().toLowerCase();
  if (!q) return metas.map((meta) => ({ meta, score: 0 }));
  const terms = q.split(/\s+/);
  const results: SearchResult[] = [];
  for (const meta of metas) {
    const text = searchableText(meta);
    let score = 0;
    let allHit = true;
    for (const term of terms) {
      if (text.includes(term)) {
        // 标题命中权重更高
        if (meta.title.toLowerCase().includes(term)) score += 10;
        else if ((meta.keywords ?? []).some((k) => k.toLowerCase().includes(term))) score += 5;
        else score += 1;
      } else {
        allHit = false;
        break;
      }
    }
    if (allHit) results.push({ meta, score });
  }
  return results.sort((a, b) => b.score - a.score);
}
