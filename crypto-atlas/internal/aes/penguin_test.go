package aes

import (
	"bytes"
	"testing"
)

// TestPenguinDemo 是 ECB 企鹅图的核心教学断言：
//   - ECB 加密后存在重复块（相同的"身体"明文块→相同密文块）→ 模式泄露
//   - CBC 加密后无重复块（链式 XOR 打散一切）→ 消除泄露
func TestPenguinDemo(t *testing.T) {
	res, err := RunPenguinDemo()
	if err != nil {
		t.Fatalf("RunPenguinDemo 出错: %v", err)
	}

	// 图像必须是 8x8。
	if len(res.PlaintextImage) != 8 {
		t.Fatalf("明文图像应有 8 行，实际 %d", len(res.PlaintextImage))
	}
	for i, row := range res.PlaintextImage {
		if len(row) != 8 {
			t.Fatalf("明文第 %d 行长度应为 8，实际 %d", i, len(row))
		}
	}

	// ECB：必须复现重复块（企鹅图的灵魂）。
	if !res.ECBDuplicates || res.ECBDuplicatePairs < 1 {
		t.Errorf("ECB 应存在重复密文块（模式泄露），实际 duplicates=%v pairs=%d",
			res.ECBDuplicates, res.ECBDuplicatePairs)
	}

	// CBC：不应有重复块。
	if res.CBCDuplicates || res.CBCDuplicatePairs != 0 {
		t.Errorf("CBC 不应有重复密文块，实际 duplicates=%v pairs=%d",
			res.CBCDuplicates, res.CBCDuplicatePairs)
	}

	// ECB 的重复程度必须严格大于 CBC（教学对比要点）。
	if res.ECBDuplicatePairs <= res.CBCDuplicatePairs {
		t.Errorf("ECB 重复块对数 (%d) 应大于 CBC (%d)",
			res.ECBDuplicatePairs, res.CBCDuplicatePairs)
	}
}

// TestPenguinImageHasDuplicatePlaintextBlock 验证明文图本身确实构造了两个
// 完全相同的 16 字节明文块（行 2-3 == 行 4-5）。这是 ECB 模式泄露的"罪证源头"。
func TestPenguinImageHasDuplicatePlaintextBlock(t *testing.T) {
	plain, err := ImageToBytes(penguinImage)
	if err != nil {
		t.Fatal(err)
	}
	blocks := splitBlocks(plain)
	if len(blocks) != 4 {
		t.Fatalf("8x8=64 字节应切成 4 块，实际 %d", len(blocks))
	}
	// block1（行 2-3）与 block2（行 4-5）应完全相同。
	if !bytes.Equal(blocks[1], blocks[2]) {
		t.Errorf("明文 block1 与 block2 应相同（构造企鹅身体重复带）\nblock1=%x\nblock2=%x",
			blocks[1], blocks[2])
	}
	// 其余块之间不应偶然相等（保证 ECB 重复只来自我们构造的那一对）。
	if bytes.Equal(blocks[0], blocks[1]) || bytes.Equal(blocks[0], blocks[3]) ||
		bytes.Equal(blocks[2], blocks[3]) {
		t.Error("明文除 block1==block2 外不应有其它相等块")
	}
}

// TestPenguinECBMatchesBlockEncryption 验证 ECB 企鹅图的每个密文块
// 都等于对应明文块单独用 ECB 加密的结果（逐块独立）。
// 这正是 ECB 的定义，也是模式泄露的根因。
func TestPenguinECBMatchesBlockEncryption(t *testing.T) {
	plain, _ := ImageToBytes(penguinImage)
	ecbBlocks, err := EncryptImageECB(penguinImage, PenguinKey)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		single, err := EncryptECB(plain[i*16:(i+1)*16], PenguinKey)
		if err != nil {
			t.Fatal(err)
		}
		// single 经 PKCS7 填充后是 32 字节（补一整块），取第一块比较。
		if !bytes.Equal(ecbBlocks[i], single[:16]) {
			t.Errorf("block %d: ECB 逐块加密结果不一致", i)
		}
	}
	// 由于明文 block1==block2，ECB 密文 block1 也必须 == block2。
	if !bytes.Equal(ecbBlocks[1], ecbBlocks[2]) {
		t.Error("明文 block1==block2 时，ECB 密文 block1 必须 == block2")
	}
}

// TestPenguinCBCNoRepeatedBlockEvenWithRepeatedPlaintext 再次强调：
// 即使明文有两个完全相同的块，CBC 也不会让密文出现重复块。
func TestPenguinCBCNoRepeatedBlockEvenWithRepeatedPlaintext(t *testing.T) {
	cbcBlocks, err := EncryptImageCBC(penguinImage, PenguinKey, PenguinIV)
	if err != nil {
		t.Fatal(err)
	}
	if HasDuplicateBlock(cbcBlocks) {
		t.Error("CBC 不应在有重复明文块的情况下仍产生重复密文块")
	}
}

// TestImageToBytesRejectsNonASCII 确保非 ASCII 字符（多字节 rune）被拒绝，
// 以维持"一格像素 = 一字节"的演示语义。
func TestImageToBytesRejectsNonASCII(t *testing.T) {
	bad := [8]string{
		"........",
		"..o@@o..",
		".######.",
		"##@@@@##",
		".######.",
		"##@@@@##",
		"..####..",
		"...企鹅..", // 含中文
	}
	if _, err := ImageToBytes(bad); err == nil {
		t.Error("含非 ASCII 字符的图像应报错")
	}
}

// TestImageToBytesRejectsWrongWidth 确保行长不是 8 时报错。
func TestImageToBytesRejectsWrongWidth(t *testing.T) {
	bad := [8]string{
		".....", // 5
		"........",
		"........",
		"........",
		"........",
		"........",
		"........",
		"........",
	}
	if _, err := ImageToBytes(bad); err == nil {
		t.Error("行长非 8 应报错")
	}
}

// TestBytesToImageIsPrintable 确保密文图每行都是可打印 ASCII。
func TestBytesToImageIsPrintable(t *testing.T) {
	ecbBlocks, err := EncryptImageECB(penguinImage, PenguinKey)
	if err != nil {
		t.Fatal(err)
	}
	img := BytesToImage(bytes.Join(ecbBlocks, nil))
	for r, row := range img {
		for c, ch := range row {
			if ch < 0x20 || ch > 0x7e {
				t.Errorf("密文图 [%d][%d] 字符码点 %d 不可打印", r, c, ch)
			}
		}
	}
}
