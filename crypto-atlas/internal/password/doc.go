// Package password 的更多背景见 NOTES.md。
//
// 本包是 crypto-atlas 里少数"非密码学原语"的包：它不产生密文或摘要，
// 而是评估口令强度。把它放进 atlas 是为了和哈希（sha256/md5）、口令存储
// 形成对照——"先判断口令够不够强"是"再把它哈希存库"的前置步骤。
package password
