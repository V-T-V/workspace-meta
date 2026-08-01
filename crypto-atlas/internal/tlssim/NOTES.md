# TLS 1.2 握手模拟 · 设计笔记

## 这个包是什么 / 不是什么

`tlssim` 把 TLS 1.2 握手拆成 **5 个清晰阶段**做教学演示，复用本仓库已有的
`rsa` / `dh` / `aes` / `hmac` 四个原语包。它回答一个问题：

> 读完 RSA、DH、AES、HMAC 四个孤立的算法后——**它们如何拼成一个真实协议？**

它**不是**真实 TLS：

- 没有网络（`Handshake()` 是一个同步函数调用）
- 没有 X.509 证书链 / CA 校验（`Certificate` 只是 `{Subject, PublicKey}` 二元组）
- 没有真实 PRF（用单轮 `SHA256` 代替 HMAC-based PRF 多轮扩展）
- 没有 Finished 消息 / 握手摘要验证
- 没有 AEAD（用"分离的 AES + HMAC"代替 GCM/ChaCha20-Poly1305）
- 教材级小参数（RSA N=3233、DH p=23），密码学上完全不安全

## 5 阶段流程

```
客户端                                           服务端
──────                                           ──────
[1] 生成 ClientRandom ──ClientHello──→
                                                 [2] 生成 ServerRandom
                                                    生成临时 DH 密钥对 (b, B)
                                                    出示证书（RSA 公钥 N,E）
                          ←──ServerHello────────
                              + 证书 + B
[3] 验证证书（信任绑定）
    生成临时 DH 密钥对 (a, A)
    算 preMaster = B^a mod p        ──A────────→  算 preMaster = A^b mod p
    （= g^(ab) mod p，两侧必然相等）

[4] 双方各自：master = SHA256(preMaster || ClientRandom || ServerRandom)
    AESKey = master[0:16]   （AES-128）
    HMACKey = master[16:48] （HMAC-SHA256）

[5] record layer：AES-CBC 加密 + HMAC-SHA256 认证应用数据
```

## 为什么用 DHE-RSA？

对应 TLS 1.2 的 `TLS_DHE_RSA_WITH_*` 密码套件：

- **DHE（Ephemeral DH）**：每次握手生成**临时** DH 私钥，握手后丢弃 → **前向保密**
  （Forward Secrecy）。即使服务端 RSA 私钥日后泄露，也无法解密历史会话——
  因为 preMaster 只存在于临时 DH 中，从未被 RSA 加密传输。
- **RSA**：用于证书签名 / 身份认证（防止中间人把 DH 公开值 B 换成自己的 B'）。
  注意：RSA **不**加密 preMaster（那是静态 RSA 密钥交换 `TLS_RSA_WITH_*`，无前向保密）。

## 密钥派生：为什么不能直接用 DH 共享密钥？

DH 教材参数 p=23, g=5, a=6, b=15 → 共享密钥 = **2**（单字节 `0x02`）。

- AES-128 需要 **16 字节**密钥，HMAC-SHA256 需要 **32 字节**。
- 直接取"前 16/32 字节"对 `0x02` 来说是退化（全是 0/重复）。
- 真实 TLS 用 **PRF**（HMAC-based，多轮扩展）把 preMaster 扩展成任意长度的
  key block（master secret + 各方向 AES/MAC 密钥 + IV）。

本包简化为**双块 SHA256 展开**（类 PRF，输出 64 字节）：

```
block1 = SHA256(preMaster || ClientRandom || ServerRandom || 0x01)  // 32 字节
block2 = SHA256(preMaster || ClientRandom || ServerRandom || 0x02)  // 32 字节
master = block1 || block2                                            // 64 字节
AESKey  = master[0:16]
HMACKey = master[16:48]
```

这是任务要求的"简化：直接取前 16/32 字节"——只是先展开成 64 字节让短输入
（preMaster=2）扩展成满熵密钥材料，再切片。ClientRandom/ServerRandom 一起喂入，
保证**不同会话派生不同密钥**（防重放/防预测），这正是两个随机数的作用。
真实 TLS 的 PRF 是 `HMAC` 嵌套（P_hash），更规范但思想一致：把短秘密扩展成长 key block。

## Encrypt-then-MAC vs MAC-then-Encrypt

本包 record layer 用 **Encrypt-then-MAC**（先 AES 加密，再对密文算 HMAC）：

```go
ct  = AES-CBC-Encrypt(plaintext)
mac = HMAC-SHA256(HMACKey, ct)        // MAC 覆盖密文
```

接收方先验 MAC，再解密。优点：

- **快速失败**：密文被篡改时，MAC 校验直接拒绝，**不会触发解密**
  （MAC-then-Encrypt 会让攻击者通过操纵密文触发 padding oracle）。
- 这是 RFC 7366 定义的可选扩展，也是 TLS 1.3（AEAD）的天然形态。

TLS 1.2 默认是 **MAC-then-Encrypt**（`HMAC(plaintext)`，先算 MAC 再整体加密），
历史上导致了 **Lucky13**（CBC + HMAC 时序）等攻击。本教学包为清晰展示
"加密 + 认证两个独立步骤"，故意选 Encrypt-then-MAC。

## 安全局限（必读）

1. **教材参数完全不可靠**：RSA N=3233 一秒可分解；DH p=23 离散对数可手算。
   真实 TLS 用 2048+/3072+ 位 RSA、≥2048 位 DH 群（FFDHE）或 256 位 ECDH 曲线。
2. **无证书链校验** → 无法防中间人（MITM）。真实 TLS 靠 CA 信任链 + 主机名校验。
3. **无 Finished 验证** → 握手本身可被注入篡改而不被发现。
   真实 TLS 在握手末尾用派生密钥对**全部握手消息的摘要**做 HMAC，双方互验。
4. **IV 复用**：本包 IV 取自 ClientRandom（demo 简化）。真实 CBC 必须每条 record
   用独立不可预测 IV，否则泄露明文相关性。TLS 1.3 直接废弃 CBC，强制 AEAD。

## TLS 演进

| 版本  | 关键变化 |
|-------|----------|
| SSL 3.0 (1996) | 首版，已废弃（POODLE 攻击） |
| TLS 1.0 (1999) | 修 SSL3 漏洞，仍 CBC |
| TLS 1.1 (2006) | 显式 IV（堵 BEAST） |
| TLS 1.2 (2008) | AEAD（GCM）可选，SHA-256；本包模拟对象 |
| TLS 1.3 (2018) | 1-RTT 握手、强制 AEAD、移除静态 RSA/CBC/MD5/SHA1、支持 0-RTT |

## 与仓库其他包的关系

```
tlssim ──┬── rsa   （证书公钥；身份认证原语）
         ├── dh    （preMaster 协商；前向保密核心）
         ├── aes   （record layer 加密）
         └── hmac  （record layer 认证 / 篡改检测）
```

读到这里，你应该能回答：**为什么 TLS 需要 4 种原语而不是 1 种？**
因为安全通信需要同时满足 4 个独立目标：

1. **机密性**（只有对方能读）→ AES
2. **完整性**（未被篡改）→ HMAC
3. **密钥分发**（如何安全共享对称密钥）→ DH
4. **身份认证**（确认对方是其所声称的）→ RSA 证书

缺任何一项都会留下可利用的攻击面——这正是每个算法单独存在的理由。
