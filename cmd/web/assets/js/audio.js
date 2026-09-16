(function () {
    const page = document.getElementById('audio-page');
    if (!page) return;

    const title = document.getElementById('audio-title');
    const meta = document.getElementById('audio-meta');
    const badge = document.getElementById('audio-badge');
    const download = document.getElementById('audio-download');
    const queue = document.getElementById('audio-queue');
    const btnPlay = document.getElementById('btn-play');
    const iconPlay = document.getElementById('icon-play');
    const iconPause = document.getElementById('icon-pause');
    const btnBackward = document.getElementById('btn-backward');
    const btnForward = document.getElementById('btn-forward');
    const btnMute = document.getElementById('btn-mute');
    const iconVolume = document.getElementById('icon-volume');
    const iconMuted = document.getElementById('icon-muted');
    const volumeSlider = document.getElementById('volume-slider');
    const currentTimeEl = document.getElementById('current-time');
    const totalDurationEl = document.getElementById('total-duration');
    const waveformLoading = document.getElementById('waveform-loading');

    let audios = [];
    let currentID = page.dataset.audioId;
    let lastProgressSave = 0;
    let wavesurfer = null;
    let isReady = false;

    function formatTime(seconds) {
        if (isNaN(seconds) || seconds < 0) return '00:00';
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
    }

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
        return (String(mime).split('/')[1] || 'AUDIO').toUpperCase();
    }

    function normalFolder(folder) {
        return !folder || folder === '/' ? '/' : '/' + String(folder).replace(/^\/+|\/+$/g, '');
    }

    function currentAudio() {
        return audios.find(audio => audio.id === currentID);
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

    async function initWavesurfer() {
        if (wavesurfer) {
            wavesurfer.destroy();
        }

        const isDark = document.documentElement.classList.contains('dark');
        const waveColor = isDark ? '#475569' : '#cbd5e1';
        const progressColor = isDark ? '#6366f1' : '#4f46e5';

        wavesurfer = WaveSurfer.create({
            container: '#waveform',
            waveColor: waveColor,
            progressColor: progressColor,
            cursorColor: isDark ? '#818cf8' : '#4338ca',
            barWidth: 3,
            barGap: 3,
            barRadius: 3,
            height: 80,
            normalize: true
        });

        isReady = false;
        if (waveformLoading) {
            waveformLoading.style.opacity = '1';
            waveformLoading.textContent = 'Loading waveform...';
        }

        try {
            const res = await fetch(`/api/file/${encodeURIComponent(currentID)}`, { credentials: 'include' });
            if (!res.ok) throw new Error('Failed to load audio file');
            const blob = await res.blob();
            const blobUrl = URL.createObjectURL(blob);
            wavesurfer.load(blobUrl);
        } catch (err) {
            console.error('Failed to fetch audio file:', err);
            if (waveformLoading) waveformLoading.textContent = 'Failed to load audio waveform';
        }

        wavesurfer.on('ready', () => {
            isReady = true;
            if (waveformLoading) waveformLoading.style.opacity = '0';
            totalDurationEl.textContent = formatTime(wavesurfer.getDuration());

            const active = currentAudio();
            if (active && active.playback_progress && active.playback_progress > 0) {
                wavesurfer.setTime(active.playback_progress);
            }
        });

        wavesurfer.on('audioprocess', (time) => {
            currentTimeEl.textContent = formatTime(time);
            if (wavesurfer.isPlaying()) {
                savePlaybackProgress(time);
            }
        });

        wavesurfer.on('timeupdate', (time) => {
            currentTimeEl.textContent = formatTime(time);
            if (wavesurfer.isPlaying()) {
                savePlaybackProgress(time);
            }
        });

        wavesurfer.on('play', () => {
            iconPlay.classList.add('hidden');
            iconPause.classList.remove('hidden');
        });

        wavesurfer.on('pause', () => {
            iconPlay.classList.remove('hidden');
            iconPause.classList.add('hidden');
        });

        wavesurfer.on('finish', () => {
            iconPlay.classList.remove('hidden');
            iconPause.classList.add('hidden');
        });
    }

    function queueAudios() {
        const active = currentAudio();
        if (!active) return [];
        return audios.filter(audio => normalFolder(audio.folder) === normalFolder(active.folder));
    }

    function renderQueue() {
        const items = queueAudios();
        queue.innerHTML = items.map(audio => {
            const active = audio.id === currentID;
            const gradient = gradientForKey(audio.id);
            return `<button type="button" data-audio-id="${escapeHtml(audio.id)}" aria-current="${active ? 'true' : 'false'}" class="grid w-full grid-cols-[60px_minmax(0,1fr)] gap-3 rounded-lg p-2 text-left transition-colors ${active ? 'bg-accent' : 'hover:bg-muted'}">
                <span class="relative aspect-square overflow-hidden rounded-lg bg-gradient-to-br ${gradient} grid place-items-center text-white shadow-sm">
                    <svg class="h-6 w-6" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 9l10.5-3m-10.5 3v9m0-9L4.5 10.5M19.5 6v12a3 3 0 11-3-3m3 3H9"/></svg>
                </span>
                <span class="min-w-0 self-center">
                    <span class="block truncate text-sm font-semibold">${escapeHtml(displayName(audio.filename))}</span>
                    <span class="mt-0.5 block text-xs text-muted-foreground">${formatSize(audio.size)} &middot; ${escapeHtml(typeBadge(audio.mime_type))}</span>
                </span>
            </button>`;
        }).join('');

        queue.querySelectorAll('[data-audio-id]').forEach(button => {
            button.addEventListener('click', () => selectAudio(button.dataset.audioId, true));
        });
    }

    function selectAudio(id, updateHistory) {
        const audio = audios.find(item => item.id === id);
        if (!audio || id === currentID) return;

        currentID = id;
        if (wavesurfer) {
            wavesurfer.pause();
        }

        title.textContent = displayName(audio.filename);
        meta.textContent = `${formatSize(audio.size)} · Trove ${normalFolder(audio.folder)}`;
        if (badge) badge.textContent = typeBadge(audio.mime_type);
        download.href = `/api/file/${encodeURIComponent(audio.id)}`;
        document.title = `${displayName(audio.filename)} · Trove`;

        if (updateHistory) history.pushState({ audioID: id }, '', `/listen/${encodeURIComponent(id)}`);
        renderQueue();
        initWavesurfer();
    }

    async function loadQueue() {
        try {
            const response = await fetch('/api/file/', { credentials: 'include' });
            if (!response.ok) throw new Error('Unable to load audio files');
            const data = await response.json();
            audios = (data.files || []).filter(file => String(file.mime_type || '').toLowerCase().startsWith('audio/'));
            if (!currentAudio()) throw new Error('Audio unavailable');
            renderQueue();
            initWavesurfer();
        } catch (error) {
            queue.innerHTML = '<p class="px-2 py-4 text-sm text-muted-foreground">Unable to load audio files. Try refreshing the page.</p>';
        }
    }

    // UI Controls listeners
    if (btnPlay) {
        btnPlay.addEventListener('click', () => {
            if (wavesurfer) wavesurfer.playPause();
        });
    }

    if (btnBackward) {
        btnBackward.addEventListener('click', () => {
            if (wavesurfer) wavesurfer.setTime(Math.max(0, wavesurfer.getCurrentTime() - 10));
        });
    }

    if (btnForward) {
        btnForward.addEventListener('click', () => {
            if (wavesurfer) wavesurfer.setTime(Math.min(wavesurfer.getDuration(), wavesurfer.getCurrentTime() + 10));
        });
    }

    if (volumeSlider) {
        volumeSlider.addEventListener('input', (e) => {
            const vol = parseFloat(e.target.value);
            if (wavesurfer) wavesurfer.setVolume(vol);
            if (vol === 0) {
                iconVolume.classList.add('hidden');
                iconMuted.classList.remove('hidden');
            } else {
                iconVolume.classList.remove('hidden');
                iconMuted.classList.add('hidden');
            }
        });
    }

    if (btnMute) {
        btnMute.addEventListener('click', () => {
            if (!wavesurfer) return;
            const muted = wavesurfer.getMuted();
            wavesurfer.setMuted(!muted);
            if (!muted) {
                iconVolume.classList.add('hidden');
                iconMuted.classList.remove('hidden');
            } else {
                iconVolume.classList.remove('hidden');
                iconMuted.classList.add('hidden');
            }
        });
    }

    window.addEventListener('popstate', () => {
        const id = window.location.pathname.split('/').pop();
        if (id && id !== currentID) selectAudio(id, false);
    });

    document.addEventListener('keydown', (e) => {
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
        const key = e.key.toLowerCase();

        if (e.code === 'Space' || key === 'k') {
            e.preventDefault();
            if (wavesurfer) wavesurfer.playPause();
        } else if (key === 'arrowleft' || key === 'j') {
            e.preventDefault();
            if (wavesurfer) wavesurfer.setTime(Math.max(0, wavesurfer.getCurrentTime() - (key === 'j' ? 10 : 5)));
        } else if (key === 'arrowright' || key === 'l') {
            e.preventDefault();
            if (wavesurfer) wavesurfer.setTime(Math.min(wavesurfer.getDuration(), wavesurfer.getCurrentTime() + (key === 'l' ? 10 : 5)));
        } else if (key === 'arrowup') {
            e.preventDefault();
            if (wavesurfer) {
                const vol = Math.min(1, wavesurfer.getVolume() + 0.1);
                wavesurfer.setVolume(vol);
                volumeSlider.value = vol;
            }
        } else if (key === 'arrowdown') {
            e.preventDefault();
            if (wavesurfer) {
                const vol = Math.max(0, wavesurfer.getVolume() - 0.1);
                wavesurfer.setVolume(vol);
                volumeSlider.value = vol;
            }
        } else if (key === 'm') {
            e.preventDefault();
            if (wavesurfer) {
                const muted = wavesurfer.getMuted();
                wavesurfer.setMuted(!muted);
            }
        }
    });

    loadQueue();
})();
