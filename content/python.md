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


## classmethod 装饰器

`classmethod` 是 Python 内置的装饰器，用于定义类方法。类方法的第一个参数是类本身（通常命名为 `cls`），而不是实例（`self`）。

### 基本用法

```python
class MyClass:
    count = 0  # 类属性

    def __init__(self):
        MyClass.count += 1

    @classmethod
    def get_count(cls):
        """获取创建的实例数量"""
        return cls.count

# 创建实例
a = MyClass()
b = MyClass()

# 通过类调用
print(MyClass.get_count())  # 输出: 2

# 也可以通过实例调用
print(a.get_count())  # 输出: 2
```

### 核心特点

1. **第一个参数是类**：`classmethod` 修饰的方法自动接收类作为第一个参数
2. **可以访问类属性**：能够读取和修改类级别的状态
3. **支持继承**：在子类中调用时，`cls` 会指向子类而非父类

### 常见应用场景

#### 1. 工厂方法

`classmethod` 最常见的用途是定义替代构造函数：

```python
class Person:
    def __init__(self, name, age):
        self.name = name
        self.age = age

    @classmethod
    def from_birth_year(cls, name, birth_year):
        """根据出生年份创建实例"""
        import datetime
        current_year = datetime.datetime.now().year
        age = current_year - birth_year
        return cls(name, age)

# 使用工厂方法创建
p = Person.from_birth_year('张三', 1990)
print(p.name, p.age)  # 输出: 张三 35
```

#### 2. 支持继承的工厂方法

`classmethod` 在继承场景下特别有用，因为 `cls` 会自动指向正确的子类：

```python
class Animal:
    def __init__(self, name):
        self.name = name

    @classmethod
    def create(cls, name):
        """工厂方法，子类调用时返回子类实例"""
        return cls(name)

class Dog(Animal):
    def speak(self):
        return f'{self.name} says Woof!'

class Cat(Animal):
    def speak(self):
        return f'{self.name} says Meow!'

# 自动创建正确的子类实例
dog = Dog.create('Buddy')
cat = Cat.create('Whiskers')

print(dog.speak())  # Buddy says Woof!
print(cat.speak())  # Whiskers says Meow!
```

#### 3. 类状态管理

用于管理类级别的配置或状态：

```python
class Database:
    _connection_string = None

    @classmethod
    def configure(cls, connection_string):
        """配置数据库连接"""
        cls._connection_string = connection_string

    @classmethod
    def get_connection(cls):
        """获取数据库连接"""
        if cls._connection_string is None:
            raise ValueError('Database not configured')
        return f'Connected to {cls._connection_string}'

# 配置一次，全局可用
Database.configure('postgresql://localhost/mydb')
print(Database.get_connection())
```

### 最佳实践

1. **优先使用 classmethod 而非 staticmethod**：当需要访问类属性或支持继承时，`classmethod` 是更好的选择
2. **工厂方法命名约定**：以 `from_` 开头，如 `from_dict`、`from_string`、`from_json`
3. **避免滥用**：如果方法不需要访问类或实例，考虑使用普通函数或 `staticmethod`

## event 循环

在 go 中开启一个 thread 是非常简单的，只需要调用 `go` 关键字即可。在 python 中也有类似的操作，那就是使用 `threading.Thread` 类。
```python
self._loop = asyncio.new_event_loop()
self._thread = threading.Thread(target=self._run_loop, daemon=True)
self._thread.start()
```

这个 _run_loop 方法就是 event 循环，用于处理事件。
```python
def _run_loop(self):
    asyncio.set_event_loop(self._loop)
    self._loop.run_until_complete(你的操作)
```