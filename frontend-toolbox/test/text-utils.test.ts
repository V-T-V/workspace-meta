import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  convertCase,
  countText,
  sortLines,
  dedupLines,
  removeEmptyLines,
  diffLines,
  testRegex,
  lorem,
} from '../src/lib/text-utils.ts';

describe('text: convertCase', () => {
  test('upper/lower', () => {
    assert.equal(convertCase('aBc', 'upper'), 'ABC');
    assert.equal(convertCase('aBc', 'lower'), 'abc');
  });
  test('camelCase', () => {
    assert.equal(convertCase('hello world', 'camel'), 'helloWorld');
  });
  test('PascalCase', () => {
    assert.equal(convertCase('hello world', 'pascal'), 'HelloWorld');
  });
  test('snake_case', () => {
    assert.equal(convertCase('HelloWorld', 'snake'), 'hello_world');
  });
  test('kebab-case', () => {
    assert.equal(convertCase('HelloWorld', 'kebab'), 'hello-world');
  });
  test('CONSTANT_CASE', () => {
    assert.equal(convertCase('hello world', 'constant'), 'HELLO_WORLD');
  });
  test('title', () => {
    assert.equal(convertCase('hello world', 'title'), 'Hello World');
  });
});

describe('text: countText', () => {
  test('基本统计', () => {
    const s = 'hello world\n第二行';
    const c = countText(s);
    assert.equal(c.characters, s.length);
    assert.equal(c.lines, 2);
    assert.equal(c.words, 3); // hello world 第二行
  });
  test('空串', () => {
    assert.equal(countText('').words, 0);
    assert.equal(countText('').lines, 0);
  });
});

describe('text: sortLines', () => {
  test('升序', () => {
    assert.equal(sortLines('b\na\nc', 'asc'), 'a\nb\nc');
  });
  test('反转', () => {
    assert.equal(sortLines('a\nb\nc', 'reverse'), 'c\nb\na');
  });
  test('长度升序', () => {
    assert.equal(sortLines('aaa\nb\ncc', 'length-asc'), 'b\ncc\naaa');
  });
});

describe('text: dedupLines', () => {
  test('去重', () => {
    const r = dedupLines('a\nb\na');
    assert.equal(r.output, 'a\nb');
    assert.equal(r.removed, 1);
  });
  test('忽略大小写', () => {
    const r = dedupLines('A\na', false);
    assert.equal(r.output, 'A');
  });
});

describe('text: removeEmptyLines', () => {
  test('移除空行', () => {
    assert.equal(removeEmptyLines('a\n\n  \nb'), 'a\nb');
  });
});

describe('text: diffLines', () => {
  test('差异', () => {
    const d = diffLines('a\nb\nc', 'a\nB\nc');
    assert.ok(d.some((l) => l.type === 'del' && l.text === 'b'));
    assert.ok(d.some((l) => l.type === 'add' && l.text === 'B'));
  });
});

describe('text: testRegex', () => {
  test('全局匹配', () => {
    const r = testRegex('\\d+', 'g', 'a1b22c333');
    assert.equal(r.matches.length, 3);
    assert.equal(r.matches[0]!.match, '1');
  });
  test('捕获组', () => {
    const r = testRegex('(\\w)(\\d)', '', 'a1');
    assert.deepEqual(r.matches[0]!.groups, ['a', '1']);
  });
  test('错误返回 error', () => {
    const r = testRegex('(', '', 'a');
    assert.ok(r.error);
  });
});

describe('text: lorem', () => {
  test('段落数', () => {
    const out = lorem(3, 4);
    assert.equal(out.split('\n\n').length, 3);
  });
});
