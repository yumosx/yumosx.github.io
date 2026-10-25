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
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// Post represents a blog post
type Post struct {
	Title     string
	Date      string
	Content   template.HTML
	Slug      string
	WordCount int
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

// Run performs full build: dirs, posts, public tree, HTML.
func (g *Generator) Run() error {
	if err := g.ensureDirs(); err != nil {
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
	sort.SliceStable(posts, func(i, j int) bool {
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
	if err := g.renderAbout(site); err != nil {
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
				post.Title = strings.TrimSpace(parts[1])
			case "Date":
				post.Date = normalizeDate(strings.TrimSpace(parts[1]))
			}
		}
	}

	if !foundFrontMatter {
		frontMatterEnd = 0
	}

	post.Slug = strings.TrimSuffix(filepath.Base(filePath), ".md")
	postContent := string(content[frontMatterEnd:])
	post.Content = template.HTML(convertMarkdownToHTML(postContent))
	post.WordCount = countWords(string(post.Content))

	return post, nil
}

var (
	markdownConverter = goldmark.New(
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("xcode"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
	)
	externalLinkRe = regexp.MustCompile(`<a href="(https?://[^"]*)"`)
	htmlTagRe      = regexp.MustCompile(`<[^>]*>`)
	cjkRe          = regexp.MustCompile(`\p{Han}`)
	wordRe         = regexp.MustCompile(`[A-Za-z0-9_]+`)
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

// countWords 统计正文字数：中日韩字符逐字算，其余按连续的英文/数字单词算，
// 口径近似 hexo-theme-yinwang 的 wordcount 助手。
func countWords(html string) int {
	text := htmlTagRe.ReplaceAllString(html, "")
	return len(cjkRe.FindAllString(text, -1)) + len(wordRe.FindAllString(text, -1))
}

// normalizeDate 把 front matter 里的日期统一成 YYYY-MM-DD，与主题的日期展示格式一致。
func normalizeDate(raw string) string {
	date := strings.SplitN(raw, " ", 2)[0] // 忽略日期后面可能跟的时分秒
	for _, layout := range []string{"2006-1-2", "2006/1/2"} {
		if t, err := time.Parse(layout, date); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
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
		"Site":   site,
		"Title":  site.Title,
		"Posts":  site.Posts,
		"IsHome": true,
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
			"IsHome":  false,
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
		"Site":   site,
		"Title":  "友链",
		"Links":  linksConfig.Links,
		"IsHome": false,
	}
	if err := tmpl.Execute(f, ctx); err != nil {
		return fmt.Errorf("渲染友链页: %w", err)
	}
	return nil
}

func (g *Generator) renderAbout(site *Site) error {
	fragment := filepath.Join(g.cfg.TemplatesDir, "about.html")
	tmpl, err := g.parseLayoutWithFragment(fragment)
	if err != nil {
		return err
	}

	outPath := filepath.Join(g.cfg.PublicDir, "about.html")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("创建 %s: %w", outPath, err)
	}
	defer f.Close()

	ctx := map[string]interface{}{
		"Site":   site,
		"Title":  "关于",
		"IsHome": false,
	}
	if err := tmpl.Execute(f, ctx); err != nil {
		return fmt.Errorf("渲染关于页: %w", err)
	}
	return nil
}

// writeGitignore 只在缺少 .gitignore 时补一份默认值，避免覆盖使用者自己维护的文件。
func (g *Generator) writeGitignore() error {
	if _, err := os.Stat(".gitignore"); err == nil {
		return nil
	}
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
