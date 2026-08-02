import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'unicode',
  groupId: 'encode',
  title: 'Unicode 转义',
  summary: '字符与 \\uXXXX / \\u{XXXXXX} 转义序列互转，查看码点。',
  keywords: ['unicode', 'codepoint', 'escape', '码点', '转义'],
  icon: '🌐',
};

export default meta;
