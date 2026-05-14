---
Title: Python
Date: 2025-9-22
---

## 字符串格式化

在 Python 中 `%` 左边是字符串的话，右边可以是一个值，表示对应的字符串格式化

```python
key = 'my_var' value = 1.234
formatted = '%-10s = %.2f' %(key, value) print (formatted)
>>> my_var
my_var = 1.23
```

在 新的 Python 中，我们可以使用 `format` 方法来格式化字符串

```python
key = 'my_var' value = 1.234
formatted = '{:<10s} = {:.2f}'.format(key, value) print (formatted)
>>> my_var
my_var = 1.23
```

## new 方法

在 Python 中真正构造一个对象的其实是 New 方法，而 __init__ 方法只是一个打前阵的小兵。