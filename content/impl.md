---
Title: Go 代码大全
Date: 2023-11-15
---

## 接口的显式实现

在 Go 语言中，与 Java 或 Python 等语言不同，结构体（struct）无需显式声明对某个接口的实现。
例如，在 Java 中我们常会写 class Student implements Person 来明确指定接口实现，这种方式具有一定的优势。
如果采用隐式实现机制，当接口中的方法发生变更时，之前实现该接口的结构体可能会因缺少某个方法而不再符合接口要求。
然而，在代码的某些地方可能仍然将该结构体作为该接口类型进行传递，从而引发错误。

许多开源项目采用以下方式来显式确保接口的实现：

```
var _ 接口类型 = (*实现类型)(nil)
```

1. 明确告知用户该结构体实现了特定接口，增强代码的可读性和可维护性；
2. 防止因接口或结构体的变更导致实现关系被意外破坏，建立一种编译期的强绑定检查机制。

## context

### 超时控制

在平常的开发过程中，往往会存在一些耗时的操作阻塞住我们的 go 程，比如调用一些第三方的接口，数据库查询，从某个 chan 中去读取对应的数据，
那这个时候往往就很有必要去做好超时控制， 在 Go 语言中去实现超时控制是非常简单的，只需要使用 context.WithTimeout()，

```
ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Second)
defer cancelFunc()
```

### 上下文信息传递

有时候我们还希望使用 context 去携带一些信息，比如某个服务的链路 Id, 这一点在一些 可观测场景或者是微服务框架中非常的常见, 但是由于 context.WithValue 的接口其实并不是类型
安全的，因为它的接口是一个 interfance{} 类型，所以我们可以把任何数据给塞到这个 context 里面去的，如果你把一切塞到 context 里面去的话，我们也许就会写出下面这种代码:
这样导致的后果就是无法根据函数参数去判断函数是做什么的。
```
func add(ctx context.Context) int{
    a := ctx.Value("a").(int)
    b := ctx.Value("b").(int)
    return a + b
}
```
总结起来的话, 如果你过度使用 context.WithValue() 往往会带来下面这些问题

1. 失去类型安全（No Type Safety）: interface{} 导致类型检查失效
2. 隐藏依赖，使代码难以理解（Hidden Dependencies）
3. 隐藏依赖，使代码难以理解（Hidden Dependencies）

其次 context.WithValue 这个接口的另外一个坏处就是，数 比如如果你使用了类似 "id" 这样的东西作为键的时候, 但是可能在另外一个地方我们也可能去使用 "id" 这个 key 去 set 一些值, 这就会导致前面一个 id 被后面一个 id 给覆盖了，最好的情况就是我们使用一个未导出的结构体作为键。

```
type Key struct{}
```
