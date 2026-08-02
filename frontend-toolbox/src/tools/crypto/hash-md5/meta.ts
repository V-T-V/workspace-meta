import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'hash-md5',
  groupId: 'crypto',
  title: 'MD5 哈希',
  summary: '计算输入文本的 MD5 摘要（128 位 / 32 位 hex）。仅用于校验，已不安全。',
  keywords: ['md5', 'hash', '摘要', 'checksum'],
  icon: '🔐',
};

export default meta;
