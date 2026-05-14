(function() {
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
})();