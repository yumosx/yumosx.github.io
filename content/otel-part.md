---
Title: opentelemetry go 随手记 
Date: 2025-9-20
---

本文档记录我参与 OTEL 开源项目碰到的坑，碰到有趣的点


## OTLP 导出器的部分失败问题

OTLP 是 opentelemetry 中的协议传输标准，在收集到对应的metrics， trace，log 这些数据之后，我们往往会把对应的数据发送到远端，比如对应的 collector
对于整个数据传输的结果其实是存在三种状态的:

1. 失败状态: 比如因为网络问题出现的失败
2. 部分失败: 有些数据发送成功但是有些数据发送失败, 出现这种情况的原因可能是服务端拒绝了某些连接
3. 发送成功

当出现发送失败之后最先做的就是进行重试, OTLP 官方采用的重试策略是使用指数退避算法，

```
type RequestFunc func(context.Context, func(context.Context) error) error
```

如果是部分失败, 是不会去重试的, 因为如果重试的话很可能会导致出现数据覆盖的情况
```
The client MUST NOT retry the request when it receives a partial success response where the partial_success is populated.
```

下面这段代码处理的方式其实也说明了这一点, 当出现部分失败的时候是不会 return 一个 err 的
```go
return errors.Join(uploadErr, c.requestFunc(ctx, func(ctx context.Context) error {
    success = count
    resp, err := c.lsc.Export(ctx, &collogpb.ExportLogsServiceRequest{
        ResourceLogs: rl,
    })
    if resp != nil && resp.PartialSuccess != nil {
        msg := resp.PartialSuccess.GetErrorMessage()
        n := resp.PartialSuccess.GetRejectedLogRecords()
        success -= n
        if n != 0 || msg != "" {
            err := errPartial{msg: msg, n: n}
            uploadErr = errors.Join(uploadErr, err)
        }
    }
    // nil is converted to OK.
    if status.Code(err) == codes.OK {
        // Success.
        return nil
    }
    success = 0
    return err
}))
```

## otel 的 manual reader 组件

这个接口可以让我们直接去读取相对应的指标, 首先我们需要看下这个接口是如何使用的

1. 创建一个 manual reader 组件, 并传递给 meter provider
```go
reader := otel.NewManualReader()
mp := metrics.NewMeterProvider(metrics.WithReader(reader))
```
2. 使用 collect 接口去收集对应的 metric

```go
var got metricdata.ResourceMetrics
reader.Collect(ctx, &got)
```

3. 我们查看一下 got 中是否有我们期望的 metric

```go
assert.Equal(t, 1, len(got.ResourceMetrics))
```


## otel 的跨上下文传播组件


在 otel 中有一个非常神奇的接口叫做 TextMapPropagator 这个接口用来在分布式追踪系统用于跨进程传播上下文信息的核心组件。

```go
type TextMapPropagator interface {
	Inject(ctx context.Context, carrier TextMapCarrier)
	Extract(ctx context.Context, carrier TextMapCarrier) context.Context
	Fields() []string
}
```

- Inject: 将当前上下文 ctx 中的跨进程,写入到载体 carrier 中
- Extract: 从载体 carrier 中提取跨进程信息,并返回一个新的上下文
- Fields: 返回 propagator 支持的字段

下面是一段 Client/Server 代码来详细的演示这个流程:

1. 首先 client 创建根 span
2. 然后通过 inject 方法注入到追踪上下文中的 http 请求头中去
3. 服务端从请求头中拿到对应的追踪上下文
4. 在服务端创建对应的子 span
5. 返回对应的响应
6. 子 span 结束


server 端的代码:
```go
func handler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	ctx, span := tracer.Start(ctx, "server-header")
	defer span.End()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello, world"))
}
```

client的代码:

```go
func request(ctx context.Context) (err error) {
	propagator := otel.GetTextMapPropagator()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080", nil)
	if err != nil {
		return err
	}
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
```

client 创建对应的子 span:
```go
ctx, span := tracer.Start(context.Background(), "client-operation")
defer span.End()

if err := request(ctx); err != nil {
	log.Fatal(err)
}
```