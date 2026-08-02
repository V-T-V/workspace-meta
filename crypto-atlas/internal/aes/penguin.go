package aes

import (
	"bytes"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// Penguin 演示 AES-ECB 的"模式泄露"——著名的 ECB 企鹅图（ECB penguin）的文本版。
//
// 背景故事：把一张企鹅位图用 ECB 模式加密后，密文里企鹅的轮廓依然清晰可见，
// 因为 ECB 对每个分组独立加密：相同的 16 字节明文永远产生相同的 16 字节密文。
// 于是图像里大片相同颜色的区域（背景、身体）加密后仍聚成相同的块，轮廓不散。
// 这说明 ECB 根本没有"语义安全"——它只加密"内容"，不掩盖"结构"。
//
// 本文件不依赖任何图像库，用 8x8 的 rune 矩阵（每格一个 ASCII 字符代表灰度）
// 模拟一张图，足够展示"轮廓在 ECB 下保留、在 CBC 下被打破"的核心现象。
//
// 图像设计（8 列 × 8 行，共 64 字节 = 4 个 AES 分组）：
//
//	........      ← 行 0  ┐
//	..o@@o..      ← 行 1  ┘ block0（头部 + 背景）
//	.######.      ← 行 2  ┐
//	##@@@@##      ← 行 3  ┘ block1（上半身：身体 # + 肚子 @）
//	.######.      ← 行 4  ┐
//	##@@@@##      ← 行 5  ┘ block2（== block1：刻意让两块明文完全相同）
//	..####..      ← 行 6  ┐
//	...::...      ← 行 7  ┘ block3（脚 + 地面）
//
// 关键：block1 与 block2 的明文字节完全一致（行 2-3 == 行 4-5）。
// 因此 ECB 加密后这两个密文块也完全一致（"身体"轮廓清晰保留）；
// 而 CBC 因 IV/链式 XOR 会把这种重复彻底打散。

// penguinImage 是 8x8 的灰度文本图（每个字符必须为 ASCII，映射成 1 字节）。
// 字符越靠后视觉上越"亮"（. < : < o < # < @），模拟灰度深浅。
var penguinImage = [8]string{
	"........",
	"..o@@o..",
	".######.",
	"##@@@@##",
	".######.",
	"##@@@@##",
	"..####..",
	"...::...",
}

// PenguinKey 是演示用的固定 AES-128 密钥（16 字节）。教学确定性，请勿在生产环境硬编码。
var PenguinKey = []byte("0123456789abcdef") // 恰好 16 字节

// PenguinIV 是 CBC 演示用的固定初始向量（16 字节）。
var PenguinIV = []byte("abcdef9876543210") // 恰好 16 字节

// grayscaleChars 是把密文字节映射回可打印 ASCII 字符的字符表（用于"画"密文图）。
// 用前 95 个可打印 ASCII（0x20..0x7e）按字节值取模映射，保证输出始终可打印、
// 且对相近字节产生相近字符（视觉上能看出块的"纹理"是否重复）。
const printableBase = 0x20

// printableByte 把任意字节映射到一个可打印 ASCII 字符。
func printableByte(b byte) byte {
	return printableBase + b%(0x7f-printableBase)
}

// ImageToBytes 把 8x8 rune 图展平成 64 字节（逐行、每字符取其 ASCII 码）。
// 要求每个字符码点 < 128（ASCII），否则报错——AES 分组是字节，多字节 rune 会被
// 拆散破坏"一个像素 = 一字节"的对应关系，使演示失真。
func ImageToBytes(img [8]string) ([]byte, error) {
	out := make([]byte, 0, 64)
	for r, row := range img {
		if len(row) != 8 {
			return nil, fmt.Errorf("penguin: 第 %d 行长度 %d 非 8（图像必须是 8x8）", r, len(row))
		}
		for _, c := range row {
			if c >= 128 {
				return nil, fmt.Errorf("penguin: 字符 %q 非 ASCII（码点 %d），图像必须用 ASCII 灰度字符", c, c)
			}
			out = append(out, byte(c))
		}
	}
	return out, nil
}

// BytesToImage 把 64 字节重新画成 8x8 的可打印字符图（每个字节 → 一个 ASCII 像素）。
// 仅用于密文的"可视化"：把不可打印的密文字节映射成可打印字符，方便人眼对比块的重复。
// 输入长度不要求恰好 64；按 8 字节一行分行，不足则相应短行。
func BytesToImage(data []byte) [8]string {
	var img [8]string
	for r := 0; r < 8; r++ {
		var row []byte
		start := r * 8
		end := start + 8
		if end > len(data) {
			end = len(data)
		}
		if start > len(data) {
			break
		}
		for _, b := range data[start:end] {
			row = append(row, printableByte(b))
		}
		img[r] = string(row)
	}
	return img
}

// EncryptImageECB 用 ECB 模式加密整张 8x8 图像，返回 4 个密文块（每块 16 字节）。
func EncryptImageECB(img [8]string, key []byte) ([][]byte, error) {
	plain, err := ImageToBytes(img)
	if err != nil {
		return nil, err
	}
	ct, err := EncryptECB(plain, key)
	if err != nil {
		return nil, err
	}
	return splitBlocks(ct), nil
}

// EncryptImageCBC 用 CBC 模式加密整张 8x8 图像，返回 4 个密文块（每块 16 字节）。
func EncryptImageCBC(img [8]string, key, iv []byte) ([][]byte, error) {
	plain, err := ImageToBytes(img)
	if err != nil {
		return nil, err
	}
	ct, err := EncryptCBC(plain, key, iv)
	if err != nil {
		return nil, err
	}
	return splitBlocks(ct), nil
}

// splitBlocks 把密文按 16 字节切片成若干块（ECB/CBC 演示统一用）。
func splitBlocks(ct []byte) [][]byte {
	var blocks [][]byte
	for i := 0; i < len(ct); i += blockSize {
		end := i + blockSize
		if end > len(ct) {
			end = len(ct)
		}
		blk := make([]byte, end-i)
		copy(blk, ct[i:end])
		blocks = append(blocks, blk)
	}
	return blocks
}

// HasDuplicateBlock 检查密文块序列里是否存在至少一对完全相同的块。
// ECB 企鹅图的核心证据：相同的明文块产生相同的密文块 → 重复块出现 → 轮廓泄露。
func HasDuplicateBlock(blocks [][]byte) bool {
	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			if bytes.Equal(blocks[i], blocks[j]) {
				return true
			}
		}
	}
	return false
}

// CountDuplicatePairs 返回密文块序列中相等块对的数量（用于断言 ECB>CBC 的重复程度）。
func CountDuplicatePairs(blocks [][]byte) int {
	n := 0
	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			if bytes.Equal(blocks[i], blocks[j]) {
				n++
			}
		}
	}
	return n
}

// RenderImage 把 8x8 图像渲染成多行字符串（带行框），便于打印对比。
func RenderImage(img [8]string) string {
	var sb []byte
	sb = append(sb, "+--------+\n"...)
	for _, row := range img {
		sb = append(sb, '|')
		sb = append(sb, row...)
		sb = append(sb, '|', '\n')
	}
	sb = append(sb, "+--------+\n"...)
	return string(sb)
}

// PenguinDemo 是企鹅图演示的完整结果，供测试与展示工具消费。
type PenguinDemo struct {
	PlaintextImage    [8]string // 原始明文图（灰度字符）
	ECBCipherImage    [8]string // ECB 密文重新画成的图（应能看到重复"身体"块）
	CBCCipherImage    [8]string // CBC 密文重新画成的图（应是均匀噪声）
	ECBBlocks         [][]byte  // ECB 密文的 4 个 16 字节块
	CBCBlocks         [][]byte  // CBC 密文的 4 个 16 字节块
	ECBDuplicatePairs int       // ECB 相等块对数（应 >= 1，证明模式泄露）
	CBCDuplicatePairs int       // CBC 相等块对数（应为 0）
	ECBDuplicates     bool      // ECB 是否存在重复块
	CBCDuplicates     bool      // CBC 是否存在重复块
}

// RunPenguinDemo 跑一遍完整的 ECB 企鹅图对比演示，打印过程并返回结构化结果。
//
// 教学结论：
//   - ECB：明文里两块相同（行 2-3 == 行 4-5）→ 密文里这两块也相同，
//     重新画成图后能看到清晰重复的"身体"条带 → 轮廓/结构泄露。
//   - CBC：即便明文块相同，IV 与链式 XOR 使每块密文都不同 → 密文图呈均匀噪声。
func RunPenguinDemo() (*PenguinDemo, error) {
	plain, err := ImageToBytes(penguinImage)
	if err != nil {
		return nil, err
	}

	ecbBlocks, err := EncryptImageECB(penguinImage, PenguinKey)
	if err != nil {
		return nil, err
	}
	cbcBlocks, err := EncryptImageCBC(penguinImage, PenguinKey, PenguinIV)
	if err != nil {
		return nil, err
	}

	// 把密文（拼接后）重新画成 8x8 图，便于直观对比"轮廓是否还在"。
	ecbFlat := bytes.Join(ecbBlocks, nil)
	cbcFlat := bytes.Join(cbcBlocks, nil)

	res := &PenguinDemo{
		PlaintextImage:    penguinImage,
		ECBCipherImage:    BytesToImage(ecbFlat),
		CBCCipherImage:    BytesToImage(cbcFlat),
		ECBBlocks:         ecbBlocks,
		CBCBlocks:         cbcBlocks,
		ECBDuplicatePairs: CountDuplicatePairs(ecbBlocks),
		CBCDuplicatePairs: CountDuplicatePairs(cbcBlocks),
	}
	res.ECBDuplicates = res.ECBDuplicatePairs > 0
	res.CBCDuplicates = res.CBCDuplicatePairs > 0

	// ---- 打印演示 ----
	fmt.Println("=== AES-ECB 企鹅图（文本版）演示 ===")
	fmt.Println("说明：8x8 灰度图（. 最暗 → @ 最亮），刻意让中间两块明文完全相同。")
	fmt.Println("\n明文图像（企鹅）：")
	fmt.Print(RenderImage(penguinImage))
	fmt.Printf("明文 hex: %s\n\n", core.HexEncode(plain))

	fmt.Println("--- ECB 模式加密后的密文图 ---")
	fmt.Print(RenderImage(res.ECBCipherImage))
	fmt.Printf("ECB 密文 hex: %s\n", core.HexEncode(ecbFlat))
	fmt.Printf("ECB 相同块对数: %d  ← 相同明文块→相同密文块，'身体'轮廓清晰保留\n\n",
		res.ECBDuplicatePairs)

	fmt.Println("--- CBC 模式加密后的密文图 ---")
	fmt.Print(RenderImage(res.CBCCipherImage))
	fmt.Printf("CBC 密文 hex: %s\n", core.HexEncode(cbcFlat))
	fmt.Printf("CBC 相同块对数: %d  ← 链式 XOR 打散重复，轮廓消失成噪声\n",
		res.CBCDuplicatePairs)

	return res, nil
}
