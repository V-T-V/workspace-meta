import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  parseHex,
  toHex,
  rgbToHsl,
  hslToRgb,
  parseAny,
  describeAll,
  isLight,
  complement,
  adjustLightness,
  hueWheel,
} from '../src/lib/color-utils.ts';

describe('color: parseHex', () => {
  test('3 位', () => {
    assert.deepEqual(parseHex('#abc'), { r: 0xaa, g: 0xbb, b: 0xcc });
  });
  test('6 位', () => {
    assert.deepEqual(parseHex('#ff8800'), { r: 255, g: 136, b: 0 });
  });
  test('无 #', () => {
    assert.deepEqual(parseHex('000000'), { r: 0, g: 0, b: 0 });
  });
  test('无效抛错', () => {
    assert.throws(() => parseHex('#xyz'));
  });
});

describe('color: toHex', () => {
  test('转 hex', () => {
    assert.equal(toHex({ r: 255, g: 136, b: 0 }), '#ff8800');
  });
  test('往返', () => {
    assert.equal(toHex(parseHex('#3a7bd5')), '#3a7bd5');
  });
});

describe('color: rgb <-> hsl 往返', () => {
  test('往返接近', () => {
    const rgb = { r: 100, g: 200, b: 50 };
    const hsl = rgbToHsl(rgb);
    const back = hslToRgb(hsl);
    assert.ok(Math.abs(back.r - rgb.r) <= 2);
    assert.ok(Math.abs(back.g - rgb.g) <= 2);
    assert.ok(Math.abs(back.b - rgb.b) <= 2);
  });
  test('灰色 s=0', () => {
    assert.deepEqual(rgbToHsl({ r: 128, g: 128, b: 128 }), { h: 0, s: 0, l: 50 });
  });
});

describe('color: parseAny', () => {
  test('hex', () => {
    assert.deepEqual(parseAny('#ff0000'), { r: 255, g: 0, b: 0 });
  });
  test('rgb()', () => {
    assert.deepEqual(parseAny('rgb(255, 0, 0)'), { r: 255, g: 0, b: 0 });
  });
  test('hsl()', () => {
    const rgb = parseAny('hsl(0, 100%, 50%)');
    assert.ok(Math.abs(rgb.r - 255) <= 1);
  });
});

describe('color: describeAll', () => {
  test('多种格式输出', () => {
    const c = describeAll('#2563eb');
    assert.ok(c.hex.startsWith('#'));
    assert.ok(c.rgb.startsWith('rgb'));
    assert.ok(c.hsl.startsWith('hsl'));
  });
});

describe('color: 工具函数', () => {
  test('isLight', () => {
    assert.equal(isLight({ r: 255, g: 255, b: 255 }), true);
    assert.equal(isLight({ r: 0, g: 0, b: 0 }), false);
  });
  test('complement', () => {
    assert.deepEqual(complement({ r: 0, g: 0, b: 0 }), { r: 255, g: 255, b: 255 });
  });
  test('adjustLightness 不越界', () => {
    const rgb = adjustLightness({ r: 128, g: 128, b: 128 }, 200);
    assert.ok(rgb.r >= 0 && rgb.r <= 255);
  });
  test('hueWheel 数量', () => {
    assert.equal(hueWheel({ r: 100, g: 100, b: 100 }, 6).length, 6);
  });
});
