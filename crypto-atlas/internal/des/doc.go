// Package des 的更多背景（DES 历史、56 位密钥为何不安全、3DES、Feistel 网络）见 NOTES.md。
//
// 本包聚焦三件事：
//  1. 用标准库 crypto/des 演示 DES 这个历史算法"是什么、怎么用"；
//  2. 突出 DES 的 8 字节小块与 AES 16 字节块的对比；
//  3. 校验密钥长度（8 字节，有效 56 位），用 PKCS#7 处理填充。
//
// 注意：DES 已不安全（56 位密钥可被暴力穷举）。新代码请用 AES（internal/aes）。
package des
