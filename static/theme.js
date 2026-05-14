(function() {
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
})();