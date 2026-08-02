# crypto-atlas

> 零依赖密码学教学库 —— 从古典密码（凯撒/维吉尼亚）到现代密码（AES/RSA/DH/SHA/HMAC），10 类算法，用 Go 纯标准库实现，每种配离线 demo + 确定性单测 + 历史/原理笔记。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![零依赖](https://img.shields.io/badge/dependencies-0-success)
![algorithms](https://img.shields.io/badge/algorithms-10-orange)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区在安全/密码学象限是**零覆盖**——go-rmm 的 mTLS 是功能不是教学，frontend-toolbox 的加密是工具不是学。`crypto-atlas` 用最小可读的代码补上这一环，让"凯撒为什么会破""AES 为什么安全""RSA 数学原理是什么"这些问题有可运行的答案。

## 算法家族

| 家族 | 包 | 算法 | 论文/历史 |
|------|-----|------|-----------|
| **古典密码** | `caesar` | 凯撒密码（位移替换） | 公元前 50 年 |
| | `vigenere` | 维吉尼亚密码（多表替换） | Bellaso 1553 |
| | `xor` | XOR 密码（流密码鼻祖） | Vernam 1917 |
| **对称加密** | `aes` | AES-128/192/256（ECB vs CBC） | Rijndael 2001 NIST |
| | `des` | DES（教学对比） | IBM 1977 NBS |
| **哈希** | `sha256` | SHA-256 | NSA 2001 |
| | `md5` | MD5（已不安全，对比用） | Rivest 1991 |
| **消息认证** | `hmac` | HMAC-SHA-256（带密钥的哈希） | Bellare-Canetti-Krawczyk 1996 RFC 2104 |
| **公钥密码** | `rsa` | RSA（手写小素数版） | Rivest-Shamir-Adleman 1977 |
| | `dh` | Diffie-Hellman 密钥交换 | Diffie-Hellman 1976 |

## 快速开始

```bash
cd crypto-atlas

# 单个 demo
go run ./cmd/atlas -d caesar      # 凯撒密码：Hello → Khoor
go run ./cmd/atlas -d aes         # AES：ECB vs CBC（为什么 ECB 不安全）
go run ./cmd/atlas -d rsa         # RSA：手写小素数加解密
go run ./cmd/atlas -d dh          # DH：双方算出相同共享密钥
go run ./cmd/atlas -d hmac        # HMAC：带密钥的哈希 + 篡改检测

# 全部 demo
go run ./cmd/atlas -d all

# 全部测试
make test
```

所有 demo **离线可跑**，零外部依赖。

## 学习路径

按难度递增：

1. **caesar**（最易）—— 理解"密钥/加密/解密/替换"
2. **vigenere**（易）—— 理解"多表替换"，抗频率分析
3. **xor**（易）—— 理解对称加密的数学基础（XOR 自反）
4. **sha256 / md5**（中）—— 理解哈希（单向/雪崩/定长）+ 为什么 MD5 不安全
5. **hmac**（中）—— 理解"带密钥的哈希"（完整性 + 真实性）+ 长度扩展攻击
6. **aes**（中难）—— 理解分组密码 + ECB vs CBC 模式
7. **des**（中）—— 理解历史算法 + 为什么被 AES 取代
8. **rsa**（难）—— 理解公钥密码的数学原理（大数分解）
9. **dh**（难）—— 理解密钥交换（离散对数）

## 核心设计

```
                 ┌─────────────────────────────┐
                 │   密码学核心概念             │
                 │   密钥 / 加密 / 解密 / 哈希  │
                 └──────────────┬──────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
  ┌───────────┐           ┌───────────┐           ┌───────────┐
  │ 古典密码   │           │ 对称加密   │           │ 公钥密码   │
  │ caesar    │           │ aes/des   │           │ rsa/dh    │
  │ vigenere  │           │ (共享密钥)│           │ (公/私钥) │
  │ xor       │           └───────────┘           └───────────┘
  └───────────┘
                 ┌───────────┐
                 │ 哈希       │  ← 无密钥、单向
                 │ sha256/md5│
                 └───────────┘
                 ┌───────────┐
                 │ 消息认证   │  ← 带密钥的哈希（完整性+真实性）
                 │ hmac      │
                 └───────────┘
```

### 单个算法的文件结构（5 件套）

```
internal/caesar/
├── caesar.go        # 实现（Encrypt/Decrypt）
├── caesar_test.go   # 表驱动测试（往返/边界/确定性）
├── demo.go          # Demo(ctx) 离线可跑
├── doc.go           # Go package doc
└── NOTES.md         # 历史 + 算法 + 判定红线 + 安全性 + 参考
```

## 设计原则

1. **零外部依赖是灵魂约束** —— go.mod 无 require，纯标准库（含 crypto/aes, crypto/sha256, math/big）。
2. **教学清晰** —— AES/SHA 用标准库展示"怎么用"，RSA/DH 手写展示"数学原理"。
3. **5 件套齐全** —— 每个算法含 impl + test + demo + doc.go + NOTES.md。
4. **确定性 demo** —— 用固定参数（如 RSA p=61/q=53），可复现。
5. **判定红线** —— 每个 NOTES.md 写明"少了什么就不算该算法"。

## 目录结构

```
crypto-atlas/
├── go.mod / Makefile / LICENSE
├── README.md / AGENTS.md
├── cmd/atlas/main.go        # -d <name> 统一入口
├── internal/
│   ├── core/                # 共享：HexEncode/XorBytes/PKCS7Pad
│   ├── caesar/              # 5 件套
│   ├── vigenere/            # 5 件套
│   ├── xor/                 # 5 件套
│   ├── aes/                 # 5 件套
│   ├── des/                 # 5 件套
│   ├── sha256/              # 5 件套
│   ├── md5/                 # 5 件套
│   ├── hmac/                # 5 件套
│   ├── rsa/                 # 5 件套
│   └── dh/                  # 5 件套
└── docs/
    └── RESEARCH_SUMMARY.md  # 密码学全景 + 选型指南
```

## 路线图

- **M1（当前）**：10 算法（3 古典 + 2 对称 + 2 哈希 + 1 消息认证 + 2 公钥）5 件套
- **M2 候选**：3DES / ChaCha20 / ECDSA / 数字签名验证 / TLS 握手模拟

## 不做的

- 生产级安全（教学用小参数，RSA 用 61/53 不安全；生产用 crypto/rsa 标准库）
- 完整 TLS 实现（那是 Go 标准库 crypto/tls 的领域）

## 相关项目

- [`consensus-atlas`](../consensus-atlas) / [`go-agent-research`](../go-agent-research) —— 同范式的零依赖教学库
