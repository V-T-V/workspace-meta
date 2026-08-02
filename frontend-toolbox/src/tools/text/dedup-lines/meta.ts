import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'dedup-lines',
  groupId: 'text',
  title: '行去重',
  summary: '去除重复行，可选忽略大小写与首尾空白，附移除空行、首尾 trim。',
  keywords: ['dedup', 'unique', 'distinct', '去重', '重复'],
  icon: '🎏',
};

export default meta;
