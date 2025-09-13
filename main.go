package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Post represents a blog post
type Post struct {
	Title      string
	Date       string
	Content    template.HTML
	Slug       string
	Summary    string
}

// Site represents the whole blog site
type Site struct {
	Title   string
	BaseURL string
	Posts   []Post
}

func main() {
	// Create necessary directories if they don't exist
	createDirIfNotExist("content")
	createDirIfNotExist("templates")
	createDirIfNotExist("public")
	createDirIfNotExist("static")

	// Create default templates
	createDefaultTemplates()

	// Create sample content
	createSampleContent()

	// Generate the blog
	site := &Site{
		Title:  "我的博客",
		BaseURL: "https://yumosx.github.io",
	}

	// Load all posts
	posts, err := loadPosts("content")
	if err != nil {
		fmt.Printf("Error loading posts: %v\n", err)
		return
	}

	// Sort posts by date (newest first)
	sort.Slice(posts, func(i, j int) bool {
		timeI, _ := time.Parse("2006-01-02", posts[i].Date)
		timeJ, _ := time.Parse("2006-01-02", posts[j].Date)
		return timeI.After(timeJ)
	})

	site.Posts = posts

	// Generate public directory structure
	generatePublicDir("public")

	// Render all pages
	renderIndex(site, "templates/index.html", "public/index.html")
	renderPosts(site, "templates/post.html", "public/posts")
	renderRSS(site, "public/feed.xml")

	// Create .gitignore file
	createGitignore()

	fmt.Println("博客生成成功！请查看 public 目录")
}

// createDirIfNotExist creates a directory if it doesn't exist
func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0755)
	}
}

// createDefaultTemplates creates the default template files
func createDefaultTemplates() {
	// Main template
	mainTmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.Site.Title}} - {{.Title}}</title>
	<link rel="stylesheet" href="/static/style.css">
	<link rel="alternate" type="application/rss+xml" href="/feed.xml" title="{{.Site.Title}} RSS Feed">
</head>
<body>
	<header>
		<h1><a href="/">{{.Site.Title}}</a></h1>
	</header>
	<main>
		{{template "content" .}}
	</main>
	<footer>
		<p>© {{now.Format "2006"}} {{.Site.Title}}</p>
	</footer>
</body>
</html>`

	// Index template
	indexTmpl := `{{define "content"}}
	<h2>博客文章</h2>
	<ul class="post-list">
	{{range .Posts}}
		<li>
			<a href="/posts/{{.Slug}}.html">{{.Title}}</a>
			<span class="post-date">{{.Date}}</span>
			<p>{{.Summary}}</p>
		</li>
	{{end}}
	</ul>
{{end}}`

	// Post template
	postTmpl := `{{define "content"}}
	<article class="post">
		<h2>{{.Title}}</h2>
		<div class="post-meta">{{.Date}}</div>
		<div class="post-content">{{.Content}}</div>
	</article>
{{end}}`

	ioutil.WriteFile("templates/main.html", []byte(mainTmpl), 0644)
	ioutil.WriteFile("templates/index.html", []byte(indexTmpl), 0644)
	ioutil.WriteFile("templates/post.html", []byte(postTmpl), 0644)

	// Create CSS file
	css := `body {
	font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
	line-height: 1.6;
	color: #333;
	max-width: 800px;
	margin: 0 auto;
	padding: 20px;
}

header {
	border-bottom: 1px solid #eee;
	padding-bottom: 20px;
	margin-bottom: 30px;
}
	h1 {
		margin: 0;
	}

	a {
		color: #333;
		text-decoration: none;
	}

main {
	margin-bottom: 40px;
}

footer {
	border-top: 1px solid #eee;
	padding-top: 20px;
	text-align: center;
	color: #777;
}

.post-list {
	list-style: none;
	padding: 0;
}

	li {
		margin-bottom: 30px;
		padding-bottom: 20px;
		border-bottom: 1px solid #eee;
	}

		a {
			font-size: 1.2em;
			font-weight: bold;
			display: block;
			margin-bottom: 5px;
		}

			a:hover {
				color: #007acc;
			}

	.post-date {
		display: block;
		color: #777;
		font-size: 0.9em;
		margin-bottom: 10px;
	}

.post {
	margin-bottom: 40px;
}

	.post-meta {
		color: #777;
		margin-bottom: 20px;
	}

	.post-content {
		time: 1.8;
	}

	.post-content h2 {
		margin-top: 40px;
	}

	.post-content pre {
		background-color: #f5f5f5;
		padding: 15px;
		border-radius: 5px;
		overflow-x: auto;
	}

	.post-content code {
		background-color: #f5f5f5;
		padding: 2px 5px;
		border-radius: 3px;
	}`

	ioutil.WriteFile("static/style.css", []byte(css), 0644)
}

// createSampleContent creates sample blog posts
func createSampleContent() {
	post1 := `---
Title: 第一篇博客文章
Date: 2023-11-15
---
这是我的第一篇博客文章。

欢迎来到我的博客！这是一个使用Go语言搭建的简单博客系统，适合在GitHub Pages上托管。

## 功能特点

- 简单易用
- 生成静态HTML
- 支持Markdown
- 自动生成RSS订阅源

希望你喜欢！`

	post2 := `---
Title: Go语言入门
Date: 2023-11-10
---
# Go语言入门指南

Go是一种开源的编程语言，它能让构造简单、可靠且高效的软件变得容易。

## 为什么选择Go

- 简洁的语法
- 强大的并发支持
- 快速的编译速度
- 优秀的性能

## 开始学习Go

1. 安装Go开发环境
2. 阅读官方教程
3. 编写第一个程序
4. 实践中学习

如果你想了解更多关于Go语言的信息，可以访问[Go官方网站](https://golang.org)。`

	ioutil.WriteFile("content/first-post.md", []byte(post1), 0644)
	ioutil.WriteFile("content/go-intro.md", []byte(post2), 0644)
}

// loadPosts loads all blog posts from the content directory
func loadPosts(dir string) ([]Post, error) {
	var posts []Post

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".md" {
			continue
		}

		post, err := parsePost(filepath.Join(dir, file.Name()))
		if err != nil {
			fmt.Printf("Error parsing post %s: %v\n", file.Name(), err)
			continue
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// parsePost parses a markdown file into a Post struct
func parsePost(filePath string) (Post, error) {
	var post Post

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return post, err
	}

	// Read front matter
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inFrontMatter := false
	frontMatterEnd := 0

	for scanner.Scan() {
		line := scanner.Text()
		frontMatterEnd += len(line) + 1 // +1 for newline

		if line == "---" {
			if !inFrontMatter {
				inFrontMatter = true
			} else {
				inFrontMatter = false
				break
			}
			continue
		}

		if inFrontMatter {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				switch parts[0] {
				case "Title":
					post.Title = parts[1]
				case "Date":
					post.Date = parts[1]
				}
			}
		}
	}

	// Generate slug from filename
	post.Slug = strings.TrimSuffix(filepath.Base(filePath), ".md")

	// Process content (simplified HTML conversion)
	postContent := string(content[frontMatterEnd:])

	// Simple markdown to HTML conversion
	htmlContent := convertMarkdownToHTML(postContent)
	post.Content = template.HTML(htmlContent)

	// Extract summary
	post.Summary = extractSummary(postContent)

	return post, nil
}

// convertMarkdownToHTML converts simple markdown to HTML
func convertMarkdownToHTML(markdown string) string {
	// This is a simplified conversion for demonstration
	// In a real-world scenario, you would use a proper markdown library

	var result strings.Builder

	for _, line := range strings.Split(markdown, "\n") {
		// Headers
		if strings.HasPrefix(line, "# ") {
			result.WriteString(fmt.Sprintf("<h1>%s</h1>\n", strings.TrimPrefix(line, "# ")))
		} else if strings.HasPrefix(line, "## ") {
			result.WriteString(fmt.Sprintf("<h2>%s</h2>\n", strings.TrimPrefix(line, "## ")))
		} else if strings.HasPrefix(line, "### ") {
			result.WriteString(fmt.Sprintf("<h3>%s</h3>\n", strings.TrimPrefix(line, "### ")))
		} else {
			// Paragraphs
			if len(line) > 0 {
				result.WriteString(fmt.Sprintf("<p>%s</p>\n", line))
			} else {
				result.WriteString("<br>\n")
			}
		}
	}

	return result.String()
}

// extractSummary extracts a summary from the post content
func extractSummary(content string) string {
	// Simple summary extraction
	lines := strings.Split(content, "\n")
	var summary strings.Builder

	for i, line := range lines {
		if i > 5 { // Take first 5 lines as summary
			break
		}
		if strings.HasPrefix(line, "#") {
			continue // Skip headers
		}
		if len(line) > 0 {
			summary.WriteString(line)
			summary.WriteString(" ")
		}
	}

	summaryStr := summary.String()
	if len(summaryStr) > 150 {
		summaryStr = summaryStr[:150] + "..."
	}

	return summaryStr
}

// generatePublicDir creates the public directory structure
func generatePublicDir(dir string) {
	createDirIfNotExist(dir)
	createDirIfNotExist(filepath.Join(dir, "posts"))
	createDirIfNotExist(filepath.Join(dir, "static"))

	// Copy static files
	staticFiles, _ := ioutil.ReadDir("static")
	for _, file := range staticFiles {
		src := filepath.Join("static", file.Name())
		dst := filepath.Join(dir, "static", file.Name())
		content, _ := ioutil.ReadFile(src)
		ioutil.WriteFile(dst, content, 0644)
	}
}

// renderIndex renders the index page
func renderIndex(site *Site, templateFile, outputFile string) {
	// Read template files
	mainContent, err := ioutil.ReadFile("templates/main.html")
	if err != nil {
		fmt.Printf("Error reading main template: %v\n", err)
		return
	}

	indexContent, err := ioutil.ReadFile(templateFile)
	if err != nil {
		fmt.Printf("Error reading index template: %v\n", err)
		return
	}

	// Combine templates
	combinedTemplate := string(mainContent) + string(indexContent)

	tmpl, err := template.New("blog").Funcs(template.FuncMap{
		"now": time.Now,
	}).Parse(combinedTemplate)
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		return
	}

	output, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer output.Close()

	// Pass a merged context including site and posts
	context := map[string]interface{}{
		"Site":  site,
		"Title": site.Title,
		"Posts": site.Posts,
	}

	err = tmpl.Execute(output, context)
	if err != nil {
		fmt.Printf("Error executing template: %v\n", err)
	}
}

// renderPosts renders each post as a separate HTML file
func renderPosts(site *Site, templateFile, outputDir string) {
	// Read template files
	mainContent, err := ioutil.ReadFile("templates/main.html")
	if err != nil {
		fmt.Printf("Error reading main template: %v\n", err)
		return
	}

	postContent, err := ioutil.ReadFile(templateFile)
	if err != nil {
		fmt.Printf("Error reading post template: %v\n", err)
		return
	}

	// Combine templates
	combinedTemplate := string(mainContent) + string(postContent)

	for _, post := range site.Posts {
		tmpl, err := template.New("blog").Funcs(template.FuncMap{
			"now": time.Now,
		}).Parse(combinedTemplate)
		if err != nil {
			fmt.Printf("Error parsing template: %v\n", err)
			continue
		}

		outputFile := filepath.Join(outputDir, post.Slug+".html")
		output, err := os.Create(outputFile)
		if err != nil {
			fmt.Printf("Error creating output file: %v\n", err)
			continue
		}
		defer output.Close()

		// Pass a merged context including site and post data
		context := map[string]interface{}{
			"Site":   site,
			"Title":  post.Title,
			"Date":   post.Date,
			"Content": post.Content,
		}

		err = tmpl.Execute(output, context)
		if err != nil {
			fmt.Printf("Error executing template for post %s: %v\n", post.Title, err)
		}
	}
}

// renderRSS generates an RSS feed for the blog
func renderRSS(site *Site, outputFile string) {
	var rss strings.Builder

	rss.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
`)
	rss.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
`)
	rss.WriteString(`<channel>
`)
	rss.WriteString(fmt.Sprintf(`<title>%s</title>
`, site.Title))
	rss.WriteString(fmt.Sprintf(`<link>%s</link>
`, site.BaseURL))
	rss.WriteString(`<description>我的博客</description>
`)
	rss.WriteString(fmt.Sprintf(`<atom:link href="%s/feed.xml" rel="self" type="application/rss+xml"/>
`, site.BaseURL))

	for _, post := range site.Posts {
		rss.WriteString(`<item>
`)
		rss.WriteString(fmt.Sprintf(`<title>%s</title>
`, post.Title))
		rss.WriteString(fmt.Sprintf(`<link>%s/posts/%s.html</link>
`, site.BaseURL, post.Slug))
		rss.WriteString(fmt.Sprintf(`<description>%s</description>
`, post.Summary))
		rss.WriteString(fmt.Sprintf(`<pubDate>%s</pubDate>
`, formatRSSDate(post.Date)))
		rss.WriteString(fmt.Sprintf(`<guid>%s/posts/%s.html</guid>
`, site.BaseURL, post.Slug))
		rss.WriteString(`</item>
`)
	}

	rss.WriteString(`</channel>
`)
	rss.WriteString(`</rss>`)

	ioutil.WriteFile(outputFile, []byte(rss.String()), 0644)
}

// formatRSSDate formats a date string for RSS feed
func formatRSSDate(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Now().Format(time.RFC1123)
	}
	return t.Format(time.RFC1123)
}

// createGitignore creates a .gitignore file
func createGitignore() {
	gitignore := `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Build output
public/

# Environment variables
.env
.env.local

# Editor directories and files
.vscode/
.idea/
*.swp
*.swo
*~
`

	ioutil.WriteFile(".gitignore", []byte(gitignore), 0644)
}