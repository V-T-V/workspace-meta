import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'bcrypt-info',
  groupId: 'crypto',
  title: 'bcrypt 哈希信息',
  summary: '解析 bcrypt 哈希字符串，展示版本、cost、盐值与摘要部分。',
  keywords: ['bcrypt', 'hash', 'password', 'cost', '盐', '密码'],
  icon: '🔑',
};

export default meta;
