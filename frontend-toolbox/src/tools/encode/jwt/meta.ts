import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'jwt',
  groupId: 'encode',
  title: 'JWT 解析',
  summary: '解码 JWT 三段（Header / Payload / Signature），展示声明与过期时间。',
  keywords: ['jwt', 'json', 'web', 'token', '解析'],
  icon: '🪪',
};

export default meta;
