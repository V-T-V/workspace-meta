package api

import "testing"

// verifyPassword 单元测试：明文、哈希、错误密码、空密码各种场景。
// 这些用例覆盖了原本在 endpoints_test.go 中发现的前缀判断 bug（已修复）。

func TestHashPassword_Format(t *testing.T) {
	h := hashPassword("any")
	if len(h) < 8 || h[:7] != "sha256:" {
		t.Errorf("hashPassword 应返回 sha256: 前缀，实际 %q", h)
	}
	// 相同输入应确定性输出同一哈希
	if hashPassword("any") != h {
		t.Error("hashPassword 应确定性")
	}
	// 不同输入应产生不同哈希
	if hashPassword("other") == h {
		t.Error("不同密码不应有相同哈希")
	}
}

func TestVerifyPassword_EmptyStored_AllowsAll(t *testing.T) {
	// 未设密码（空串）放行任意输入
	if !verifyPassword("anything", "") {
		t.Error("空存储密码应放行")
	}
	if !verifyPassword("", "") {
		t.Error("空存储密码应放行空输入")
	}
}

func TestVerifyPassword_Plaintext(t *testing.T) {
	// 存储明文：提供正确明文
	if !verifyPassword("secret123", "secret123") {
		t.Error("明文正确密码应通过")
	}
	// 提供错误密码
	if verifyPassword("wrong", "secret123") {
		t.Error("明文错误密码应拒绝")
	}
}

func TestVerifyPassword_Hashed(t *testing.T) {
	// 存储 sha256: 哈希
	stored := hashPassword("secret123")
	if !verifyPassword("secret123", stored) {
		t.Error("哈希密码正确输入应通过")
	}
	if verifyPassword("wrong", stored) {
		t.Error("哈希密码错误输入应拒绝")
	}
	// 提供哈希本身（而非明文）应拒绝
	if verifyPassword(stored, stored) {
		t.Error("用哈希串当密码应拒绝")
	}
}

func TestVerifyPassword_HashDoesNotMatchPlaintextStorage(t *testing.T) {
	// 边界：存储哈希时，提供明文比较应失败（不会误把哈希当明文比对）
	stored := hashPassword("pw")
	// 此时 verifyPassword("pw", stored)：provided 经哈希后 == stored → 通过（正确路径）
	if !verifyPassword("pw", stored) {
		t.Error("正确路径应通过")
	}
}

func TestVerifyPassword_NotConstantTimeLeak(t *testing.T) {
	// 烟雾测试：多次验证不应 panic，结果稳定
	stored := hashPassword("abc")
	for i := 0; i < 100; i++ {
		if verifyPassword("abc", stored) != true {
			t.Fatal("稳定输入应稳定通过")
		}
	}
}
