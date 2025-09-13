#!/bin/bash

# 博客管理脚本

# 显示帮助信息
show_help() {
	cat << EOF
用法: ./blog.sh [命令]

命令:
	generate	生成静态博客
	preview		在本地预览博客
	deploy		部署博客到GitHub Pages
	new		创建新博客文章
	help		显示帮助信息
EOF
}

# 生成静态博客
generate_blog() {
	echo "正在生成博客..."
	go run main.go
	if [ $? -eq 0 ]; then
		echo "博客生成成功！静态文件位于 public 目录"
	else
		echo "博客生成失败！"
		return 1
	fi
}

# 在本地预览博客
preview_blog() {
	# 先确保博客已经生成
	if [ ! -d "public" ]; then
		echo "public 目录不存在，请先生成博客"
		generate_blog
		if [ $? -ne 0 ]; then
			return 1
		fi
	fi

	echo "正在启动本地预览服务器..."
	echo "请访问 http://localhost:8000 查看博客"
	echo "按 Ctrl+C 停止服务器"
	
	# 尝试使用Python的HTTP服务器
	if command -v python3 &> /dev/null; then
		cd public && python3 -m http.server 8000
	elif command -v python &> /dev/null; then
		cd public && python -m http.server 8000
	else
		echo "未找到Python，请手动安装并运行服务器"
		return 1
	fi
}

# 部署博客到GitHub Pages
deploy_blog() {
	# 先确保博客已经生成
	if [ ! -d "public" ]; then
		echo "public 目录不存在，请先生成博客"
		generate_blog
		if [ $? -ne 0 ]; then
			return 1
		fi
	fi

	echo "正在部署博客到GitHub Pages..."
	cd public
	
	# 初始化git仓库（如果尚未初始化）
	if [ ! -d ".git" ]; then
		git init
		git remote add origin https://github.com/yumosx/yumosx.github.io.git
	fi
	
	# 添加并提交所有更改
	git add .
	git commit -m "Update blog at $(date '+%Y-%m-%d %H:%M:%S')"
	
	# 检查当前分支
	current_branch=$(git branch --show-current)
	
	# 如果当前不是gh-pages分支，创建并切换到gh-pages分支
	if [ "$current_branch" != "gh-pages" ]; then
		git checkout -b gh-pages
	fi
	
	# 推送到远程gh-pages分支
	git push -u origin gh-pages --force
	if [ $? -eq 0 ]; then
		echo "博客部署成功！"
		echo "请在GitHub仓库设置中启用GitHub Pages，并选择 gh-pages 分支"
	else
		echo "博客部署失败！请检查您的GitHub配置"
		return 1
	fi
}

# 创建新博客文章
create_new_post() {
	read -p "请输入文章标题: " title
	
	# 生成slug
	slug=$(echo "$title" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
	
	# 获取当前日期
	date=$(date '+%Y-%m-%d')
	
	# 创建文章文件
	file_path="content/${slug}.md"
	cat > "$file_path" << EOF
---
Title: $title
Date: $date
---

# $title

在这里开始编写你的博客内容...

## 章节标题

内容...

EOF
	
	if [ $? -eq 0 ]; then
		echo "新文章已创建: $file_path"
		echo "请编辑该文件添加内容，然后运行 ./blog.sh generate 生成博客"
	else
		echo "创建文章失败！"
		return 1
	fi
}

# 主程序
case "$1" in
generate)
	generate_blog
	;;
preview)
	preview_blog
	;;
deploy)
	deploy_blog
	;;
new)
	create_new_post
	;;
help)
	show_help
	;;
*)
	echo "未知命令: $1"
	show_help
	exit 1
	;;

esac