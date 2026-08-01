// M 语言示例：递归 fib
fn fib(n) {
  if (n < 2) {
    return n;
  }
  return fib(n - 1) + fib(n - 2);
}

// 计算 fib(10)
let r = fib(10);
return r;  // 期望输出 55
