# HMAC · 设计笔记

## 标准

- **RFC 2104** "HMAC: Keyed-Hashing for Message Authentication"（Bellare, Canetti, Krawczyk 1996）
- https://tools.ietf.org/html/rfc2104

## 核心算法

```
HMAC(K, m) = H((K' ⊕ opad) || H((K' ⊕ ipad) || m))

K' = 密钥处理：
  - 若 len(K) > blockSize：K' = H(K) （先哈希）
  - 再补 0 到 blockSize

ipad = 0x36 重复 blockSize 次
opad = 0x5c 重复 blockSize 次
H = SHA-256（本包）
```

## 最小可识别特征

1. **基于哈希 + 密钥**（不是纯哈希，也不是加密）
2. **嵌套哈希**（内层 H(ipad||msg)，外层 H(opad||inner)）
3. **ipad/opad 常量**（0x36 / 0x5c，RFC 2104 规定）

## 判定红线

- 无密钥的哈希（只算 SHA-256(msg)）→ 不是 HMAC，是普通哈希
- 不嵌套（只 H(key||msg)，即"前缀 MAC"）→ 长度扩展攻击可伪造
- ipad/opad 用其他值 → 不符合 RFC 2104

## 安全性

- **抗长度扩展攻击**：嵌套结构确保攻击者无法用 H(key||msg) 的结果伪造 H(key||msg')。
- **恒定时间比较**：验证 MAC 时用 constantTimeCompare（遍历全部字节），防时序攻击。
- HMAC-SHA-256 目前无已知有效攻击。

## 应用

| 场景 | 用法 |
|------|------|
| AWS API 签名（SigV4） | HMAC(secret, canonical_request) |
| JWT（JSON Web Token） | HS256 = HMAC-SHA-256(header.payload) |
| TLS 1.2 记录层 MAC | HMAC(MAC_key, record) |
| Git commit 对象 ID | 实际是 HMAC-SHA1（历史选择） |
| 消息完整性 + 真实性 | 比"只哈希"更强（需密钥才能伪造） |

## 与其他算法的关系

- HMAC vs SHA-256：SHA-256 无密钥（任何人可算）；HMAC 需密钥（只有持密钥者可算）。
- HMAC vs RSA 签名：HMAC 是对称的（收发双方共享密钥）；RSA 是非对称的（私钥签、公钥验）。
- HMAC vs AEAD（如 AES-GCM）：AEAD 同时加密+认证；HMAC 只认证（不加密）。

## 本包实现要点

- 用 crypto/sha256 标准库做底层哈希
- 手写 ipad/opad + 嵌套结构（教学清晰）
- processKey 处理长密钥（>64 字节先哈希）
- constantTimeCompare 恒定时间比较（防时序）
- 与 Go 标准库 crypto/hmac 交叉验证（测试 TestHMACMatchesStdLib）

## 参考

- RFC 2104: https://tools.ietf.org/html/rfc2104
- Wikipedia: https://en.wikipedia.org/wiki/HMAC
