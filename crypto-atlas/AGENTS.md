# crypto-atlas · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库实现的**密码学教学库**——10 类经典密码算法（古典：凯撒/维吉尼亚/XOR；对称：AES/DES；哈希：SHA-256/MD5；消息认证：HMAC；公钥：RSA/DH），每种配可运行 demo + 确定性单测 + 历史原理笔记（NOTES.md）。

```
                 ┌─────────────────────────────┐
                 │   密码学核心概念             │
                 └──────────────┬──────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
  ┌───────────┐           ┌───────────┐           ┌───────────┐
  │ 古典密码   │           │ 对称加密   │           │ 公钥密码   │
  │ caesar    │           │ aes/des   │           │ rsa/dh    │
  │ vigenere  │           └───────────┘           └───────────┘
  │ xor       │
  └───────────┘
                 ┌───────────┐
                 │ 哈希       │
                 │ sha256/md5│
                 └───────────┘
                 ┌───────────┐
                 │ 消息认证   │
                 │ hmac      │
                 └───────────┘
```

**做**：10 算法的实现 + demo + 测试 + 笔记；AES/SHA 用标准库展示用法，RSA/DH 手写展示数学原理，HMAC 展示带密钥哈希。
**不做**：生产级安全（教学用小参数）、完整 TLS 实现、后量子密码。

## 目标（Goal）

- **G1**：10 类算法每类都有"少了就不算该算法"的最小可识别实现（判定红线见各 NOTES.md）。
- **G2**：所有 demo 离线可跑、确定性（RSA 用 p=61/q=53，DH 用 p=23/g=5 等固定参数）。
- **G3**：10 算法建在同一 core 底座上（HexEncode/XorBytes/PKCS7Pad）。
- **G4**：每个算法包 5 件套齐全。
- **成功标准**：`go test ./...` 全绿 + 全 demo 离线跑通 + 每算法 NOTES.md。

## 当前情况（Status）

- **完成度**：**M1 完成**
- **底座**（`internal/core`）：HexEncode/HexDecode/XorBytes/PKCS7Pad/PKCS7Unpad
- **古典密码**：caesar / vigenere / xor（5 件套）
- **对称加密**：aes（ECB+CBC）/ des（5 件套）
- **哈希**：sha256 / md5（5 件套）
- **消息认证**：hmac（HMAC-SHA-256，RFC 2104，5 件套）
- **公钥密码**：rsa（手写小素数）/ dh（密钥交换）（5 件套）
- **测试**：11 包全绿（含 HMAC 与标准库 crypto/hmac 交叉验证）

## 技术栈与架构

- **语言**：Go 1.25.6
- **依赖**：**零外部依赖**（module 只引标准库：crypto/aes / crypto/des / crypto/sha256 / crypto/md5 / crypto/hmac（测试交叉验证用） / math/big）
- **设计参考**：consensus-atlas / go-agent-research（5 件套 + Demo 入口范式）、Applied Cryptography（Schneier）
- **目录**：cmd + internal，10 算法各自独立包，只共享 internal/core

## 如何运行

```bash
go run ./cmd/atlas -d caesar      # 凯撒
go run ./cmd/atlas -d aes         # AES（ECB vs CBC）
go run ./cmd/atlas -d rsa         # RSA 手写
go run ./cmd/atlas -d dh          # DH 密钥交换
go run ./cmd/atlas -d hmac        # HMAC（带密钥的哈希 + 篡改检测）
go run ./cmd/atlas -d all         # 全部 10 个 demo
make test                         # 全部测试
```

## 关键约定

- **零外部依赖是灵魂约束**：go.mod 无 require，纯标准库。
- **5 件套齐全**：每算法 impl + test + demo + doc.go + NOTES.md。
- **确定性 demo**：固定参数（RSA p=61/q=53，DH p=23/g=5），无真随机。
- **教学分层**：AES/SHA 用标准库（展示用法），RSA/DH 手写（展示数学原理）。
- **判定红线**：每个 NOTES.md 写明"少了什么就不算该算法"。

## 与其他项目的关系

- **与 [`consensus-atlas`](../consensus-atlas) / [`go-agent-research`](../go-agent-research) 同范式**：零依赖教学库，5 件套 + Demo 入口，本库的目录结构/文档风格全部对齐它们。
- **与 [`go-rmm`](../go-rmm)**：go-rmm 的 mTLS 是功能实现，本库是教学；理念同源（安全）但定位不同。
- **工作区定位**：补全工作区在"安全/密码学教学"象限的空白。
