(function () {
    const page = document.getElementById('watch-page');
    if (!page) return;

    const player = document.getElementById('watch-player');
    const title = document.getElementById('watch-title');
    const meta = document.getElementById('watch-meta');
    const download = document.getElementById('watch-download');
    const queue = document.getElementById('watch-queue');
    const queueCount = document.getElementById('watch-queue-count');
    let videos = [];
    let currentID = page.dataset.videoId;

    function escapeHtml(value) {
        return String(value || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
    }

    function displayName(filename) {
        return String(filename || '').replace(/\.[^.]+$/, '');
    }

    function formatSize(bytes) {
        if (!bytes) return '0 B';
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), sizes.length - 1);
        return (bytes / Math.pow(1024, index)).toFixed(index ? 1 : 0).replace(/\.0$/, '') + ' ' + sizes[index];
    }

    function typeBadge(mime) {
        return (String(mime).split('/')[1] || 'VIDEO').toUpperCase();
    }

    function normalFolder(folder) {
        return !folder || folder === '/' ? '/' : '/' + String(folder).replace(/^\/+|\/+$/g, '');
    }

    function currentVideo() {
        return videos.find(video => video.id === currentID);
    }

    function queueVideos() {
        const active = currentVideo();
        if (!active) return [];
        return videos.filter(video => normalFolder(video.folder) === normalFolder(active.folder));
    }

    function renderQueue() {
        const items = queueVideos();
        queueCount.textContent = items.length === 1 ? '1 video in this folder' : `${items.length} videos in this folder`;
        queue.innerHTML = items.map(video => {
            const active = video.id === currentID;
            return `<button type="button" data-watch-id="${escapeHtml(video.id)}" aria-current="${active ? 'true' : 'false'}" class="flex w-full items-center gap-3 rounded-xl p-2 text-left transition-colors ${active ? 'bg-primary/10 ring-1 ring-primary/30' : 'hover:bg-accent'}">
                <div class="relative h-14 w-24 shrink-0 overflow-hidden rounded-lg bg-muted">
                    <img src="/api/file/${escapeHtml(video.id)}/thumb" alt="" class="h-full w-full object-cover" onerror="this.style.display='none'" />
                    <svg class="absolute inset-0 m-auto h-5 w-5 text-muted-foreground" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                </div>
                <span class="min-w-0 flex-1">
                    <span class="block truncate text-sm font-medium text-slate-900 dark:text-slate-100">${escapeHtml(displayName(video.filename))}</span>
                    <span class="block pt-0.5 text-xs text-muted-foreground">${formatSize(video.size)} &middot; ${escapeHtml(typeBadge(video.mime_type))}</span>
                </span>
            </button>`;
        }).join('');

        queue.querySelectorAll('[data-watch-id]').forEach(button => {
            button.addEventListener('click', () => selectVideo(button.dataset.watchId, true));
        });
    }

    function selectVideo(id, updateHistory) {
        const video = videos.find(item => item.id === id);
        if (!video || id === currentID) return;

        currentID = id;
        player.pause();
        player.src = `/api/file/${encodeURIComponent(video.id)}`;
        player.load();
        title.textContent = displayName(video.filename);
        meta.textContent = `${formatSize(video.size)} · ${typeBadge(video.mime_type)}`;
        download.href = `/api/file/${encodeURIComponent(video.id)}`;
        document.title = `${displayName(video.filename)} · Media Server`;
        if (updateHistory) history.pushState({ videoID: id }, '', `/watch/${encodeURIComponent(id)}`);
        renderQueue();
    }

    async function loadQueue() {
        try {
            const response = await fetch('/api/file/', { credentials: 'include' });
            if (!response.ok) throw new Error('Unable to load videos');
            const data = await response.json();
            videos = (data.files || []).filter(file => String(file.mime_type || '').toLowerCase().startsWith('video/'));
            if (!currentVideo()) throw new Error('Video unavailable');
            renderQueue();
        } catch (error) {
            queueCount.textContent = 'Unable to load videos';
            queue.innerHTML = '<p class="px-2 py-4 text-sm text-muted-foreground">Try refreshing the page.</p>';
        }
    }

    window.addEventListener('popstate', () => {
        const id = window.location.pathname.split('/').pop();
        if (id && id !== currentID) selectVideo(id, false);
    });

    loadQueue();
})();
