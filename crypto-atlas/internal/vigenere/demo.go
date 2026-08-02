// Package vigenere 的 demo 入口与可执行示例。
//
// 本文件仅为满足"5 件套"结构而独立出 demo 函数的入口注释；
// 真正的 Demo(ctx) 实现见 vigenere.go，避免与实现耦合。
//
// 运行方式（go run 时调用方负责 context）：
//
//	r, err := vigenere.Demo(context.Background())
package vigenere
