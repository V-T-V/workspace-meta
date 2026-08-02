import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  parseCSV,
  stringifyCSV,
  prettyCSV,
  csvToObjects,
  objectsToCSV,
  jsonToCSV,
  csvToJSON,
  detectDelimiter,
} from '../src/lib/csv-utils.ts';

describe('csv: parseCSV', () => {
  test('基本解析', () => {
    assert.deepEqual(parseCSV('a,b,c'), [['a', 'b', 'c']]);
  });
  test('多行', () => {
    assert.deepEqual(parseCSV('a,b\n1,2'), [['a', 'b'], ['1', '2']]);
  });
  test('引号内含逗号', () => {
    assert.deepEqual(parseCSV('"a,b",c'), [['a,b', 'c']]);
  });
  test('转义双引号', () => {
    assert.deepEqual(parseCSV('"say ""hi""",x'), [['say "hi"', 'x']]);
  });
  test('CRLF 换行', () => {
    assert.deepEqual(parseCSV('a,b\r\nc,d'), [['a', 'b'], ['c', 'd']]);
  });
});

describe('csv: stringifyCSV', () => {
  test('基本', () => {
    assert.equal(stringifyCSV([['a', 'b']]), 'a,b');
  });
  test('含逗号转义', () => {
    assert.equal(stringifyCSV([['a,b', 'c']]), '"a,b",c');
  });
  test('往返', () => {
    const rows = [['name', 'note'], ['Alice', 'hello, world'], ['Bob', '"quote"']];
    assert.deepEqual(parseCSV(stringifyCSV(rows)), rows);
  });
});

describe('csv: prettyCSV', () => {
  test('对齐', () => {
    const out = prettyCSV([['a', 'bbb'], ['cc', 'd']]);
    assert.ok(out.includes('a  '));
    assert.ok(out.includes('bbb'));
  });
});

describe('csv: objects 互转', () => {
  test('csvToObjects', () => {
    const rows = [['name', 'age'], ['Alice', '30']];
    assert.deepEqual(csvToObjects(rows), [{ name: 'Alice', age: '30' }]);
  });
  test('objectsToCSV 往返', () => {
    const items = [{ name: 'Alice', age: '30' }];
    const csv = objectsToCSV(items);
    assert.ok(csv.includes('name,age'));
    assert.ok(csv.includes('Alice,30'));
  });
});

describe('csv: json/csv 互转', () => {
  test('jsonToCSV', () => {
    const csv = jsonToCSV('[{"a":1,"b":2}]');
    assert.ok(csv.includes('a,b'));
    assert.ok(csv.includes('1,2'));
  });
  test('csvToJSON', () => {
    const json = csvToJSON('name,age\nAlice,30');
    assert.ok(json.includes('"name": "Alice"'));
  });
});

describe('csv: detectDelimiter', () => {
  test('逗号', () => {
    assert.equal(detectDelimiter('a,b,c\n1,2,3'), ',');
  });
  test('分号', () => {
    assert.equal(detectDelimiter('a;b;c'), ';');
  });
  test('制表符', () => {
    assert.equal(detectDelimiter('a\tb\tc'), '\t');
  });
});
