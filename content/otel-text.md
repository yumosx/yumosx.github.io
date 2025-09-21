---
Title: opentelemetry 中的跨上下文传播
Date: 2023-9-20
---

在 otel 中有一个非常神奇的接口叫做 TextMapPropagator 这个接口用来在分布式追踪系统用于跨进程传播上下文信息的核心组件。

```
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
```
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

```
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