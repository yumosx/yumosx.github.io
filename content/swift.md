---
Title: cs193p ios 学习笔记
Date: 2026-07-01
---

学到和 swift一些语法，
- 最后一个函数参数是 lambda 表达式且没有参数的时候的时候可以直接写成 VStack{}
- some View 标识编译器自动推断，不需要手动指定类型
- @ViewBuilder 修饰的函数，返回值是 some View 类型，不需要手动指定类型
- @ViewBuilder 还可以作用到函数参数上去，比如 VStack(@ViewBuilder content: get_content)

```swift
import SwiftUI

struct ContentView: View {
    var body: some View {
        VStack(content: get_content)
    }
    
    // 使用viewbuilder 修饰对应的对应函数
    // some view 标识编译器自动推断
    @ViewBuilder
    func get_content() -> some View {
    }
}
```
