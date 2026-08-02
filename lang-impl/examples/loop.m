// while 循环：累加 1..100
fn sumTo(n) {
  let total = 0;
  let i = 1;
  while (i <= n) {
    total = total + i;
    i = i + 1;
  }
  return total;
}

return sumTo(100);  // 5050
