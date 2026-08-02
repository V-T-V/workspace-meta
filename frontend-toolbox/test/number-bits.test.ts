import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  toBinary32,
  toBinary8,
  radixStrings,
  parseIEEE754,
  bitOperations,
  numberToBits,
  bitsToNumber,
} from '../src/lib/number-bits.ts';

describe('number-bits: toBinary', () => {
  test('8 位', () => {
    assert.equal(toBinary8(5), '00000101');
    assert.equal(toBinary8(255), '11111111');
  });
  test('32 位分组', () => {
    const b = toBinary32(1);
    assert.ok(b.includes('00000001'));
    assert.equal(b.split(' ').length, 4); // 4 组 × 8 位
  });
  test('负数补码', () => {
    assert.equal(toBinary32(-1).replace(/ /g, ''), '11111111111111111111111111111111');
  });
});

describe('number-bits: radixStrings', () => {
  test('255', () => {
    const r = radixStrings(255);
    assert.equal(r.dec, '255');
    assert.equal(r.bin, '11111111');
    assert.equal(r.oct, '377');
    assert.equal(r.hex, 'FF');
  });
});

describe('number-bits: IEEE754', () => {
  test('1.0', () => {
    const f = parseIEEE754(1.0);
    assert.equal(f.sign, 0);
    assert.equal(f.exponent, 1023);
    assert.ok(f.mantissaBits.split('').every((b) => b === '0'));
  });
  test('0', () => {
    const f = parseIEEE754(0);
    assert.equal(f.isZero, true);
  });
  test('-1.0', () => {
    const f = parseIEEE754(-1.0);
    assert.equal(f.sign, 1);
    assert.equal(f.exponent, 1023);
  });
  test('Infinity', () => {
    const f = parseIEEE754(Infinity);
    assert.equal(f.isInfinity, true);
  });
  test('NaN', () => {
    const f = parseIEEE754(NaN);
    assert.equal(f.isNaN, true);
  });
});

describe('number-bits: 位运算', () => {
  test('AND', () => {
    const ops = bitOperations(12, 10); // 1100 & 1010 = 1000
    const and = ops[0]!;
    assert.equal(and.result, 8);
  });
  test('OR', () => {
    const ops = bitOperations(12, 10);
    const or = ops[1]!;
    assert.equal(or.result, 14);
  });
  test('XOR', () => {
    const ops = bitOperations(12, 10);
    const xor = ops[2]!;
    assert.equal(xor.result, 6);
  });
});

describe('number-bits: 位掩码', () => {
  test('numberToBits', () => {
    const bits = numberToBits(5, 4); // 0101 低位在前
    assert.deepEqual(bits, [true, false, true, false]);
  });
  test('bitsToNumber 往返', () => {
    const bits = numberToBits(13, 8);
    assert.equal(bitsToNumber(bits), 13);
  });
});
