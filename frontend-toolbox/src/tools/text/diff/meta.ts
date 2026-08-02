import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'diff',
  groupId: 'text',
  title: '文本对比',
  summary: '逐行对比两段文本，标注新增 / 删除 / 不变（基于 LCS 算法）。',
  keywords: ['diff', 'compare', 'text', '对比', '差异'],
  icon: '⚖️',
};

export default meta;
