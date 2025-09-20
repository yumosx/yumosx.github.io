# Go静态博客生成器

## 概述

这是一个使用Go语言编写的静态博客生成器，专为GitHub Pages设计。它将Markdown格式的文章转换为静态HTML页面，无需数据库支持，适合个人博客和文档网站的快速部署。

## 功能

- Markdown文章转HTML静态页面
- 自动生成RSS订阅源
- 响应式设计，适配各种设备
- 支持自定义样式和模板
- 一键生成完整静态网站

## 安装

### 前提条件

- Go语言环境 (1.16或更高版本)
- Git（用于部署到GitHub Pages）

### 获取源码

```bash
git clone https://github.com/yumosx/yumosx.github.io.git
cd yumosx.github.io
```

## 使用

### 生成博客

在项目根目录执行以下命令生成静态博客：

```bash
go run main.go
```

生成的静态网站文件将保存在 `public` 目录中。

### 撰写文章

1. 在 `content` 目录下创建新的Markdown文件（扩展名为.md）
2. 按照以下格式编写文章：

```markdown
---
Title: 文章标题
Date: YYYY-MM-DD
---
文章内容（支持标准Markdown语法）
```

3. 重新运行 `go run main.go` 生成包含新文章的博客

## 配置

### 修改网站信息

编辑 `main.go` 文件中的以下部分以修改博客标题和基础URL：

```go
site := &Site{
	Title:   "yumosx's 博客",
	BaseURL: "https://yumosx.github.io",
}
```

### 自定义样式

编辑 `static/style.css` 文件可自定义博客的外观样式。主要样式区域包括：
- 全局样式（字体、颜色、布局）
- 文章列表样式（.post-list）
- 单篇文章样式（.post-content）
- 代码块样式（pre, code）

### 修改HTML模板

编辑 `templates` 目录下的以下文件可自定义页面结构：
- `main.html`: 网站主模板，包含整体布局
- `index.html`: 首页模板，显示文章列表
- `post.html`: 文章详情页模板

## 架构

### 核心流程

1. **初始化阶段**：创建必要目录并生成默认模板
2. **内容加载**：从content目录读取Markdown文章
3. **文章解析**：解析文章元数据和内容，转换Markdown为HTML
4. **页面生成**：使用模板渲染生成HTML文件
5. **资源复制**：将static目录下的静态资源复制到public目录

### 目录结构

```
├── main.go         # 主程序文件，包含所有核心功能
├── content/        # Markdown格式的博客文章
├── templates/      # HTML模板文件
├── static/         # 静态资源（CSS文件等）
└── public/         # 生成的静态网站文件（自动生成）
```

### 主要功能模块

- **模板系统**：使用Go的html/template包实现页面渲染
- **Markdown解析**：使用blackfriday库将Markdown转换为HTML
- **文件系统操作**：处理文件读写和目录创建
- **RSS生成**：创建符合标准的RSS订阅源

## 本地预览

在生成博客后，可以使用任意静态文件服务器预览网站：

```bash
cd public
python -m http.server 8000
```

然后在浏览器中访问 http://localhost:8000 查看效果。

## 部署到GitHub Pages

### 自动部署脚本

使用项目根目录下的 `blog.sh` 脚本可一键生成并部署博客：

```bash
chmod +x blog.sh
./blog.sh
```

### 手动部署

1. 生成博客（如前所述）
2. 进入 `public` 目录并推送到GitHub：

```bash
cd public
git init
git add .
git commit -m "Update blog content"
git remote add origin https://github.com/yumosx/yumosx.github.io.git

# 检查当前分支
current_branch=$(git branch --show-current)

# 切换到gh-pages分支（如果不存在则创建）
if [ "$current_branch" != "gh-pages" ]; then
git checkout -b gh-pages
fi

# 推送到远程仓库
git push -u origin gh-pages --force
```

3. 在GitHub仓库设置中，启用GitHub Pages并选择 `gh-pages` 分支

## 许可证

MIT