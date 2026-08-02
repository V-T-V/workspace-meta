// Command atlas 是 crypto-atlas 的统一 demo 入口。
//
// 用法：
//
//	crypto-atlas -d caesar      # 凯撒密码
//	crypto-atlas -d vigenere    # 维吉尼亚密码
//	crypto-atlas -d xor         # XOR 密码
//	crypto-atlas -d aes         # AES 对称加密（ECB vs CBC）
//	crypto-atlas -d des         # DES 对称加密
//	crypto-atlas -d sha256      # SHA-256 哈希
//	crypto-atlas -d md5         # MD5 哈希
//	crypto-atlas -d rsa         # RSA 公钥密码
//	crypto-atlas -d dh          # Diffie-Hellman 密钥交换
//	crypto-atlas -d otp         # 一次性密码本（信息论绝对安全）
//	crypto-atlas -d tlssim      # TLS 1.2 简化握手模拟
//	crypto-atlas -d all         # 依次运行全部 demo
//	crypto-atlas -version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/QiuShichang/crypto-atlas/internal/aes"
	"github.com/QiuShichang/crypto-atlas/internal/caesar"
	"github.com/QiuShichang/crypto-atlas/internal/des"
	"github.com/QiuShichang/crypto-atlas/internal/dh"
	"github.com/QiuShichang/crypto-atlas/internal/hmac"
	"github.com/QiuShichang/crypto-atlas/internal/md5"
	"github.com/QiuShichang/crypto-atlas/internal/otp"
	"github.com/QiuShichang/crypto-atlas/internal/rsa"
	"github.com/QiuShichang/crypto-atlas/internal/sha256"
	"github.com/QiuShichang/crypto-atlas/internal/tlssim"
	"github.com/QiuShichang/crypto-atlas/internal/vigenere"
	"github.com/QiuShichang/crypto-atlas/internal/xor"
)

var version = "dev"

func main() {
	var (
		demo    string
		showVer bool
	)
	flag.StringVar(&demo, "d", "caesar", "demo: caesar|vigenere|xor|aes|des|sha256|md5|rsa|dh|otp|tlssim|all")
	flag.BoolVar(&showVer, "version", false, "打印版本号")
	flag.Parse()

	if showVer {
		fmt.Println("crypto-atlas", version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, demo); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, demo string) error {
	if demo == "all" {
		all := []string{"caesar", "vigenere", "xor", "aes", "des", "sha256", "md5", "rsa", "dh", "otp", "hmac", "tlssim"}
		for _, d := range all {
			fmt.Printf("\n========== ▶ %s ==========\n", d)
			if err := run(ctx, d); err != nil {
				return err
			}
		}
		return nil
	}
	switch demo {
	case "caesar":
		_, err := caesar.Demo(ctx)
		return err
	case "vigenere":
		_, err := vigenere.Demo(ctx)
		return err
	case "xor":
		_, err := xor.Demo(ctx)
		return err
	case "aes":
		_, err := aes.Demo(ctx)
		return err
	case "des":
		_, err := des.Demo(ctx)
		return err
	case "sha256":
		_, err := sha256.Demo(ctx)
		return err
	case "md5":
		_, err := md5.Demo(ctx)
		return err
	case "rsa":
		_, err := rsa.Demo(ctx)
		return err
	case "dh":
		_, err := dh.Demo(ctx)
		return err
	case "otp":
		_, err := otp.Demo(ctx)
		return err
	case "hmac":
		_, err := hmac.Demo(ctx)
		return err
	case "tlssim":
		_, err := tlssim.Demo(ctx)
		return err
	default:
		return fmt.Errorf("未知 demo: %s（可选: caesar|vigenere|xor|aes|des|sha256|md5|rsa|dh|otp|tlssim|all）", demo)
	}
}
