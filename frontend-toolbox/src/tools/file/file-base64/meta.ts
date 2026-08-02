import type { ToolMeta } from '../../../types.ts';

const meta: ToolMeta = {
  id: 'file-base64',
  groupId: 'file',
  title: '文件 ↔ Base64',
  summary: '把文件读成 Data URL（Base64），或反向解码 Base64 为文件下载。',
  keywords: ['file', 'base64', 'dataurl', 'data url', '文件', '编码'],
  icon: '📁',
};

export default meta;
