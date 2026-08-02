// Package md5 的 demo 入口与可执行示例。
//
// 真正的 Demo(ctx) 实现见 md5.go；本文件独立出来以满足"5 件套"结构，
// 集中放置 demo 相关说明，避免与算法实现耦合。
//
// 运行方式（go run 时调用方负责 context）：
//
//	r, err := md5.Demo(context.Background())
package md5
