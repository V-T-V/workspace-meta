import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  tryParseJSON,
  formatJSON,
  minifyJSON,
  sortObjectKeys,
  escapeJSONString,
  unescapeJSONString,
  statsJSON,
  getByPath,
} from '../src/lib/json-utils.ts';

describe('json: tryParseJSON', () => {
  test('合法', () => {
    const r = tryParseJSON('{"a":1}');
    assert.equal(r.ok, true);
    if (r.ok) assert.deepEqual(r.value, { a: 1 });
  });
  test('非法返回 error', () => {
    const r = tryParseJSON('{a:1}');
    assert.equal(r.ok, false);
    if (!r.ok) assert.ok(r.error.length > 0);
  });
});

describe('json: formatJSON', () => {
  test('缩进', () => {
    assert.equal(formatJSON('{"a":1}'), '{\n  "a": 1\n}\n');
  });
  test('压缩 indent=0', () => {
    assert.equal(formatJSON('{"a":1}', { indent: 0 }), '{"a":1}');
  });
  test('key 排序', () => {
    assert.equal(
      formatJSON('{"b":1,"a":2}', { sortKeys: true }),
      '{\n  "a": 2,\n  "b": 1\n}\n',
    );
  });
});

describe('json: minifyJSON', () => {
  test('压缩', () => {
    assert.equal(minifyJSON('{\n  "a" : 1,\n  "b" : 2\n}'), '{"a":1,"b":2}');
  });
});

describe('json: sortObjectKeys', () => {
  test('嵌套排序', () => {
    assert.deepEqual(sortObjectKeys({ b: 1, a: { d: 2, c: 3 } }), {
      a: { c: 3, d: 2 },
      b: 1,
    });
  });
  test('数组顺序保留', () => {
    assert.deepEqual(sortObjectKeys({ z: [3, 1, 2] }), { z: [3, 1, 2] });
  });
});

describe('json: escape / unescape string', () => {
  test('转义', () => {
    assert.equal(escapeJSONString('a"b'), '"a\\"b"');
  });
  test('反转义', () => {
    assert.equal(unescapeJSONString('"a\\"b"'), 'a"b');
  });
  test('无引号也能反', () => {
    assert.equal(unescapeJSONString('a\\"b'), 'a"b');
  });
});

describe('json: statsJSON', () => {
  test('对象统计', () => {
    const s = statsJSON({ a: { b: 1 }, c: [1, 2] });
    assert.equal(s.keys, 3);
    assert.equal(s.arrayItems, 2);
    assert.equal(s.depth, 2);
  });
});

describe('json: getByPath', () => {
  test('点号取值', () => {
    assert.equal(getByPath({ a: { b: 5 } }, 'a.b'), 5);
  });
  test('数组索引', () => {
    assert.equal(getByPath({ a: [10, 20] }, 'a[1]'), 20);
  });
  test('不存在返回 undefined', () => {
    assert.equal(getByPath({ a: 1 }, 'b'), undefined);
  });
});
