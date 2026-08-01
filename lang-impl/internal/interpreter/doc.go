// Package interpreter 实现 M 语言的树遍历解释执行，更多背景见 NOTES.md。
//
// 三种执行策略对比：
//
//  1. 树遍历（tree-walking，本包所用）：直接递归遍历 AST，每个节点即时求值。
//     优点：实现简单、启动快、易调试；缺点：每次运行都要遍历 AST、
//     类型分发在运行时做，速度慢。
//
//  2. 字节码（bytecode VM）：把 AST 编译成一串字节码指令，再由虚拟机执行。
//     优点：可优化、指令紧凑；缺点：需要编译器 + VM 两层。
//
//  3. 原生编译（AOT/JIT）：直接编译到机器码（或 WASM）。
//     优点：速度最快；缺点：实现复杂。
//
// 本包选择树遍历，因为教学目标是"最小可用的解释器核心"。
//
// 核心数据结构：
//
//	Environment —— 作用域链（map + parent 指针）
//	funcs        —— 函数表（name → FnDecl）
//
// 控制流技巧：
//
//	return 用 panic(returnSignal{...}) + recover 实现"非局部跳转"，
//	这是树遍历解释器实现 return/异常的标准模式（避免每层函数都返回多值）。
package interpreter
