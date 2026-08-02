import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { formatCSS, minifyCSS, formatHTML, formatSQL } from '../src/lib/format-utils.ts';
import { convert } from '../src/lib/unit-utils.ts';
import { renderMarkdown } from '../src/lib/markdown-utils.ts';

describe('format-utils: CSS', () => {
  test('美化', () => {
    const out = formatCSS('a{color:red;b:2px}');
    assert.ok(out.includes('a {'));
    assert.ok(out.includes('color: red'));
  });
  test('压缩', () => {
    assert.equal(minifyCSS('a { color : red ; }'), 'a{color:red}');
  });
});

describe('format-utils: HTML', () => {
  test('缩进', () => {
    const out = formatHTML('<div><p>x</p></div>');
    const lines = out.trim().split('\n');
    assert.ok(lines.length >= 2);
    assert.ok(lines[1]!.startsWith('  '));
  });
});

describe('format-utils: SQL', () => {
  test('关键字大写', () => {
    const out = formatSQL('select * from t');
    assert.ok(out.includes('SELECT'));
    assert.ok(out.includes('FROM'));
  });
});

describe('unit-utils: convert', () => {
  test('长度', () => {
    assert.ok(Math.abs(convert('length', 1, 'm', 'cm') - 100) < 0.001);
  });
  test('数据', () => {
    assert.ok(Math.abs(convert('data', 1, 'KB', 'B') - 1024) < 0.001);
  });
  test('温度 C→F', () => {
    assert.ok(Math.abs(convert('temperature', 0, 'C', 'F') - 32) < 0.001);
  });
  test('温度 C→K', () => {
    assert.ok(Math.abs(convert('temperature', 0, 'C', 'K') - 273.15) < 0.001);
  });
});

describe('markdown-utils: renderMarkdown', () => {
  test('标题', () => {
    assert.match(renderMarkdown('# T'), /<h1>T<\/h1>/);
  });
  test('加粗', () => {
    assert.match(renderMarkdown('**x**'), /<strong>x<\/strong>/);
  });
  test('列表', () => {
    const out = renderMarkdown('- a\n- b');
    assert.match(out, /<ul>/);
    assert.match(out, /<li>a<\/li>/);
  });
  test('代码块', () => {
    const out = renderMarkdown('```\ncode\n```');
    assert.match(out, /<pre><code/);
    assert.match(out, /code/);
  });
  test('转义防注入', () => {
    const out = renderMarkdown('<script>alert(1)</script>');
    assert.ok(!out.includes('<script>'));
    assert.ok(out.includes('&lt;script&gt;'));
  });
});
