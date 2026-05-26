---
Title: gRPC 高级篇 
Date: 2025-10-02
---

```go
type UnaryServerInterceptor func(ctx context.Context, req any, info *UnaryServerInfo, handler UnaryHandler) (resp any, err error)
```

## 参数说明

| 参数 | 说明 |
|------|------|
| `ctx` | 请求上下文，可传递元数据、取消信号等 |
| `req` | 客户端请求对象（尚未被业务处理） |
| `info` | 包含服务方法名等信息的结构体 |
| `handler` | 实际的业务处理函数，调用它将请求传递给下游 |

## 工作原理

拦截器采用 **中间件模式**，请求流程如下：

```
客户端请求 → 拦截器前置逻辑 → handler() → 业务方法 → 拦截器后置逻辑 → 响应
```

## 典型用途

- **日志记录**：记录请求/响应、耗时统计
- **认证授权**：验证 token、检查权限
- **限流熔断**：保护服务免受过载
- **错误处理**：统一异常转换和响应
- **链路追踪**：注入/提取 trace 信息

## 示例用法

```go
func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    start := time.Now()
    resp, err := handler(ctx, req)  // 调用实际业务
    log.Printf("%s 耗时: %v", info.FullMethod, time.Since(start))
    return resp, err
}

// 注册拦截器
s := grpc.NewServer(grpc.UnaryInterceptor(loggingInterceptor))
```