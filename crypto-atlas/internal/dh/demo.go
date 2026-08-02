// Package dh 的 demo 入口。
//
// Demo(ctx) 用经典教材参数（p=23, g=5）演示 Diffie-Hellman 完整流程：
// Alice(a=6) 与 Bob(b=15) 在公开信道上交换公钥，各自算出相同的共享密钥=2。
// 实现见 dh.go，本文件仅为"5 件套"结构保留独立文件注释入口。
//
// 运行：
//
//	go run ./cmd/atlas -d dh
package dh
