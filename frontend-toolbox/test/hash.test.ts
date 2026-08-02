import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { md5, sha1, sha256, bytesToHex, hexToBytes } from '../src/lib/hash.ts';

describe('hash: bytesToHex / hexToBytes', () => {
  test('bytesToHex', () => {
    assert.equal(bytesToHex(new Uint8Array([0, 255, 16])), '00ff10');
  });
  test('hexToBytes 往返', () => {
    const hex = 'deadbeef';
    const bytes = hexToBytes(hex);
    assert.deepEqual(Array.from(bytes), [0xde, 0xad, 0xbe, 0xef]);
    assert.equal(bytesToHex(bytes), hex);
  });
});

describe('hash: MD5（已知向量）', () => {
  test('空串', () => {
    assert.equal(md5(''), 'd41d8cd98f00b204e9800998ecf8427e');
  });
  test('abc', () => {
    assert.equal(md5('abc'), '900150983cd24fb0d6963f7d28e17f72');
  });
  test('消息摘要', () => {
    assert.equal(
      md5('message digest'),
      'f96b697d7cb7938d525a2f31aaf161d0',
    );
  });
  test('中文', () => {
    assert.equal(md5('中文'), 'a7bac2239fcdcb3a067903d8077c4a07');
  });
  test('长文本', () => {
    assert.equal(md5('a'.repeat(64)), '014842d480b571495a4a0363793f7367');
  });
});

describe('hash: SHA（Web Crypto）', () => {
  test('SHA-256 abc', async () => {
    assert.equal(
      await sha256('abc'),
      'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
    );
  });
  test('SHA-1 空串', async () => {
    assert.equal(await sha1(''), 'da39a3ee5e6b4b0d3255bfef95601890afd80709');
  });
  test('SHA-256 中文往返一致性', async () => {
    const a = await sha256('前端工具箱');
    const b = await sha256('前端工具箱');
    assert.equal(a, b);
    assert.equal(a.length, 64);
  });
});
