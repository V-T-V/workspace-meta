// Package rsa 的 demo 入口。
//
// Demo(ctx) 用固定教材参数（p=61, q=53）展示 RSA 完整流程：
// 生成密钥 → 加密 'A'(65) → 解密还原 → 签名 → 验签。
// 实现见 rsa.go，本文件仅为"5 件套"结构保留独立文件注释入口。
//
// 运行：
//
//	go run ./cmd/atlas -d rsa
package rsa
