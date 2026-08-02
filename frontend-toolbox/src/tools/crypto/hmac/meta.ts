import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'hmac',
  groupId: 'crypto',
  title: 'HMAC 签名',
  summary: '用密钥对消息计算 HMAC（SHA-1/256/512），用于接口签名校验。',
  keywords: ['hmac', 'signature', 'sign', '签名', '密钥'],
  icon: '✍️',
};

export default meta;
