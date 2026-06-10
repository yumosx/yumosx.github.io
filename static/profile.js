(function () {
	function initCopyButton() {
		var btn = document.getElementById('copy-profile');
		if (!btn) return;

		btn.addEventListener('click', function () {
			var url = window.location.origin + '/';
			if (navigator.clipboard && navigator.clipboard.writeText) {
				navigator.clipboard.writeText(url).then(showCopied).catch(fallbackCopy);
			} else {
				fallbackCopy(url);
			}
		});

		function fallbackCopy(text) {
			var ta = document.createElement('textarea');
			ta.value = text;
			ta.style.position = 'fixed';
			ta.style.opacity = '0';
			document.body.appendChild(ta);
			ta.select();
			try {
				document.execCommand('copy');
				showCopied();
			} catch (e) {}
			document.body.removeChild(ta);
		}

		function showCopied() {
			btn.classList.add('copied');
			btn.setAttribute('aria-label', '已复制');
			setTimeout(function () {
				btn.classList.remove('copied');
				btn.setAttribute('aria-label', '复制主页链接');
			}, 2000);
		}
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', initCopyButton);
	} else {
		initCopyButton();
	}
})();
