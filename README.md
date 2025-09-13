# Go静态博客生成器

这是一个使用Go语言编写的简单静态博客生成器，专为GitHub Pages设计。

## 功能特点

- 简单易用的静态博客生成系统
- 支持Markdown格式的博客文章
- 自动生成静态HTML页面
- 内置RSS订阅功能
- 响应式设计，支持移动设备
- 适合部署到GitHub Pages

## 快速开始

### 安装和使用

1. 确保你已安装Go语言环境（1.16+）

2. 克隆此仓库
```bash
git clone https://github.com/yumosx/yumosx.github.io.git
cd yumosx.github.io
```

3. 运行博客生成器
```bash
go run main.go
```

4. 生成的静态网站将位于 `public` 目录中

### 部署到GitHub Pages

1. 将 `public` 目录中的所有内容推送到GitHub仓库的 `gh-pages` 分支

```bash
cd public
git init
git add .
git commit -m "Initial commit"
git remote add origin https://github.com/yumosx/yumosx.github.io.git
# 检查当前分支
current_branch=$(git branch --show-current)
# 如果当前不是gh-pages分支，创建并切换到gh-pages分支
if [ "$current_branch" != "gh-pages" ]; then
git checkout -b gh-pages
fi
# 推送到远程gh-pages分支
git push -u origin gh-pages --force
```

2. 在GitHub仓库设置中启用GitHub Pages，并选择 `gh-pages` 分支

## 项目结构

```
├── main.go         # 主程序文件
├── go.mod          # Go模块文件
├── content/        # 博客文章（Markdown格式）
├── templates/      # HTML模板文件
├── static/         # 静态资源（CSS、图片等）
└── public/         # 生成的静态HTML文件
```

## 撰写博客文章

1. 在 `content` 目录下创建新的Markdown文件（扩展名为.md）

2. 文章格式示例：
```markdown
---
Title: 文章标题
Date: 2023-11-15
---
文章内容（支持Markdown格式）
```

3. 运行 `go run main.go` 重新生成博客

## 自定义配置

### 修改博客标题和描述

在 `main.go` 文件中修改以下内容：
```go	site := &Site{
		Title: "我的博客",
	}
```

### 自定义样式

编辑 `static/style.css` 文件来自定义博客的样式

### 修改HTML模板

编辑 `templates` 目录下的HTML模板文件来自定义页面结构

## 本地预览

在生成博客后，你可以使用任何静态文件服务器来预览网站：

```bash
cd public
python -m http.server 8000
# 或使用Go的文件服务器
# go run -tags=httpfs -mod=mod github.com/kevinburke/go-static-file-server/v2/server@latest
```

然后在浏览器中访问 http://localhost:8000

## 许可证

MIT