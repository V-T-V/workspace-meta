# examples

crypto-atlas 的示例与用法入口。

## 快速演示

```bash
# 在项目根目录运行
go run ./cmd/atlas -d caesar      # 凯撒密码：Hello → Khoor
go run ./cmd/atlas -d vigenere    # 维吉尼亚：多表替换
go run ./cmd/atlas -d xor         # XOR 密码
go run ./cmd/atlas -d aes         # AES：ECB vs CBC（为什么 ECB 不安全）
go run ./cmd/atlas -d des         # DES：历史算法
go run ./cmd/atlas -d sha256      # SHA-256 哈希（雪崩效应）
go run ./cmd/atlas -d md5         # MD5（对比 SHA-256）
go run ./cmd/atlas -d hmac        # HMAC：带密钥的哈希 + 篡改检测
go run ./cmd/atlas -d rsa         # RSA：手写小素数加解密 + 签名
go run ./cmd/atlas -d dh          # DH：双方协商出相同密钥
go run ./cmd/atlas -d all         # 依次跑全部 10 个 demo
```

所有 demo **离线可跑**，零外部依赖。

## 作为库使用

### 凯撒密码
```go
package main

import (
	"fmt"
	"github.com/QiuShichang/crypto-atlas/internal/caesar"
)

func main() {
	ct := caesar.Encrypt("Hello, World!", 3)
	fmt.Println(ct) // Khoor, Zruog!
	pt := caesar.Decrypt(ct, 3)
	fmt.Println(pt) // Hello, World!
}
```

### SHA-256 哈希
```go
package main

import (
	"fmt"
	"github.com/QiuShichang/crypto-atlas/internal/sha256"
)

func main() {
	h := sha256.HashHex([]byte("hello"))
	fmt.Println(h) // 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
}
```

### HMAC-SHA-256（带密钥的哈希）
```go
package main

import (
	"fmt"
	"github.com/QiuShichang/crypto-atlas/internal/hmac"
)

func main() {
	key := []byte("secret")
	msg := []byte("transfer:alice→bob:100")
	mac := hmac.Compute(key, msg) // 返回 32 字节 MAC
	fmt.Printf("%x\n", mac)
	ok := hmac.Verify(key, msg, mac) // 恒定时间比较，防时序攻击
	fmt.Println(ok)                   // true
}
```

## 学习路径

按难度递增（也是密码学历史演进顺序）：

1. **caesar**（最易）—— 古老替换密码，引入加密/解密/密钥概念
2. **vigenere** —— 多表替换，抗频率分析
3. **xor** —— 对称加密的数学基础
4. **sha256 / md5** —— 哈希（单向函数）+ 为什么 MD5 不安全
5. **hmac** —— 带密钥的哈希（完整性 + 真实性）+ 长度扩展攻击
6. **aes** —— 现代对称加密标准 + ECB vs CBC 模式
7. **des** —— 历史对比（为什么 56 位不安全）
8. **rsa** —— 公钥密码的数学原理（大数分解）
9. **dh** —— 密钥交换（离散对数）

每个算法的 `NOTES.md` 有历史背景 + 核心算法 + 判定红线 + 安全性分析，建议对照阅读。
