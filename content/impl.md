---
Title: Go 代码大全
Date: 2023-11-15
---

## Go 语言的封装

Go 语言关于结构体的字段封装是通过字段命名的大小写来完成的, 当我们


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

## 良好的 type 命名

在定义接口的时候使用一个合适的类型, 避免已经一些不必要的实现，比如下面这种:

```
type CredentialsService interface {
	Get(ctx context.Context, sid string, scheme string) (string, error)
}

type CredentialsService interface {
	Get(ctx context.Context, sid SessionID, scheme SecuritySchemeName) (AuthCredential, error)
}
```

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

###  time.Sleep()

在我们日常开发中, 为了等待一些连接的建立或者是让 CPU 运转的更好往往可能会使用一个 time.Sleep() 方法，比如下面这段代码,

```
for {
    select {
        case <-ctx.Done():
        case <-ch:
        default:
            time.Sleep(10 * time.Second)
    }
}
```

如果你一直没有从 <-ch 中去读取好对应的数据的话, 那么这个 for 循环就会一直的空转, 这样其实不利于别的任务的执行, 而加上这个 sleep 之后, 可以避免CPU空转, 但是 sleep 在这个地方其实
用的并不是很好，因为在 sleep 的时候我们无法判断当前外部的环境是否已经超时,也就是 ctx 控制的链路, 我们必须要等到这个 sleep 结束之后才会去判断是否超时

那么优雅的写法是怎样子的呢?

```
func sleepWithCtx(ctx context.Context, d time.Duration) error {
	// Wait a bit before retrying
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
```