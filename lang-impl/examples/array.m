// M 语言示例：数组
fn sum(arr) {
  let total = 0;
  let i = 0;
  while (i < len(arr)) {
    total = total + arr[i];
    i = i + 1;
  }
  return total;
}

let nums = [10, 20, 30, 40];
return sum(nums);  // 100
