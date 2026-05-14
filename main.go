package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/russross/blackfriday/v2"
)

// Post represents a blog post
type Post struct {
	Title   string
	Date    string
	Content template.HTML
	Slug    string
	Summary string
}

// Site represents the whole blog site
type Site struct {
	Title   string
	BaseURL string
	Posts   []Post
}

// Config holds all paths and site metadata so nothing scatters magic strings.
type Config struct {
	SiteTitle    string
	BaseURL      string
	ContentDir   string
	TemplatesDir string
	PublicDir    string
	StaticDir    string
}

func defaultConfig() Config {
	return Config{
		SiteTitle:    "yumosx's 写字的地方",
		BaseURL:      "https://yumosx.github.io",
		ContentDir:   "content",
		TemplatesDir: "templates",
		PublicDir:    "public",
		StaticDir:    "static",
	}
}

// Generator wires filesystem layout, parsing, and rendering.
type Generator struct {
	cfg Config
}

func NewGenerator(cfg Config) *Generator {
	return &Generator{cfg: cfg}
}

func main() {
	g := NewGenerator(defaultConfig())
	if err := g.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "生成失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("博客生成成功！请查看 public 目录")
}

// Run performs full build: dirs, optional defaults, posts, public tree, HTML.
func (g *Generator) Run() error {
	if err := g.ensureDirs(); err != nil {
		return err
	}
	if err := g.writeDefaultAssets(); err != nil {
		return err
	}

	site := &Site{
		Title:   g.cfg.SiteTitle,
		BaseURL: g.cfg.BaseURL,
	}
	posts, err := g.loadPosts()
	if err != nil {
		return fmt.Errorf("加载文章: %w", err)
	}
	sort.Slice(posts, func(i, j int) bool {
		timeI, _ := time.Parse("2006-01-02", posts[i].Date)
		timeJ, _ := time.Parse("2006-01-02", posts[j].Date)
		return timeI.After(timeJ)
	})
	site.Posts = posts

	if err := g.preparePublicDir(); err != nil {
		return err
	}
	if err := g.renderIndex(site); err != nil {
		return err
	}
	if err := g.renderPosts(site); err != nil {
		return err
	}
	if err := g.writeGitignore(); err != nil {
		return err
	}
	return nil
}

func (g *Generator) ensureDirs() error {
	for _, dir := range []string{
		g.cfg.ContentDir,
		g.cfg.TemplatesDir,
		g.cfg.PublicDir,
		g.cfg.StaticDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", dir, err)
		}
	}
	return nil
}

func (g *Generator) writeDefaultAssets() error {
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
		<button id="theme-toggle" class="theme-toggle" aria-label="切换主题">🌙</button>
	</header>
	<main>
		{{template "content" .}}
	</main>
	<footer>
		<p>© {{now.Format "2006"}} {{.Site.Title}}</p>
	</footer>
	<script src="/static/theme.js"></script>
</body>
</html>`

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

	postTmpl := `{{define "content"}}
	<article class="post">
		<h2>{{.Title}}</h2>
		<div class="post-meta">{{.Date}}</div>
		<div class="post-content">{{.Content}}</div>
	</article>
{{end}}`

	css := `:root {
	--bg: #ffffff;
	--text: #333333;
	--text-secondary: #777777;
	--border: #eeeeee;
	--link: #333333;
	--link-hover: #007acc;
	--code-bg: #f5f5f5;
}

[data-theme='dark'] {
	--bg: #1a1a2e;
	--text: #e0e0e0;
	--text-secondary: #999999;
	--border: #2d2d44;
	--link: #64b5f6;
	--link-hover: #90caf9;
	--code-bg: #252540;
}

body {
	font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
	line-height: 1.6;
	color: var(--text);
	background-color: var(--bg);
	max-width: 800px;
	margin: 0 auto;
	padding: 20px;
	transition: background-color 0.3s, color 0.3s;
}

header {
	border-bottom: 1px solid var(--border);
	padding-bottom: 20px;
	margin-bottom: 30px;
	display: flex;
	justify-content: space-between;
	align-items: center;
}
	h1 { margin: 0; }

	.theme-toggle {
		background: none; border: none; font-size: 1.4em;
		cursor: pointer; padding: 4px 8px; border-radius: 6px;
		transition: background-color 0.2s;
	}
		.theme-toggle:hover { background-color: var(--border); }

a { color: var(--link); text-decoration: none; }
main { margin-bottom: 40px; }
footer { border-top: 1px solid var(--border); padding-top: 20px; text-align: center; color: var(--text-secondary); }

.post-list { list-style: none; padding: 0; }
	.post-list li { margin-bottom: 30px; padding-bottom: 20px; border-bottom: 1px solid var(--border); }
	.post-list a { font-size: 1.2em; font-weight: bold; display: block; margin-bottom: 5px; }
	.post-list a:hover { color: var(--link-hover); }
	.post-date { display: block; color: var(--text-secondary); font-size: 0.9em; margin-bottom: 10px; }

.post { margin-bottom: 40px; }
	.post-meta { color: var(--text-secondary); margin-bottom: 20px; }
	.post-content { line-height: 1.8; }
	.post-content h2 { margin-top: 40px; }
	.post-content pre { background-color: var(--code-bg); padding: 15px; border-radius: 5px; overflow-x: auto; }
	.post-content code { background-color: var(--code-bg); padding: 2px 5px; border-radius: 3px; }
	.post-content img { max-width: 100%; height: auto; border-radius: 8px; margin: 20px 0; display: block; }`

	themeJS := `(function() {
	var KEY = 'blog-theme';
	function setTheme(t) {
		document.documentElement.setAttribute('data-theme', t);
		localStorage.setItem(KEY, t);
		updateIcon(t);
	}
	function getTheme() {
		var saved = localStorage.getItem(KEY);
		if (saved) return saved;
		return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
	}
	function updateIcon(t) {
		var btn = document.getElementById('theme-toggle');
		if (!btn) return;
		btn.textContent = t === 'dark' ? '☀️' : '🌙';
		btn.setAttribute('aria-label', t === 'dark' ? '切换到亮色模式' : '切换到暗黑模式');
	}
	function init() {
		var t = getTheme();
		setTheme(t);
		var btn = document.getElementById('theme-toggle');
		if (btn) {
			btn.addEventListener('click', function() {
				setTheme(document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
			});
		}
	}
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', init);
	} else {
		init();
	}
})();`

	paths := map[string]string{
		filepath.Join(g.cfg.TemplatesDir, "main.html"):  mainTmpl,
		filepath.Join(g.cfg.TemplatesDir, "index.html"): indexTmpl,
		filepath.Join(g.cfg.TemplatesDir, "post.html"):  postTmpl,
		filepath.Join(g.cfg.StaticDir, "style.css"):     css,
		filepath.Join(g.cfg.StaticDir, "theme.js"):      themeJS,
	}
	for path, body := range paths {
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return fmt.Errorf("写入 %s: %w", path, err)
		}
	}
	return nil
}

func (g *Generator) loadPosts() ([]Post, error) {
	files, err := os.ReadDir(g.cfg.ContentDir)
	if err != nil {
		return nil, err
	}

	var posts []Post
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".md" {
			continue
		}
		path := filepath.Join(g.cfg.ContentDir, file.Name())
		post, err := parsePost(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "解析文章 %s: %v\n", file.Name(), err)
			continue
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func parsePost(filePath string) (Post, error) {
	var post Post

	content, err := os.ReadFile(filePath)
	if err != nil {
		return post, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	inFrontMatter := false
	frontMatterEnd := 0
	foundFrontMatter := false

	for scanner.Scan() {
		line := scanner.Text()
		frontMatterEnd += len(line) + 1

		if line == "---" {
			if !inFrontMatter {
				inFrontMatter = true
				foundFrontMatter = true
			} else {
				inFrontMatter = false
				break
			}
			continue
		}

		if inFrontMatter {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "Title":
				post.Title = parts[1]
			case "Date":
				post.Date = parts[1]
			}
		}
	}

	if !foundFrontMatter {
		frontMatterEnd = 0
	}

	post.Slug = strings.TrimSuffix(filepath.Base(filePath), ".md")
	postContent := string(content[frontMatterEnd:])
	post.Content = template.HTML(convertMarkdownToHTML(postContent))
	post.Summary = extractSummary(postContent)

	return post, nil
}

func convertMarkdownToHTML(markdownStr string) string {
	extensions := blackfriday.WithExtensions(blackfriday.CommonExtensions | blackfriday.AutoHeadingIDs | blackfriday.FencedCode)
	renderer := blackfriday.WithRenderer(blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.CommonHTMLFlags | blackfriday.HrefTargetBlank,
	}))
	return string(blackfriday.Run([]byte(markdownStr), extensions, renderer))
}

func extractSummary(content string) string {
	lines := strings.Split(content, "\n")
	var summary strings.Builder

	for i, line := range lines {
		if i > 5 {
			break
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "```") {
			continue
		}
		if len(line) > 0 {
			summary.WriteString(line)
			summary.WriteString(" ")
		}
	}

	summaryStr := summary.String()
	if len([]rune(summaryStr)) > 150 {
		runes := []rune(summaryStr)
		summaryStr = string(runes[:150]) + "..."
	}
	return summaryStr
}

func (g *Generator) preparePublicDir() error {
	pub := g.cfg.PublicDir
	if err := os.MkdirAll(filepath.Join(pub, "posts"), 0755); err != nil {
		return err
	}
	if err := copyDir(g.cfg.StaticDir, filepath.Join(pub, "static")); err != nil {
		return fmt.Errorf("复制静态资源: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0644)
	})
}

func (g *Generator) blogTemplateFuncs() template.FuncMap {
	return template.FuncMap{"now": time.Now}
}

func (g *Generator) parseLayoutWithFragment(fragmentPath string) (*template.Template, error) {
	mainPath := filepath.Join(g.cfg.TemplatesDir, "main.html")
	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		return nil, fmt.Errorf("读取主模板 %s: %w", mainPath, err)
	}
	fragContent, err := os.ReadFile(fragmentPath)
	if err != nil {
		return nil, fmt.Errorf("读取片段 %s: %w", fragmentPath, err)
	}
	combined := string(mainContent) + string(fragContent)
	tmpl, err := template.New("blog").Funcs(g.blogTemplateFuncs()).Parse(combined)
	if err != nil {
		return nil, fmt.Errorf("解析模板: %w", err)
	}
	return tmpl, nil
}

func (g *Generator) renderIndex(site *Site) error {
	fragment := filepath.Join(g.cfg.TemplatesDir, "index.html")
	tmpl, err := g.parseLayoutWithFragment(fragment)
	if err != nil {
		return err
	}
	outPath := filepath.Join(g.cfg.PublicDir, "index.html")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("创建 %s: %w", outPath, err)
	}
	defer f.Close()

	ctx := map[string]interface{}{
		"Site":  site,
		"Title": site.Title,
		"Posts": site.Posts,
	}
	if err := tmpl.Execute(f, ctx); err != nil {
		return fmt.Errorf("渲染首页: %w", err)
	}
	return nil
}

func (g *Generator) renderPosts(site *Site) error {
	fragment := filepath.Join(g.cfg.TemplatesDir, "post.html")
	tmpl, err := g.parseLayoutWithFragment(fragment)
	if err != nil {
		return err
	}

	outDir := filepath.Join(g.cfg.PublicDir, "posts")
	for _, post := range site.Posts {
		outPath := filepath.Join(outDir, post.Slug+".html")
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("创建 %s: %w", outPath, err)
		}
		ctx := map[string]interface{}{
			"Site":    site,
			"Title":   post.Title,
			"Date":    post.Date,
			"Content": post.Content,
		}
		if err := tmpl.Execute(f, ctx); err != nil {
			f.Close()
			return fmt.Errorf("渲染文章 %s: %w", post.Slug, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("关闭 %s: %w", outPath, err)
		}
	}
	return nil
}

func (g *Generator) writeGitignore() error {
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
	return os.WriteFile(".gitignore", []byte(gitignore), 0644)
}
