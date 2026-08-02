// Package dh 手写实现 Diffie-Hellman 密钥交换（教学版，仅依赖 math/big + crypto/rand）。
//
// 更多背景（历史、离散对数、安全性）见 NOTES.md。
//
// Diffie-Hellman 是公钥密码的另一半：它不加密、不签名，只做一件事——
// 让素不相识的两方在被窃听的信道上协商出共享密钥。它是 TLS 前向保密、
// Signal 协议、IPsec 等现代密钥协商协议的数学基石。
package dh
