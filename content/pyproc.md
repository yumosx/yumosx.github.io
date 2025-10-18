---
Title: PyProc 源码分析 (持续更新中)
Date: 2025-9-22
---

PyProc 的 frame 结构分成了两个部分, 第一个部分是对应的帧头, 第二部是对应的 数据部分, 也就是对应的数据部分，类型下面这样
```
[header][payload]
```

这个数据结构在 Go 语言中表示为:

```
type Frame struct {
	Header  FrameHeader
	Payload []byte
}
```

整个数据帧格式分成了下面三个字段: 第一个字段是魔数, 用来验证对应的帧, 第二个字段是总共的帧长度，第三个是请求的ID, 同时CRC32C 表示对应的校验和

```
const (
    FrameHeaderSize = 18  // header总大小
    MagicByte1 = 0x50     // 魔数字节1 ('P')
    MagicByte2 = 0x59     // 魔数字节2 ('Y')
)

type FrameHeader struct {
    Magic [2]byte
    Length uint32
    RequestID uint32
    CRC32C    uint32
}
```

在 pyproc 里面还有另外一个结构，叫做 Framer 专门用来处理对应的流操作，它的定义如下:
```
type Framer struct {
    rw           io.ReadWriter  // 底层流
    maxFrameSize int            // 安全限制
    enhancedMode bool           // 协议模式
}
```

那这里为什么使用 io.ReadWriter 这个类型来表示对应的流, 如果比较熟悉 Go 语言的话，在 net 包中有一个核心的抽象叫做 net.Conn, 它是接口类型
这个接口类型其实是 io.ReadWriter 的 子接口实现。


在 pyproc 中有一个核心的数据结构是 Pool, 这个 Pool 主要用来管理我们的 Go 语言程序和 Python 之间的通信,

```
type Pool struct {
  opts     PoolOptions     // 配置选项
  logger   *Logger         // 日志记录器
  workers  []*poolWorker   // workers 池子
  nextIdx  atomic.Uint64   // 用于轮询调度的索引
  shutdown atomic.Bool     // 关闭状态标志
  wg       sync.WaitGroup  // 等待组，用于优雅关闭
  semaphore chan struct{}  // 信号量，用于背压控制
  healthMu sync.RWMutex    // 健康状态锁
  healthStatus HealthStatus // 健康状态
  healthCancel context.CancelFunc // 健康监控取消函数
  activeRequests map[uint64]*activeRequest // 活动请求跟踪
  activeRequestsMu sync.RWMutex // 活动请求锁
}
```

在这个数据结构中, 还存在另外一个数据结构叫做 poolWorker, 就是这个 pool 中放的工作线程:
```
type poolWorker struct {
	worker    *Worker
	connPool  chan net.Conn
	requestID atomic.Uint64
	healthy   atomic.Bool
}
```

在 Pool 的初始化阶段, 会根据配置的 work 数量来去开启对应的 work, 同时我们也会初始化对应的 work 的状态为 healthy 状态, 如果某个 work 启动失败，我们将会去回滚已经启动的线程
```
if err := pw.worker.Start(ctx); err != nil {
    // 错误处理：停止已启动的线程
    for j := 0; j < i; j++ {
           _ = p.workers[j].worker.Stop()
    }
    return fmt.Errorf("failed to start worker %d: %w", i, err)
}
```


Pool 有一个 call 方法会选择一个 work 来执行, 这种选择方式采用的是轮询的方式
```
// input 是我们的请求参数
Call(ctx context.Context, method string, input any, output any) error
```

当轮询到对应的 work 的时候, 从这个 work 中的 connPool 中读取一个连接, 如果 work 中的 pool 没有对应的连接, 那就创建一个连接，之后就是发送对应的消息,在发送消息之前会给这个请求加上一个 requestID
然后会调用framer 相关的方法去发送对应的消息。