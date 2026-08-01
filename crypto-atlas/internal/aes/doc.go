// Package aes 的更多背景（AES 历史、ECB/CBC/GCM 模式对比、ECB 企鹅图）见 NOTES.md。
//
// 本包聚焦三件事：
//  1. 用标准库 crypto/aes 演示 AES 是什么、怎么用（教学，不手写轮函数）；
//  2. 对比 ECB 与 CBC 两种工作模式，直观展示 ECB 的模式泄露缺陷；
//  3. 校验密钥长度（16/24/32 → AES-128/192/256），用 PKCS#7 处理填充。
//
// 注意：ECB 仅为教学，生产环境请用 CBC（带随机 IV）或带认证的 GCM（crypto/cipher.NewGCM）。
package aes
