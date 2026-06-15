---
Title: GORM 高级篇 
Date: 2025-10-01
---

```go
type Plugin interface {
	Name() string
	Initialize(*DB) error
}
```

**Plugin 接口使用说明：**

`Plugin` 接口用于实现 GORM 插件，允许开发者扩展 GORM 的功能。

- **`Name() string`** - 返回插件的唯一标识名称。GORM 使用此名称来管理已注册的插件，避免重复注册。
- **`Initialize(*DB) error`** - 初始化插件逻辑。在插件注册时自动调用，接收 `*DB` 实例以便插件可以修改数据库配置、注册回调、添加 Clauses 等。返回 `nil` 表示初始化成功，返回错误会中断注册过程。

**使用示例：**

```go
type MyPlugin struct{}

func (p *MyPlugin) Name() string {
    return "myPlugin"
}

func (p *MyPlugin) Initialize(db *gorm.DB) error {
    // 注册回调、修改配置等
    db.Callback().Create().Before("gorm:create").Register("myPlugin:beforeCreate", func(db *gorm.DB) {
        // 自定义逻辑
    })
    return nil
}

// 注册插件
db.Use(&MyPlugin{})
```


```go
// ConnPool db conns pool interface
type ConnPool interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
```

**ConnPool 接口使用说明：**

`ConnPool` 接口定义了数据库连接池的核心操作方法，是 GORM 与底层 SQL 驱动交互的抽象层。

- **`PrepareContext(ctx, query) (*sql.Stmt, error)`** - 预编译 SQL 语句。返回的 `*sql.Stmt` 可重复执行，提高性能并防止 SQL 注入。适用于需要多次执行相同 SQL 的场景。
- **`ExecContext(ctx, query, args...) (sql.Result, error)`** - 执行不返回行的 SQL（如 INSERT、UPDATE、DELETE）。返回的 `sql.Result` 包含受影响的行数和最后插入的 ID。
- **`QueryContext(ctx, query, args...) (*sql.Rows, error)`** - 执行返回多行的查询（如 SELECT）。返回 `*sql.Rows` 需要调用 `Next()` 遍历，使用后必须调用 `Close()` 释放资源。
- **`QueryRowContext(ctx, query, args...) *sql.Row`** - 执行返回单行的查询。返回 `*sql.Row` 可直接调用 `Scan()` 方法获取数据，无需显式关闭。

**使用示例：**

```go
// 预编译语句
stmt, err := db.ConnPool.PrepareContext(ctx, "SELECT name FROM users WHERE id = ?")
if err != nil {
    return err
}
defer stmt.Close()

// 执行更新
result, err := db.ConnPool.ExecContext(ctx, "UPDATE users SET name = ? WHERE id = ?", "John", 1)
rowsAffected, _ := result.RowsAffected()

// 查询多行
rows, err := db.ConnPool.QueryContext(ctx, "SELECT id, name FROM users")
if err != nil {
    return err
}
defer rows.Close()
for rows.Next() {
    var id int
    var name string
    rows.Scan(&id, &name)
}

// 查询单行
var name string
err = db.ConnPool.QueryRowContext(ctx, "SELECT name FROM users WHERE id = ?", 1).Scan(&name)
```