// Package lexer 词法分析的更多背景见 NOTES.md。
//
// 与 parser 的关系：lexer 产出 []core.Token，parser 消费它构造 AST。
// lexer 不做任何语法判断（如"括号是否匹配"），只识别单个词法单元。
package lexer
