# 一次性密码本 (One-Time Pad) · 设计笔记

## 历史

- 1882 年 Frank Miller 最先描述 OTP（用于电报密码本）
- 1917 年 **Gilbert Vernam** 与 Joseph Mauborgne 发明"Vernam 密码"——
  用穿孔纸带把与明文等长的真随机密钥与明文 XOR，被认为是 OTP 的工程实现
- 1949 年 **Claude Shannon** 在《Communication Theory of Secrecy Systems》中
  形式化证明 OTP **绝对安全（perfect secrecy）**——这是密码学史上第一个、
  也是最著名的信息论安全证明
- 冷战时期"红色电话"（华盛顿-莫斯科直通热线）真实使用过 OTP 加密

## 核心算法

```
加密：C[i] = P[i] XOR K[i]
解密：P[i] = C[i] XOR K[i]      （与加密完全相同的运算）
        ↑ 依赖 XOR 自反性：(a XOR b) XOR b = a
```

数学上和 XOR 密码完全一样，区别全在**密钥的约束**。

## 判定红线（满足全部三条才是 OTP）

1. **密钥必须真随机**——用 `crypto/rand`（操作系统 CSPRNG），不是 `math/rand`
   （伪随机）。伪随机密钥 → 密钥可预测 → 退化为可破的流密码。
2. **密钥长度 = 明文长度**——逐字节一一对应，**不循环、不截断**。
   `len(K) != len(P)` 直接报错（本包 `ErrKeyLengthMismatch`）。
3. **密钥只用一次**——绝不复用。两次用同一密钥加密即 **two-time pad**，
   `C1 XOR C2 = P1 XOR P2`，明文的统计结构立刻暴露（这正是把 XOR 密码
   “循环复用”打成筛子的同一种攻击）。

> 缺任一条 → 不是 OTP，只是普通的、可破的 XOR 密码（见 `internal/xor`）。

## Shannon 绝对安全证明（直觉版）

OTP 满足"完美保密"（perfect secrecy）的定义：

> 对于任意明文 P 和任意密文 C，`Pr[E(P) = C]` 与 P 无关。

直觉：因为密钥 K 在所有 `2^n` 种可能上**均匀随机**，对任意给定的密文 C 和
任意明文 P'，都**恰好存在一个** K' = P' XOR C 使得 `E_{K'}(P') = C`。所以
"看到密文 C"对推断明文毫无帮助——每个明文都同样可能，攻击者的后验知识
等于先验知识。**无论算力多强、即使 P=NP，都无法破解 OTP。**

形式化：`I(P; C) = 0`（明文与密文的互信息为零）。

## 为什么 OTP 实际不实用

- **密钥分发难题**：要安全传 1 GB 明文，得先安全传 1 GB 密钥。能安全传
  1 GB 密钥的话，直接传明文即可——鸡生蛋问题。
- **密钥管理开销**：每条消息都得新密钥，且用后即弃。1 万条消息要 1 万份
  一次性密钥，存储/销毁成本爆炸。
- **任何复用都是灾难**：人难免犯错，密钥一旦复用（哪怕只重叠几字节），
  整段密文在该区间立刻可破。
- 因此现实里几乎不用 OTP。替代品是 **计算安全**的方案：
  - **AES-GCM / ChaCha20-Poly1305**：短种子（128/256 位）+ PRNG 扩展出与
    明文等长的伪随机密钥流。安全性依赖"PRNG 不可逆"（计算假设，非信息论），
    但密钥短、可重复用（每次用 nonce 区分），实用得多。

## 与 XOR 密码的区别（关键对比）

| 维度 | 本包 OTP | `internal/xor`（循环 XOR 密码） |
|------|----------|--------------------------------|
| 密钥来源 | crypto/rand 真随机 | 调用方提供任意短密钥 |
| 密钥长度 | 严格 = 明文长度 | 通常 < 明文，循环复用 |
| 密钥复用 | 绝不（one-time） | 必然复用（循环） |
| 安全性 | 信息论绝对安全 | 几乎无（已知明文攻击即破） |
| 实用性 | 仅教学 / 极端场景 | 仅教学（反面教材） |

两者代码几乎一样（都是 `P XOR K`），差距完全来自**密钥的纪律**。
这就是密码学的一条核心教训：算法本身往往不是短板，密钥管理才是。

## 最小可识别特征

1. **逐字节 XOR**（与 XOR 密码相同）
2. **密钥与明文等长**（不循环）—— 这是与循环 XOR 密码的分水岭
3. **密钥真随机 + 一次性**（用 crypto/rand，每次重新生成）

## 本包实现要点

- `Encrypt` 用 `crypto/rand.Reader` 生成与明文等长的真随机密钥
- `EncryptWithKey` 强校验 `len(key) == len(plaintext)`，不等即报错
- 解密 == 加密（XOR 自反），复用 `core.XorBytes`
- 零外部依赖：`crypto/rand` 是标准库

## 参考

- Shannon, C. E. (1949). "Communication Theory of Secrecy Systems"
- Wikipedia: One-time pad / Perfect secrecy
- Bruce Schneier "Applied Cryptography"（信息论安全章节）
- Vernam, G. S. (1926). "Cipher Printing Telegraph Systems"
- 现实替代：RFC 8439 (ChaCha20-Poly1305)、NIST SP 800-38D (AES-GCM)
