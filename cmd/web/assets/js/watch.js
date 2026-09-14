(function () {
    const page = document.getElementById('watch-page');
    if (!page) return;

    const player = document.getElementById('watch-player');
    const title = document.getElementById('watch-title');
    const meta = document.getElementById('watch-meta');
    const badge = document.getElementById('watch-badge');
    const download = document.getElementById('watch-download');
    const queue = document.getElementById('watch-queue');
    let videos = [];
    let currentID = page.dataset.videoId;
    let lastProgressSave = 0;

    const GRADIENT_PALETTE = [
        'from-blue-500 to-cyan-400',
        'from-red-500 to-orange-400',
        'from-emerald-500 to-green-400',
        'from-purple-500 to-fuchsia-400',
        'from-amber-500 to-yellow-500',
        'from-pink-500 to-rose-400',
        'from-cyan-500 to-sky-400',
        'from-indigo-500 to-violet-400',
    ];
    function gradientForKey(key) {
        let hash = 0;
        const str = String(key || '');
        for (let i = 0; i < str.length; i++) hash = (hash * 31 + str.charCodeAt(i)) >>> 0;
        return GRADIENT_PALETTE[hash % GRADIENT_PALETTE.length];
    }

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

    async function savePlaybackProgress(position) {
        if (!currentID || position <= lastProgressSave + 5) return;
        lastProgressSave = position;
        try {
            await fetch(`/api/file/${encodeURIComponent(currentID)}/progress`, {
                method: 'PATCH',
                credentials: 'include',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ progress: Math.floor(position) })
            });
        } catch (e) {
            console.error('Failed to save playback progress:', e);
        }
    }

    function restorePlaybackProgress() {
        const video = currentVideo();
        if (!video || !player) return;
        if (video.playback_progress && video.playback_progress > 0) {
            player.currentTime = video.playback_progress;
        }
    }

    function queueVideos() {
        const active = currentVideo();
        if (!active) return [];
        return videos.filter(video => normalFolder(video.folder) === normalFolder(active.folder));
    }

    function renderQueue() {
        const items = queueVideos();
        queue.innerHTML = items.map(video => {
            const active = video.id === currentID;
            const gradient = gradientForKey(video.id);
            return `<button type="button" data-watch-id="${escapeHtml(video.id)}" aria-current="${active ? 'true' : 'false'}" class="grid w-full grid-cols-[100px_minmax(0,1fr)] gap-3 rounded-lg p-2 text-left transition-colors ${active ? 'bg-accent' : 'hover:bg-muted'}">
                <span class="relative aspect-video overflow-hidden rounded-lg bg-gradient-to-br ${gradient} grid place-items-center text-white">
                    <img src="/api/file/${escapeHtml(video.id)}/thumb" alt="" class="absolute inset-0 h-full w-full object-cover" onerror="this.remove()" />
                    <svg class="relative h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                </span>
                <span class="min-w-0 self-center">
                    <span class="block truncate text-sm font-semibold">${escapeHtml(displayName(video.filename))}</span>
                    <span class="mt-1 block text-xs text-muted-foreground">${formatSize(video.size)} &middot; ${escapeHtml(typeBadge(video.mime_type))}</span>
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
        meta.textContent = `${formatSize(video.size)} · MediaVault ${normalFolder(video.folder)}`;
        if (badge) badge.textContent = typeBadge(video.mime_type);
        download.href = `/api/file/${encodeURIComponent(video.id)}`;
        document.title = `${displayName(video.filename)} · Media Server`;
        if (updateHistory) history.pushState({ videoID: id }, '', `/watch/${encodeURIComponent(id)}`);
        renderQueue();
        
        if (player) {
            player.addEventListener('loadedmetadata', function onLoaded() {
                player.removeEventListener('loadedmetadata', onLoaded);
                restorePlaybackProgress();
            }, { once: true });
        }
    }

    async function loadQueue() {
        try {
            const response = await fetch('/api/file/', { credentials: 'include' });
            if (!response.ok) throw new Error('Unable to load videos');
            const data = await response.json();
            videos = (data.files || []).filter(file => String(file.mime_type || '').toLowerCase().startsWith('video/'));
            if (!currentVideo()) throw new Error('Video unavailable');
            renderQueue();

            if (player) {
                player.addEventListener('loadedmetadata', function onLoaded() {
                    player.removeEventListener('loadedmetadata', onLoaded);
                    restorePlaybackProgress();
                }, { once: true });
                // metadata may already be loaded by the time this listener attaches
                // (e.g. cached video), in which case the event never fires again.
                if (player.readyState >= 1) {
                    restorePlaybackProgress();
                }
            }
        } catch (error) {
            queue.innerHTML = '<p class="px-2 py-4 text-sm text-muted-foreground">Unable to load videos. Try refreshing the page.</p>';
        }
    }

    window.addEventListener('popstate', () => {
        const id = window.location.pathname.split('/').pop();
        if (id && id !== currentID) selectVideo(id, false);
    });

    document.addEventListener('keydown', (e) => {
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
        const key = e.key.toLowerCase();

        if (e.code === 'Space' || key === 'k') {
            e.preventDefault();
            if (player.paused) {
                player.play();
            } else {
                player.pause();
            }
        } else if (key === 'f') {
            e.preventDefault();
            if (!document.fullscreenElement) {
                if (player.requestFullscreen) {
                    player.requestFullscreen();
                } else if (player.webkitRequestFullscreen) {
                    player.webkitRequestFullscreen();
                }
            } else {
                if (document.exitFullscreen) {
                    document.exitFullscreen();
                }
            }
        } else if (key === 'arrowleft' || key === 'j') {
            e.preventDefault();
            player.currentTime = Math.max(0, player.currentTime - (key === 'j' ? 10 : 5));
        } else if (key === 'arrowright' || key === 'l') {
            e.preventDefault();
            player.currentTime = Math.min(player.duration || 0, player.currentTime + (key === 'l' ? 10 : 5));
        } else if (key === 'arrowup') {
            e.preventDefault();
            player.volume = Math.min(1, player.volume + 0.1);
        } else if (key === 'arrowdown') {
            e.preventDefault();
            player.volume = Math.max(0, player.volume - 0.1);
        } else if (key === 'm') {
            e.preventDefault();
            player.muted = !player.muted;
        }
    });

    if (player) {
        player.addEventListener('timeupdate', () => {
            if (!player.paused) {
                savePlaybackProgress(player.currentTime);
            }
        });
    }

    loadQueue();
})();
