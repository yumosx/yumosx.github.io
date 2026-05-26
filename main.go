package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

// Post represents a blog post
type Post struct {
	Title   string
	Date    string
	Content template.HTML
	Slug    string
	Summary string
}

// Link represents a friend link
type Link struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	Desc string `toml:"desc"`
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
	if err := g.renderLinks(site); err != nil {
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
	<script>(function(){try{var t=localStorage.getItem('blog-theme');if(t==='dark'||t==='light')document.documentElement.setAttribute('data-theme',t);else if(window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches)document.documentElement.setAttribute('data-theme','dark')}catch(e){}})();</script>
</head>
<body>
	<header>
		<h1><a href="/">{{.Site.Title}}</a></h1>
		<nav class="site-nav">
			<a href="/">首页</a>
			<a href="/links.html">友链</a>
		</nav>
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

	css, err := buildDefaultCSS()
	if err != nil {
		return fmt.Errorf("生成样式: %w", err)
	}

	themeJS := `(function() {
	var KEY = 'blog-theme';
	function setTheme(t) {
		document.documentElement.setAttribute('data-theme', t);
		try { localStorage.setItem(KEY, t); } catch(e) {}
		updateIcon(t);
	}
	function getTheme() {
		try {
			var saved = localStorage.getItem(KEY);
			if (saved === 'dark' || saved === 'light') return saved;
		} catch(e) {}
		if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) return 'dark';
		return 'light';
	}
	function updateIcon(t) {
		var btn = document.getElementById('theme-toggle');
		if (!btn) return;
		btn.textContent = t === 'dark' ? '\u2600\uFE0F' : '\uD83C\uDF19';
		btn.setAttribute('aria-label', t === 'dark' ? '切换到亮色模式' : '切换到暗黑模式');
	}
	function init() {
		var t = getTheme();
		setTheme(t);
		var btn = document.getElementById('theme-toggle');
		if (btn) btn.addEventListener('click', function() {
			var cur = document.documentElement.getAttribute('data-theme');
			setTheme(cur === 'dark' ? 'light' : 'dark');
		});
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

var (
	markdownConverter = goldmark.New(
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
	)
	externalLinkRe = regexp.MustCompile(`<a href="(https?://[^"]*)"`)
)

func convertMarkdownToHTML(markdownStr string) string {
	var buf bytes.Buffer
	if err := markdownConverter.Convert([]byte(markdownStr), &buf); err != nil {
		return markdownStr
	}
	return addExternalLinkTarget(buf.String())
}

func addExternalLinkTarget(html string) string {
	return externalLinkRe.ReplaceAllString(html, `<a href="$1" target="_blank" rel="noopener noreferrer"`)
}

func chromaStyleCSS(styleName, scope string) (string, error) {
	style := styles.Get(styleName)
	if style == nil {
		return "", fmt.Errorf("chroma style %q not found", styleName)
	}
	var buf bytes.Buffer
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return "", err
	}
	css := buf.String()
	if scope != "" {
		css = strings.ReplaceAll(css, ".chroma", scope+" .chroma")
	}
	return css, nil
}

func buildDefaultCSS() (string, error) {
	lightChroma, err := chromaStyleCSS("github", ".post-content")
	if err != nil {
		return "", err
	}
	darkChroma, err := chromaStyleCSS("github-dark", "[data-theme='dark'] .post-content")
	if err != nil {
		return "", err
	}

	return `:root {
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

.site-nav {
	display: flex;
	gap: 1.2em;
	font-size: 0.95em;
}
	.site-nav a:hover { color: var(--link-hover); }

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
	.post-content h3 { margin-top: 28px; }
	.post-content blockquote {
		margin: 1.2em 0;
		padding: 0.5em 1em;
		border-left: 4px solid var(--border);
		color: var(--text-secondary);
	}
	.post-content table {
		width: 100%;
		border-collapse: collapse;
		margin: 1.2em 0;
		font-size: 0.95em;
	}
	.post-content th,
	.post-content td {
		border: 1px solid var(--border);
		padding: 8px 12px;
		text-align: left;
	}
	.post-content th { background-color: var(--code-bg); }
	.post-content pre.chroma {
		padding: 16px;
		border-radius: 8px;
		overflow-x: auto;
		border: 1px solid var(--border);
		margin: 1.2em 0;
		font-family: 'Cascadia Code', 'JetBrains Mono', Consolas, 'Courier New', monospace;
		font-size: 0.88em;
		line-height: 1.55;
	}
	.post-content pre.chroma code {
		background: none;
		padding: 0;
		border-radius: 0;
		font-size: inherit;
	}
	.post-content :not(pre) > code {
		background-color: var(--code-bg);
		padding: 2px 6px;
		border-radius: 4px;
		font-family: 'Cascadia Code', 'JetBrains Mono', Consolas, 'Courier New', monospace;
		font-size: 0.88em;
	}
	.post-content img { max-width: 100%; height: auto; border-radius: 8px; margin: 20px 0; display: block; }

.link-list { list-style: none; padding: 0; }
	.link-list li { margin-bottom: 24px; padding-bottom: 20px; border-bottom: 1px solid var(--border); }
	.link-list a { display: block; margin-bottom: 4px; }
	.link-list a:hover { color: var(--link-hover); }
	.link-title { font-weight: bold; margin-right: 0.5em; }
	.link-url { color: var(--text-secondary); font-size: 0.9em; }
	.link-desc { display: block; color: var(--text-secondary); font-size: 0.9em; margin-top: 4px; }

` + lightChroma + "\n" + darkChroma, nil
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
	return template.FuncMap{
		"now":         time.Now,
		"extractHost": extractHostFromURL,
	}
}

func extractHostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
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

func (g *Generator) renderLinks(site *Site) error {
	fragment := filepath.Join(g.cfg.TemplatesDir, "links.html")
	tmpl, err := g.parseLayoutWithFragment(fragment)
	if err != nil {
		return err
	}

	var linksConfig struct {
		Links []Link `toml:"links"`
	}
	if _, err := toml.DecodeFile("links.toml", &linksConfig); err != nil {
		return fmt.Errorf("解析 links.toml: %w", err)
	}

	outPath := filepath.Join(g.cfg.PublicDir, "links.html")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("创建 %s: %w", outPath, err)
	}
	defer f.Close()

	ctx := map[string]interface{}{
		"Site":  site,
		"Title": "友链",
		"Links": linksConfig.Links,
	}
	if err := tmpl.Execute(f, ctx); err != nil {
		return fmt.Errorf("渲染友链页: %w", err)
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
