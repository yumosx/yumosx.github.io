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

```go
var _ 接口类型 = (*实现类型)(nil)
```

1. 明确告知用户该结构体实现了特定接口，增强代码的可读性和可维护性；
2. 防止因接口或结构体的变更导致实现关系被意外破坏，建立一种编译期的强绑定检查机制。

## 良好的 type 命名

在定义接口的时候使用一个合适的类型, 避免已经一些不必要的实现，比如下面这种, scheme 的类型是 string 类型，但是如果单使用 string 的话虽然没有问题
但是在语义上并不是很清晰。

```go
type CredentialsService interface {
	Get(ctx context.Context, sid string, scheme string) (string, error)
}

type CredentialsService interface {
	Get(ctx context.Context, sid SessionID, scheme SecuritySchemeName) (AuthCredential, error)
}
```
## 接口组合

函数式编程模式中最常用的模式就是接口组合模式, 它的原理是将多个接口组合起来, 以实现更复杂的功能。
同时接口最好 是小而专的, 每个接口只负责一个功能, 然后代码只去实现特定的接口，这样可以避免接口之间的耦合, 也方便了代码的维护。

```go
type LLM interface {
	Chat
	Stream
}

type Chat interface {
}

type Stream interface {
}
```

使用接口的另外一个好处就是可以去 mock 对应的实现, 这样可以在测试中去模拟对应的实现, 而不需要去依赖真实的实现。

### 不要设计容纳一切的接口

dj 老爷子一直是 goto 有害论的提出者，goto 语句之所以被老爷子抵制的原因是因为这个东西太过于灵活，往往导致被滥用
解决这种强大容纳一切的接口的方法就是拆分成一列有限制的设计，比如将 goto 拆分成 while 循环，if 判断，break 语句


## context

```go
type Context interface {
    // 返回上下文的截止时间，以及截止时间是否已过
    Deadline() (deadline time.Time, ok bool)
    // 返回一个 channel, 当上下文被取消或者超时的时候, 会 channel 会关闭
    // 如果关闭之后, 可以通过 Err() 方法�获取到对应的错误信息
    Done() <-chan struct{}
    Err() error
    // 返回上下文中的值
    // 如果上下文中没有对应的值, 则返回 nil
    Value(key any) any
}
```


### 超时控制

在平常的开发过程中，往往会存在一些耗时的操作阻塞住我们的 go 程，比如调用一些第三方的接口，数据库查询，从某个 chan 中去读取对应的数据，
那这个时候往往就很有必要去做好超时控制， 在 Go 语言中去实现超时控制是非常简单的，只需要使用 context.WithTimeout()，

```
ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Second)
defer cancelFunc()
```

超时风暴问题, A->B->C->D, 这条链路可能网关超时，A 调用 B 超时，B 调用 C 超时，C 调用 D 超时，导致整个链路超时。所以每个环境都需要做好超时控制，1业务限制超时时间，2业务限制并发数。



### 上下文信息传递

有时候我们还希望使用 context 去携带一些信息，比如某个服务的链路 Id, 这一点在一些 可观测场景或者是微服务框架中非常的常见, 但是由于 context.WithValue 的接口其实并不是类型
安全的，因为它的接口是一个 interfance{} 类型，所以我们可以把任何数据给塞到这个 context 里面去的，如果你把一切塞到 context 里面去的话，我们也许就会写出下面这种代码:
这样导致的后果就是无法根据函数参数去判断函数是做什么的。
```go
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

```go
for {
    select {
        case <-ctx.Done():
        case <-ch:
        default:
            time.Sleep(10 * time.Second)
    }
}
```

如果你一直没有从 <-ch 中去读取好对应的数据的话, 那么这个 for 循环就会一直的空转, 这样其实不利于别的任务的执行, 而加上这个 sleep 之后, 可以避免CPU空转, 但是 sleep 在这个地方其实用的并不是很好，因为在 sleep 的时候我们无法判断当前外部的环境是否已经超时,也就是 ctx 控制的链路, 我们必须要等到这个 sleep 结束之后才会去判断是否超时

那么优雅的写法是怎样子的呢?

```go
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

## iter.Seq

Go 1.23+ 引入的迭代器模式，用于 range 遍历自定义数据结构。

### 基本用法

```go
// Seq[T] 单值迭代器
func All[E any](s []E) iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

// Seq2[K, V] 双值迭代器 (类似 map)
func Entries[K comparable, V any](m map[K]V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}

// 使用
for v := range All([]int{1, 2, 3}) {
    fmt.Println(v)
}

for k, v := range Entries(map[string]int{"a": 1, "b": 2}) {
    fmt.Println(k, v)
}
```

### 实际场景：分页查询

```go
func Paginate(db *sql.DB, query string) iter.Seq2[int, []User] {
    return func(yield func(int, []User) bool) {
        page := 1
        for {
            var users []User
            rows, _ := db.Query(query+" LIMIT ? OFFSET ?", 10, (page-1)*10)
            for rows.Next() {
                var u User
                rows.Scan(&u.ID, &u.Name)
                users = append(users, u)
            }
            
            if len(users) == 0 || !yield(page, users) {
                break
            }
            page++
        }
    }
}

// 使用
for pageNum, users := range Paginate(db, "SELECT * FROM users") {
    fmt.Printf("第%d页: %d条记录\n", pageNum, len(users))
}
```



## 模式

### 懒加载

这源自于我的一个场景，原先我在编写一段程序的时候是在一开始加载所有的资源，但是后来我发现在有些场景下，我只需要在需要的时候去加载资源，而不是在一开始加载所有的资源，所以我就使用了这种模式

在go 语言中我的实现如下:

```go
type LazyLoader struct {
	loaders map[string]func() any
	cache   map[string]any
	mu      sync.Mutex
}

func NewLazyLoader() *LazyLoader {
	return &LazyLoader{
		loaders: make(map[string]func() any),
		cache:   make(map[string]any),
	}
}

func (l *LazyLoader) Register(key string, loader func() any) {
	l.loaders[key] = loader
}

func (l *LazyLoader) Load(key string) any {
	l.mu.Lock()
	defer l.mu.Unlock()

	if val, ok := l.cache[key]; ok {
		return val
	}

	val := l.loaders[key]()
	l.cache[key] = val
	return val
}
```

### borker + 订阅模式

在一些 event 设计中, 会使用 borker + 订阅 + 推送 模式, 来实现事件的发布和订阅。
- 首先你需要一个 queue, 这个 queue 用来从queue 去读取事件, 也可以用来写入事件。
- 然后你需要一个 borker, 这个 borker 用来发布事件, 也可以用来订阅事件。

queue 接口实现
```go
type Queue interface {
	Read() (any, error)
	Write(msg any) error
}
```

broker 接口实现
```go
type Broker interface {
	Publish(v msg) Queue
}
```

## 强大的 sync.Pool

sync.Pool 可以说是go 语言中最强大的兵器之一了, 最常见的一种用法就是下面这个:

```go
var attrs = sync.Pool{
	New: func() any {
		return & Type {}
	}
}
```

但是这其实我们是固定思维模式在作祟，其实我们也可以这样用:

```go
type Processor struct{
	batch int
}


func NewProcessor(batch int) *Processor{
	attr.New = func() any {
		return &Type{}
	}
	return &Processor{}
}


func (p *Processor) Process() {
	attrs.Get().(*Type{})
}
```
### 源码分析

```go
type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal
	localSize uintptr        // size of the local array

	victim     unsafe.Pointer // local from previous cycle
	victimSize uintptr        // size of victims array
	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() any
}
```

pool 中存在两个字段，第一个字段叫做 local, 第二个字段叫做 victim。每次垃圾回收的时候，Pool 会把 victim 中的资源释放出来，然后把 local 中的资源赋值给 victim。

```go
func poolCleanup() {
    // 丢弃当前victim, STW所以不用加锁
    for _, p := range oldPools {
        p.victim = nil
        p.victimSize = 0
    }

    // 将local复制给victim, 并将原local置为nil
    for _, p := range allPools {
        p.victim = p.local
        p.victimSize = p.localSize
        p.local = nil
        p.localSize = 0
    }
    oldPools, allPools = allPools, nil
}
```

sync.Pool 中有一个 local 指针，这个指针指向叫做 poolLocalInternal 的结构体。
```go
type poolLocalInternal struct {
	private any       // Can be used only by the respective P.
	shared  poolChain // Local P can pushHead/popHead; any P can popTail.
}
```

put 方法 中有一个很好玩的东西，那就是 pin 方法。这个方法的作用就是将当前 goroutine 固定到 P 上去，也就是说这个 P 只能执行这一个G,
其次如果 private 字段是空的，直接设置这个字段，这样也意味着只有这个P 可以访问这个字段，其次把 x 加入到 shared 队列中。

可以由任意的 P 访问，但是只有本地的 P 才能 pushHead/popHead，其它 P 可以 popTail，相当于只有一个本地的 P 作为生产者（Producer），多个 P 作为消费者（Consumer），它是使用一个 local-free 的 queue 列表实现的。
```go
func (p *Pool) Put(x interface{}) {
    if x == nil { // nil值直接丢弃
        return
    }
    l, _ := p.pin()
    if l.private == nil { // 如果本地private没有值，直接设置这个值即可
        l.private = x
        x = nil
    }
    if x != nil { // 否则加入到本地队列中
        l.shared.pushHead(x)
    }
    runtime_procUnpin()
}
```

然后我们看看 Get 方法:
```go
func (p *Pool) Get() interface{} {
    // 把当前goroutine固定在当前的P上
    l, pid := p.pin()
    x := l.private // 优先从local的private字段取，快速
    l.private = nil
    if x == nil {
        // 从当前的local.shared弹出一个，注意是从head读取并移除
        x, _ = l.shared.popHead()
        if x == nil { // 如果没有，则去偷一个
            x = p.getSlow(pid) 
        }
    }
    runtime_procUnpin()
    // 如果没有获取到，尝试使用New函数生成一个新的
    if x == nil && p.New != nil {
        x = p.New()
    }
    return x
}
```
> **这段话的意思**：sync.Pool 在 OTel 这种"大量小对象、字符串密集"的解析场景下，**只能减少几个百分点的分配**，治标不治本。要真正优化收益更大的是另外两件事——**避免结构里的多余指针**（或用 arena 分配器一整块申请、用完整块释放，跳过逐对象 GC）和**反序列化时用 `[]byte` 引用原始 buffer 而不是 `string` 拷贝**。前两点收益比 sync.Pool 高一个数量级。

## mutex

在 Go 语言中实现并发同步的方式就是使用 sync.Mutex 来实现互斥锁，来保护共享资源的访问。sync.Mutex 里面包含了一个状态变量，来记录当前是否有 goroutine 正在持有锁。所以这个 sync.Mutex 是不能被拷贝的，否则会导致锁的释放问题。

其次go 语言的互斥锁其实不是 可以重新进入的，也就是相同的 goroutine 不能重新进入锁，否则会导致死锁。
