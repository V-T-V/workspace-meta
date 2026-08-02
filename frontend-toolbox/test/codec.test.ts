import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  encodeBase64,
  decodeBase64,
  encodeURL,
  decodeURL,
  encodeHex,
  decodeHex,
  encodeHTML,
  decodeHTML,
  encodeUnicodeEscape,
  decodeUnicodeEscape,
  encodeBinary,
  decodeBinary,
} from '../src/lib/codec.ts';

describe('codec: Base64', () => {
  test('ASCII 往返', () => {
    assert.equal(encodeBase64('Hello'), 'SGVsbG8=');
    assert.equal(decodeBase64('SGVsbG8='), 'Hello');
  });
  test('UTF-8 往返', () => {
    const s = '前端工具箱 🧰';
    assert.equal(decodeBase64(encodeBase64(s)), s);
  });
  test('空串', () => {
    assert.equal(encodeBase64(''), '');
    assert.equal(decodeBase64(''), '');
  });
});

describe('codec: URL', () => {
  test('标准编码解码', () => {
    const s = 'a b&c=中文';
    assert.equal(decodeURL(encodeURL(s)), s);
  });
  test('+ 号当空格', () => {
    assert.equal(decodeURL('a+b'), 'a b');
  });
});

describe('codec: HEX', () => {
  test('编码', () => {
    assert.equal(encodeHex('AB'), '4142');
  });
  test('解码往返', () => {
    const s = 'Hi 中文';
    assert.equal(decodeHex(encodeHex(s)), s);
  });
  test('忽略空格与 0x', () => {
    assert.equal(decodeHex('0x41 0x42'), 'AB');
  });
  test('奇数位抛错', () => {
    assert.throws(() => decodeHex('ABC'));
  });
});

describe('codec: HTML 实体', () => {
  test('转义特殊字符', () => {
    assert.equal(encodeHTML('<a>&"\''), '&lt;a&gt;&amp;&quot;&#39;');
  });
  test('反转义命名实体', () => {
    assert.equal(decodeHTML('&lt;a&gt;'), '<a>');
  });
  test('反转义数字实体', () => {
    assert.equal(decodeHTML('&#65;&#x42;'), 'AB');
  });
  test('往返', () => {
    const s = '<div class="x">Tom & Jerry</div>';
    assert.equal(decodeHTML(encodeHTML(s)), s);
  });
});

describe('codec: Unicode 转义', () => {
  test('BMP 字符', () => {
    assert.equal(encodeUnicodeEscape('A'), '\\u0041');
    assert.equal(decodeUnicodeEscape('\\u0041'), 'A');
  });
  test('emoji 用花括号', () => {
    const e = '🧰';
    assert.equal(decodeUnicodeEscape(encodeUnicodeEscape(e)), e);
  });
});

describe('codec: 二进制', () => {
  test('往返', () => {
    const s = 'AB';
    assert.equal(decodeBinary(encodeBinary(s)), s);
  });
  test('格式化', () => {
    assert.equal(encodeBinary('A'), '01000001');
  });
});
