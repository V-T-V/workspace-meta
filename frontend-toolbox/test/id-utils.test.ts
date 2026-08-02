import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { uuidV4, uuidCompact, uuids, nanoId, snowflake, ulid, randomToken } from '../src/lib/id-utils.ts';

describe('id: uuidV4', () => {
  test('格式', () => {
    const id = uuidV4();
    assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });
  test('唯一性', () => {
    const set = new Set(uuids(100));
    assert.equal(set.size, 100);
  });
  test('version 是 4', () => {
    assert.equal(uuidV4()[14], '4');
  });
});

describe('id: uuidCompact', () => {
  test('32 位无横线', () => {
    assert.equal(uuidCompact().length, 32);
    assert.ok(!uuidCompact().includes('-'));
  });
});

describe('id: nanoId', () => {
  test('默认长度 21', () => {
    assert.equal(nanoId().length, 21);
  });
  test('自定义长度', () => {
    assert.equal(nanoId(8).length, 8);
  });
  test('唯一性', () => {
    const set = new Set(Array.from({ length: 50 }, () => nanoId()));
    assert.equal(set.size, 50);
  });
});

describe('id: snowflake', () => {
  test('数字字符串', () => {
    const id = snowflake();
    assert.match(id, /^\d+$/);
    assert.ok(id.length >= 10);
  });
  test('单调递增（大致）', () => {
    const a = BigInt(snowflake());
    const b = BigInt(snowflake());
    assert.ok(b > a);
  });
});

describe('id: ulid', () => {
  test('长度 26', () => {
    assert.equal(ulid().length, 26);
  });
  test('Crockford 字母表', () => {
    assert.match(ulid(), /^[0-9A-HJKMNP-TV-Z]{26}$/);
  });
});

describe('id: randomToken', () => {
  test('hex 长度', () => {
    assert.equal(randomToken(16).length, 32); // 16 bytes → 32 hex chars
  });
  test('可配置', () => {
    assert.equal(randomToken(8).length, 16);
  });
});
