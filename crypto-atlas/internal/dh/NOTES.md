# Diffie-Hellman 密钥交换 · 设计笔记

## 历史

- 1976 年 **Whitfield Diffie & Martin Hellman** 发表 *New Directions in Cryptography*
- 首次提出"公钥密码"概念，被称为"密码学史上最具革命性的论文之一"
- 同时提出密钥交换协议：双方无需事先共享秘密即可协商出共享密钥
- 此前所有密码（凯撒、维吉尼亚、Enigma、乃至对称密码）都受困于"**密钥分发问题**"——
  如何把秘密安全送到对方手里？DH 给出了答案
- Ralph Merkle 同期独立提出类似思想（Merkle 谜题），故有时称 Diffie-Hellman-Merkle

## 核心算法

```
公共参数（任何人都可见）：
    p —— 大素数
    g —— 模 p 的生成元（原根）

Alice                              Bob
────────                           ────────
选随机 a ∈ [1, p-1]                 选随机 b ∈ [1, p-1]
（a 保密）                          （b 保密）
A = g^a mod p                      B = g^b mod p
        ── A 公开传输给 Bob ──→
        ←── B 公开传输给 Alice ──
算 s = B^a mod p                   算 s = A^b mod p
        = (g^b)^a = g^(ab) mod p        = (g^a)^b = g^(ba) mod p

由于乘法可交换 ab = ba，双方得到同一共享密钥 s = g^(ab) mod p。
```

### 为什么双方算出的值必然相等？

`(g^b)^a = g^(b·a) = g^(a·b) = (g^a)^b`，指数乘法可交换，故模 p 后结果一致。

## 安全性：离散对数难题（DLP）

窃听者能看到 `p, g, A=g^a, B=g^b`，要算共享密钥需先求出 `a` 或 `b`：

> 已知 g、p、A=g^a mod p，求 a —— 这就是**离散对数问题**。

- 在实数域，`log` 是容易的（O(1) 查表/迭代）
- 在模 p 的乘法群里，目前最好的通用算法（数域筛）仍需次指数时间 `exp(∛(log p))`
- 当 p 取足够大（≥2048 位）时，求解 DLP 在现实中不可行
- 因此窃听者算不出 a/b，就得不到 g^(ab)，密钥保密

**注意**：DH 的安全性"等价于"离散对数之难——计算上等价，但同样未被数学证明。

## 致命弱点：中间人攻击（MITM）

DH 本身**不认证身份**。攻击者 Mallory 可以：

```
Alice ←→ Mallory ←→ Bob
Alice 以为 A 发给 Bob，实际发给了 Mallory；
Mallory 自己生成一对密钥与 Bob 交换，充当"透明代理"。
结果：Alice 与 Mallory 共享密钥，Mallory 与 Bob 共享另一密钥，
     Mallory 解密 Alice 的流量再重新加密转发给 Bob —— 双方都察觉不到！
```

**对策**：必须结合**身份认证**：

- 数字签名（RSA / DSA / Ed25519 签名公钥）
- 证书（PKI / CA，HTTPS 的信任链）
- 预共享密钥（PSK）

这就是为什么 TLS 握手既要 DH 协商密钥，又要证书证明"对面真的是 example.com"。

## 椭圆曲线版（ECDH）

把"模 p 乘法群"换成"椭圆曲线点群"，离散对数变得更难（同等安全强度下密钥更短）：

| 安全强度 | 经典 DH (FFDHE) | 椭圆曲线 ECDH |
|---------|----------------|---------------|
| 80 bit  | 1024 bit       | 160 bit       |
| 128 bit | 3072 bit       | 256 bit       |
| 256 bit | 15360 bit      | 512 bit       |

现代 TLS 1.3 默认用 ECDHE（X25519 / P-256），经典有限域 DH 已基本淘汰。

## 应用

- **TLS 前向保密（PFS）**：每次握手生成临时 DH/ECDH 密钥对（DHE / ECDHE），
  会话结束后销毁；即使长期私钥日后泄露，也无法解密历史流量
- **Signal 协议**：用 X3DH（扩展 DH）做异步密钥协商，实现端到端加密
- **IPsec**：IKE 阶段用 DH 协商 IKE SA 密钥
- **SSH**：DH 交换会话密钥（curve25519-sha256）

## 与 RSA 的区别

| 维度 | RSA | DH |
|-----|-----|-----|
| 能加密 | ✓ | ✗ |
| 能签名 | ✓ | ✗（需配合签名算法） |
| 能协商密钥 | ✓（公钥加密对称密钥） | ✓（且支持前向保密） |
| 数学难题 | 大整数分解 | 离散对数 |
| 前向保密 | ✗（长期私钥泄露=历史全暴露） | ✓（DHE 临时密钥） |

> 因此 TLS 1.3 起已移除 RSA 密钥交换，只保留 (EC)DHE ——
> 既是为了前向保密，也是为了抗 RSA 未来被量子 Shor 算法破解。

## 本包实现要点

- 用 `math/big.Exp(base, exp, mod)` 做模幂（O(log exp)）
- 公共参数固定为 `p=23, g=5`（教材经典，确定性 demo）
- `PrivateKey(p)` 用 `crypto/rand.Int` 生成 `[1, p)` 内密码学随机数
- demo 用固定 `a=6, b=15` 保证可复现（真实场景绝不能固定私钥）
- `PeerSession` 封装一方状态，让 demo 流程与真实协议结构对齐

## 判定红线

"少了下面任意一条就不算 DH 密钥交换"——评审时按此清单逐项核对：

- **必须基于离散对数难题**。核心运算必须是 `g^x mod p`（有限域 DH）或椭圆曲线点群上的 `x·G`（ECDH）。
  - 若"用对称密钥直接交换"（如把 AES 密钥明文发给对方），那不是密钥协商，是密钥分发，且完全不安全。
  - 若双方各自生成密钥但通过私钥传送而非数学协商，也不是 DH。
- **双方最终必须算出相同的共享密钥 s**。这是 DH 最核心的数学不变量：
  - 有限域：`(g^b)^a ≡ (g^a)^b (mod p)`，因为 `ab = ba`。
  - 椭圆曲线：`b·(a·G) = a·(b·G) = (ab)·G`。
  - 若"双方密钥不等"，整个协议就是错的——这是绝对的正确性红线。
- **必须有公共参数（p, g 或基点 G）+ 各自私钥（a, b）+ 公开值（A, B）**。缺任何一方都构不成协商。
  - 私钥 a/b 必须保密、必须随机；若"私钥写死为常量"且未声明仅供确定性 demo，不合规。
- **模数 p 必须是大素数**（或曲线必须是安全曲线）。若 p 是小数（如 p=23）也未声明"仅教学"，安全性归零。
  - 没有"大素数前提"，离散对数可被轻易求解，DH 失去意义。
- **窃听者看到 p, g, A, B 仍算不出 s**——这正是 DH 的安全契约。如果实现中泄露了 a/b（如日志打印私钥），是致命漏洞。
- **必须说明中间人攻击（MITM）缺陷**：DH 本身不认证身份，攻击者可充当透明代理。
  - 若声称"DH 自身提供身份认证"或"DH 抗 MITM"，是根本性误解。
  - 生产实现必须配数字签名 / 证书 / PSK 做认证（TLS 就是 DH + 证书）。
- **支持前向保密**（用临时密钥 DHE）：会话结束后销毁 a/b，即使长期密钥日后泄露也无法解密历史流量。这是 DH 相对 RSA 密钥交换的核心优势。

## 参考

- Wikipedia: Diffie–Hellman key exchange
- Diffie & Hellman 1976 *New Directions in Cryptography*（IEEE Trans. IT）
- RFC 7919 Negotiated Finite Field Diffie-Hellman Ephemeral Parameters（FFDHE 群）
- RFC 7748 Curve25519 / Curve448（ECDH 现代标准）
- Bruce Schneier *Applied Cryptography* Ch.22
