import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  infoFromTimestamp,
  infoFromDate,
  parseToTimestamp,
  relativeTime,
  diffBetween,
} from '../src/lib/time-utils.ts';

describe('time: infoFromTimestamp', () => {
  test('秒级时间戳自动转毫秒', () => {
    const info = infoFromTimestamp(0);
    assert.equal(info.unixSeconds, 0);
    assert.equal(info.year, 1970);
  });
  test('毫秒级', () => {
    const info = infoFromTimestamp(1609459200000); // 2021-01-01 UTC
    assert.equal(info.year, 2021);
  });
  test('ISO 输出', () => {
    const info = infoFromTimestamp(0);
    assert.equal(info.iso8601, '1970-01-01T00:00:00.000Z');
  });
});

describe('time: infoFromDate', () => {
  test('从字符串解析', () => {
    const info = infoFromDate('2024-01-01T00:00:00Z');
    assert.equal(info.year, 2024);
    assert.equal(info.unixSeconds, 1704067200);
  });
});

describe('time: parseToTimestamp', () => {
  test('合法', () => {
    assert.equal(parseToTimestamp('1970-01-01T00:00:00Z'), 0);
  });
  test('非法抛错', () => {
    assert.throws(() => parseToTimestamp('not a date'));
  });
});

describe('time: relativeTime', () => {
  test('过去', () => {
    const past = new Date(Date.now() - 3 * 60 * 1000);
    assert.match(relativeTime(past), /3 分钟前/);
  });
  test('未来', () => {
    const future = new Date(Date.now() + 2 * 60 * 60 * 1000);
    assert.match(relativeTime(future), /2 小时后/);
  });
});

describe('time: diffBetween', () => {
  test('差值', () => {
    const a = new Date('2024-01-01T00:00:00Z');
    const b = new Date('2024-01-02T01:30:00Z');
    const d = diffBetween(a, b);
    assert.equal(d.days, 1);
    assert.equal(d.hours, 1);
    assert.equal(d.minutes, 30);
  });
});
