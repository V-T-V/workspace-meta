# AES · 设计笔记

## 历史

- **Rijndael**：由两位比利时密码学家 Joan Daemen、Vincent Rijmen 设计
- **1997-2000 NIST 公开竞赛**：AES（Advanced Encryption Standard）是 NIST 为取代 DES 举办的公开竞赛，从 15 个候选算法中选出 Rijndael
- **2001 年 11 月**：FIPS PUB 197 正式发布，AES 成为美国联邦政府标准
- 名字 AES 是"标准"名，Rijndael 是"算法"名；Rijndael 支持多种分组/密钥长度，AES 固定分组为 128 位、密钥 128/192/256 位

## 分组密码概念

分组密码（block cipher）把明文切成固定大小的"块"，逐块加密。AES 的块大小恒为 **16 字节（128 位）**。

- 密钥长度决定轮数：AES-128 → 10 轮，AES-192 → 12 轮，AES-256 → 14 轮
- 每轮包含 4 个步骤：SubBytes（S-Box 替换）、ShiftRows（行移位）、MixColumns（列混淆）、AddRoundKey（与轮密钥 XOR）
- 这 4 步组合了 **混淆（confusion，S-Box）** 与 **扩散（diffusion，ShiftRows+MixColumns）**，使密文的每一位都依赖密钥和明文的每一位

## 工作模式（Mode of Operation）

分组密码一次只能加密一个块。要加密多块数据，需要"工作模式"。

| 模式 | 全称 | 思路 | 是否推荐 |
|------|------|------|----------|
| **ECB** | Electronic Codebook | 每块独立加密 | ❌ 不安全（教学专用） |
| **CBC** | Cipher Block Chaining | 每块先与前块密文 XOR 再加密 | ✅ 机密性 OK，但不抗篡改 |
| **CTR** | Counter | 用递增计数器加密成流密码 | ✅ 可并行 |
| **GCM** | Galois/Counter Mode | CTR + GMAC 认证 | ✅✅ 推荐（AEAD） |

### ECB 致命缺陷——为什么不能用

ECB 把每个块独立加密，**相同的明文块永远产生相同的密文块**。于是明文中的"模式"会原样渗透到密文里：

- 最著名的演示是 **"ECB 企鹅图"（Tux penguin）**：把一张 Linux 吉祥物企鹅位图用 ECB-AES 加密后，密文图像里依然能看清企鹅轮廓
- 本包的 demo 用 `"AAAAAAAAAAAAAAAA" × 2`（两块相同明文）做最小化复现：两块密文也相同

### CBC 的改进

CBC 让每个明文块先与前一块密文 XOR，第一块与 IV（初始向量）XOR。这样"链式依赖"使得即使两块明文相同，密文也不同。本包 demo 直观验证：相同明文块在 CBC 下产生不同密文块。

CBC 的注意事项：
- IV 必须不可预测（应随机生成），但不必保密，通常随密文一起传输
- CBC 只保证机密性，**不抗主动篡改**；要同时防篡改需用带认证的模式（GCM）

## 安全性

- 截至 2026 年，**AES 没有已知的可行（< 2^128 工作量）攻击**
- 相关密钥攻击（2009 Biryukov/Khovratovich）只对 AES-256 的理论弱化，不实际威胁正常使用
- 侧信道攻击（时序/缓存）针对实现而非算法；标准库 `crypto/aes` 用硬件指令（AES-NI）且恒定时间，可防御
- 真正的风险几乎都在"用错模式"（ECB、IV 复用、缺认证），而非算法本身

## 应用

- **TLS**：HTTPS 大量使用 AES-GCM（如 `TLS_AES_256_GCM_SHA384`）
- **文件/磁盘加密**：LUKS、BitLocker、FileVault
- **VPN**：IPsec、WireGuard 都基于 AES 或 ChaCha20
- **数据库/应用层加密**：字段级加密、信封加密（envelope encryption）

## 本包实现要点

- 用标准库 `crypto/aes`（教学库，不重造 S-Box）
- 密钥长度校验：仅接受 16/24/32 → AES-128/192/256
- 用 `core.PKCS7Pad`/`PKCS7Unpad`（blockSize=16）处理填充
- ECB：逐块 `block.Encrypt/Decrypt`
- CBC：用 `cipher.NewCBCEncrypter`/`NewCBCDecrypter`
- Demo 完全确定性（固定 key/iv，无随机数）

## 判定红线

"少了下面任意一条就不算 AES"——评审时按此清单逐项核对：

- **块大小恒为 128 位（16 字节）**。如果"块大小可变"或"用 64 位块"，那是 DES/Blowfish 之类，不是 AES。
- **密钥长度只接受 128/192/256 位**。若接受 56 位密钥，那是 DES，不是 AES；若接受任意长度密钥，实现不合规。
- **轮数随密钥长度变化（10/12/14 轮）**。固定轮数说明没有按密钥长度分支。
- **必须显式处理填充（PKCS7 / 块对齐）**。明文长度非块整数倍时若直接加密/截断，缺填充就不算完整实现。
  - 即使明文已对齐，PKCS7 也必须再补一整块——少了这条就是填充实现错误。
- **工作模式必须明示**。如果"用 ECB 但不说明其缺陷"，等于把 ECB 当默认安全模式推销，是严重设计缺陷：
  - ECB 的致命缺陷是相同明文块产生相同密文块（模式泄漏），评审必须能讲清"ECB 企鹅图"原理。
  - 生产环境必须用 CBC/CTR/GCM；GCM（AEAD）是唯一同时提供机密性 + 完整性的推荐模式。
- **IV/Nonce 使用必须正确**：
  - CBC 的 IV 必须不可预测（随机），CTR/GCM 的 nonce 在同一密钥下绝不能复用（否则一次性密码本式灾难）。
  - 若"IV 写死为全 0 / 固定常量"且未声明仅供确定性 demo，不合规。
- **加密必须可解密还原**（往返一致性）。这是任何分组密码实现的最基本正确性红线。
- **不能自造 S-Box / 轮常数**。本教学包用标准库 `crypto/aes`；若自行实现 Rijndael，S-Box 必须= FIPS 197 规定值，否则是"另一个算法"。

## 参考

- FIPS PUB 197：AES 官方标准
- NIST SP 800-38A：分组密码工作模式（ECB/CBC/CFB/OFB/CTR）
- NIST SP 800-38D：GCM 模式
- Wikipedia: Block cipher mode of operation（含 ECB 企鹅图）
- Bruce Schneier "Applied Cryptography"
