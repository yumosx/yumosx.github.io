---
Title: Gopher Lua 编写
Date: 2025-10-02
---

## 基本使用

```go
package main

import lua "github.com/yuin/gopher-lua"

func main() {
	L := lua.NewState()
	defer L.Close()

	// 执行 Lua 字符串
	if err := L.DoString(`print("Hello from Lua!")`); err != nil {
		panic(err)
	}

	// 执行 Lua 文件
	// if err := L.DoFile("script.lua"); err != nil { ... }
}
```

## 设置全局变量；L.SetGlobal()


```go
// Go 设置全局变量
L.SetGlobal("name", lua.LString("gopher"))
L.SetGlobal("age", lua.LNumber(5))

// 在 Lua 中使用
L.DoString(`print(name, age)`)

// Go 获取 Lua 全局变量
L.DoString(`version = "1.0"`)
ver := L.GetGlobal("version")
if str, ok := ver.(lua.LString); ok {
    fmt.Println("Version:", string(str)) // 输出: 1.0
}
```

## 设置函数 L.NewFunction()

```go
// 定义一个加法函数
L.SetGlobal("add", L.NewFunction(func(L *lua.LState) int {
    a := L.CheckNumber(1) // 第一个参数
    b := L.CheckNumber(2) // 第二个参数
    L.Push(lua.LNumber(a + b))
    return 1 // 返回值个数
}))

// Lua 中调用
L.DoString(`print(add(10, 20))`) // 输出 30
```

## 设置表和调用表

```go
tbl := L.NewTable()
L.SetField(tbl, "x", lua.LNumber(100))
L.SetField(tbl, "y", lua.LNumber(200))
// 也可以像数组一样
tbl.RawSetInt(1, lua.LString("first"))
tbl.RawSetInt(2, lua.LString("second"))

L.SetGlobal("point", tbl)
L.DoString(`print(point.x, point.y, point[1], point[2])`)
```

## 将 Go 数据传递给 Lua

```go
type Person struct {
    Name string
    Age  int
}

// 注册一个构造器
L.SetGlobal("newPerson", L.NewFunction(func(L *lua.LState) int {
    name := L.CheckString(1)
    age  := L.CheckInt(2)
    p := &Person{Name: name, Age: age}
    
    ud := L.NewUserData()
    ud.Value = p     // 存储 Go 对象
    L.SetMetatable(ud, L.GetTypeMetatable("person")) // 设置元表
    L.Push(ud)
    return 1
}))

// 定义元方法 __index，让 userdata 支持属性访问
mt := L.NewTypeMetatable("person")
L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
    p := L.CheckUserData(1).Value.(*Person)
    key := L.CheckString(2)
    switch key {
    case "name":
        L.Push(lua.LString(p.Name))
    case "age":
        L.Push(lua.LNumber(p.Age))
    default:
        L.Push(lua.LNil)
    }
    return 1
}))

// Lua 中即可使用
L.DoString(`
    local p = newPerson("Alice", 30)
    print(p.name, p.age)
`)
```