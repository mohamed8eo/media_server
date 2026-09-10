let allFiles = [];
let allFolders = ['/'];
let currentView = localStorage.getItem('file_view') || 'grid';
let currentFilter = new URLSearchParams(window.location.search).get('type') || 'all';
let currentFolder = '/';

function initFileBrowser() {
    setFileView(currentView, false);
    setFilter(currentFilter, false);
    fetchFiles();
    fetchRecent();

    const searchInput = document.getElementById('file-search');
    if (searchInput) searchInput.addEventListener('input', () => renderFiles());
    const sortSelect = document.getElementById('file-sort');
    if (sortSelect) sortSelect.addEventListener('change', () => renderFiles());
}

function getCurrentFolder() {
    return currentFolder;
}

function navigateToFolder(path) {
    currentFolder = path;
    const params = new URLSearchParams(window.location.search);
    params.delete('folder');
    if (path !== '/') params.set('folder', path);
    const qs = params.toString();
    window.history.pushState({}, '', qs ? '?' + qs : '/');
    renderAll();
}

window.addEventListener('popstate', () => {
    const params = new URLSearchParams(window.location.search);
    currentFolder = params.get('folder') || '/';
    currentFilter = params.get('type') || 'all';
    setFilter(currentFilter, false);
    renderAll();
});

function renderAll() {
    renderFiles();
    updateActiveFilterTab();
}

function getFilesInFolder(folderPath) {
    let files = allFiles.filter(f => (f.folder || '/') === folderPath);
    if (currentFilter && currentFilter !== 'all') {
        files = files.filter(f => getCategoryFromMime(f.mime_type) === currentFilter);
    }
    return files;
}

function getCategoryFromMime(mime) {
    if (!mime) return 'other';
    if (mime.startsWith('video/')) return 'video';
    if (mime.startsWith('image/')) return 'image';
    if (mime.startsWith('audio/')) return 'audio';
    if (mime.includes('pdf') || mime.includes('document') || mime.includes('word') || mime.includes('sheet') || mime.includes('text/')) return 'document';
    return 'other';
}

function setFilter(filter, updateUrl = true) {
    currentFilter = filter;
    if (updateUrl) {
        const params = new URLSearchParams(window.location.search);
        params.delete('type');
        if (filter && filter !== 'all') params.set('type', filter);
        const qs = params.toString();
        window.history.replaceState({}, '', qs ? '?' + qs : '/');
    }
    updateActiveFilterTab();
    if (typeof updateSidebarActive === 'function') updateSidebarActive();
    renderFiles();
}

function updateActiveFilterTab() {
    const tabs = document.querySelectorAll('[data-filter]');
    tabs.forEach(tab => {
        if (tab.dataset.filter === currentFilter) {
            tab.className = 'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md text-sm font-medium transition-colors px-3 py-1.5 bg-background text-foreground shadow-sm';
        } else {
            tab.className = 'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md text-sm font-medium transition-colors px-3 py-1.5 text-muted-foreground hover:text-foreground';
        }
    });
    const titles = { all: 'All Media', video: 'Videos', image: 'Images', document: 'Documents', audio: 'Audio' };
    const titleEl = document.getElementById('section-title');
    if (titleEl) titleEl.textContent = titles[currentFilter] || 'All Media';
    const recentSection = document.getElementById('recent-section');
    if (recentSection) {
        recentSection.classList.toggle('hidden', currentFilter && currentFilter !== 'all');
    }
    const filterTabs = document.getElementById('filter-tabs');
    if (filterTabs) {
        filterTabs.classList.toggle('hidden', currentFilter && currentFilter !== 'all');
    }
}

function renderFiles() {
    const emptyEl = document.getElementById('file-empty');
    const gridEl = document.getElementById('file-grid');
    const listEl = document.getElementById('file-list');
    const fileCountEl = document.getElementById('file-count');
    const searchInput = document.getElementById('file-search');
    const sortSelect = document.getElementById('file-sort');

    const query = searchInput ? searchInput.value.toLowerCase().trim() : '';
    const sortBy = sortSelect ? sortSelect.value : 'date-desc';

    let files = getFilesInFolder(currentFolder);

    if (query) {
        files = allFiles.filter(f => f.filename.toLowerCase().includes(query));
        if (currentFilter && currentFilter !== 'all') {
            files = files.filter(f => getCategoryFromMime(f.mime_type) === currentFilter);
        }
    }

    files.sort((a, b) => {
        if (sortBy === 'date-desc') return new Date(b.created_at) - new Date(a.created_at);
        if (sortBy === 'date-asc') return new Date(a.created_at) - new Date(b.created_at);
        if (sortBy === 'name-asc') return a.filename.localeCompare(b.filename);
        if (sortBy === 'name-desc') return b.filename.localeCompare(a.filename);
        if (sortBy === 'size-desc') return b.size - a.size;
        if (sortBy === 'size-asc') return a.size - b.size;
        return 0;
    });

    if (fileCountEl) fileCountEl.textContent = files.length + ' item' + (files.length !== 1 ? 's' : '');

    if (files.length === 0) {
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (gridEl) gridEl.innerHTML = '';
        if (listEl) listEl.innerHTML = '';
        return;
    }

    if (emptyEl) emptyEl.classList.add('hidden');

    if (gridEl) {
        gridEl.innerHTML = files.map(f => renderGridCard(f)).join('');
        if (!gridEl._delegationBound) {
            gridEl.addEventListener('click', function(e) {
                var card = e.target.closest('[data-file-id]');
                if (!card) return;
                var action = e.target.closest('[data-action]');
                if (action) {
                    handleFileAction(action.dataset.action, card.dataset.fileId);
                    return;
                }
                var file = allFiles.find(function(f) { return f.id === card.dataset.fileId; });
                if (file) openFile(file);
            });
            gridEl._delegationBound = true;
        }
    }

    if (listEl) {
        listEl.innerHTML = files.map(f => renderListRow(f)).join('');
        if (!listEl._delegationBound) {
            listEl.addEventListener('click', function(e) {
                var row = e.target.closest('[data-file-id]');
                if (!row) return;
                var action = e.target.closest('[data-action]');
                if (action) {
                    handleFileAction(action.dataset.action, row.dataset.fileId);
                    return;
                }
                var file = allFiles.find(function(f) { return f.id === row.dataset.fileId; });
                if (file) openFile(file);
            });
            listEl._delegationBound = true;
        }
    }
}

function renderGridCard(f) {
    const cat = getCategoryFromMime(f.mime_type);
    const thumbHtml = getThumbnailHtml(f);
    const badge = getTypeBadge(f.mime_type);
    const displayName = getCleanName(f.filename);
    const actions = getFileActions(f);

    return `<div data-file-id="${f.id}" class="group relative cursor-pointer rounded-xl border bg-card text-card-foreground shadow-sm hover:shadow-lg hover:border-primary/50 transition-all duration-200 overflow-hidden">
        <div class="aspect-video relative overflow-hidden bg-muted">
            ${thumbHtml}
            <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200"></div>
            <div class="absolute bottom-0 left-0 right-0 p-3 flex items-center gap-2 translate-y-full group-hover:translate-y-0 transition-transform duration-200">
                ${actions}
            </div>
        </div>
        <div class="p-3 space-y-1">
            <div class="flex items-center gap-2">
                <span class="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-[10px] font-semibold text-primary uppercase tracking-wider shrink-0">${badge}</span>
                <h4 class="text-sm font-medium leading-none truncate" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
            </div>
            <div class="flex items-center justify-between text-xs text-muted-foreground">
                <span>${formatSize(f.size)}</span>
                <span>${formatDate(f.created_at)}</span>
            </div>
        </div>
    </div>`;
}

function renderListRow(f) {
    const cat = getCategoryFromMime(f.mime_type);
    const thumbHtml = getThumbnailHtmlSmall(f);
    const badge = getTypeBadge(f.mime_type);
    const displayName = getCleanName(f.filename);
    const actions = getFileActionsInline(f);

    return `<div data-file-id="${f.id}" class="group flex items-center gap-4 px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors border-b border-border last:border-0">
        <div class="w-10 h-10 rounded-lg overflow-hidden bg-muted shrink-0">
            ${thumbHtml}
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
                <h4 class="text-sm font-medium truncate" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                <span class="inline-flex items-center rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider shrink-0">${badge}</span>
            </div>
            <p class="text-xs text-muted-foreground">${formatSize(f.size)} &middot; ${formatDate(f.created_at)}</p>
        </div>
        <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
            ${actions}
        </div>
    </div>`;
}

function getThumbnailHtml(f) {
    const cat = getCategoryFromMime(f.mime_type);
    if (cat === 'image') {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIcon('${f.mime_type}')" />`;
    }
    if (cat === 'video') {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIcon('${f.mime_type}')" /><div class="absolute inset-0 flex items-center justify-center"><div class="flex h-12 w-12 items-center justify-center rounded-full bg-black/50 backdrop-blur-sm"><svg class="h-6 w-6 text-white ml-0.5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></div></div>`;
    }
    return `<div class="w-full h-full flex items-center justify-center">${getFallbackIcon(f.mime_type)}</div>`;
}

function getThumbnailHtmlSmall(f) {
    const cat = getCategoryFromMime(f.mime_type);
    if (cat === 'image') {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIconSm('${f.mime_type}')" />`;
    }
    if (cat === 'video') {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIconSm('${f.mime_type}')" />`;
    }
    return `<div class="w-full h-full flex items-center justify-center">${getFallbackIconSm(f.mime_type)}</div>`;
}

window.getFallbackIcon = function(mime) {
    const cat = getCategoryFromMime(mime);
    const icons = {
        video: '<svg class="h-10 w-10 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.777-.416L16 11"/><rect width="14" height="12" x="2" y="6" rx="2"/></svg>',
        image: '<svg class="h-10 w-10 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>',
        audio: '<svg class="h-10 w-10 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
        document: '<svg class="h-10 w-10 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>',
        other: '<svg class="h-10 w-10 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>'
    };
    return icons[cat] || icons.other;
};

window.getFallbackIconSm = function(mime) {
    const cat = getCategoryFromMime(mime);
    const icons = {
        video: '<svg class="h-5 w-5 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.777-.416L16 11"/><rect width="14" height="12" x="2" y="6" rx="2"/></svg>',
        image: '<svg class="h-5 w-5 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>',
        audio: '<svg class="h-5 w-5 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
        document: '<svg class="h-5 w-5 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>',
        other: '<svg class="h-5 w-5 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>'
    };
    return icons[cat] || icons.other;
};

function getFileActions(f) {
    const cat = getCategoryFromMime(f.mime_type);
    let html = '';
    if (cat === 'video' || cat === 'image') {
        html += `<button data-action="play" class="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 backdrop-blur-sm text-white hover:bg-white/30 transition-colors" title="Play"><svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 backdrop-blur-sm text-white hover:bg-white/30 transition-colors" title="Download"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="share" class="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 backdrop-blur-sm text-white hover:bg-white/30 transition-colors" title="Share link"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" x2="12" y1="2" y2="15"/></svg></button>`;
    return html;
}

function getFileActionsInline(f) {
    const cat = getCategoryFromMime(f.mime_type);
    let html = '';
    if (cat === 'video' || cat === 'image') {
        html += `<button data-action="play" class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Play"><svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Download"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="share" class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Share link"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" x2="12" y1="2" y2="15"/></svg></button>`;
    return html;
}

function handleFileAction(action, fileId) {
    const file = allFiles.find(f => f.id === fileId);
    if (!file) return;
    if (action === 'play') {
        openFile(file);
    } else if (action === 'download') {
        window.location.href = '/api/file/' + file.id;
    } else if (action === 'share') {
        const url = window.location.origin + '/api/file/' + file.id;
        navigator.clipboard.writeText(url).then(() => {
            showToast('Link copied to clipboard');
        });
    }
}

function showToast(message) {
    const toast = document.createElement('div');
    toast.className = 'fixed bottom-4 right-4 z-[100] rounded-lg bg-foreground text-background px-4 py-2 text-sm font-medium shadow-lg animate-in fade-in slide-in-from-bottom-4';
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => { toast.remove(); }, 2500);
}

function getCleanName(filename) {
    const dotIndex = filename.lastIndexOf('.');
    if (dotIndex > 0) return filename.substring(0, dotIndex);
    return filename;
}

function getTypeBadge(mime) {
    if (!mime) return 'FILE';
    if (mime.startsWith('video/')) return mime.split('/')[1]?.toUpperCase().substring(0, 6) || 'VIDEO';
    if (mime.startsWith('image/')) return mime.split('/')[1]?.toUpperCase().substring(0, 6) || 'IMAGE';
    if (mime.includes('pdf')) return 'PDF';
    if (mime.startsWith('audio/')) return mime.split('/')[1]?.toUpperCase().substring(0, 6) || 'AUDIO';
    if (mime.includes('zip') || mime.includes('tar')) return 'ARCHIVE';
    if (mime.includes('json') || mime.includes('javascript')) return 'CODE';
    return 'FILE';
}

function setFileView(view, save = true) {
    currentView = view;
    if (save) localStorage.setItem('file_view', view);
    const gridEl = document.getElementById('file-grid');
    const listEl = document.getElementById('file-list');
    const btnGrid = document.getElementById('view-grid');
    const btnList = document.getElementById('view-list');
    if (view === 'grid') {
        if (gridEl) gridEl.classList.remove('hidden');
        if (listEl) listEl.classList.add('hidden');
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors h-8 w-8 bg-background text-foreground shadow-sm';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors text-muted-foreground hover:text-foreground h-8 w-8';
    } else {
        if (gridEl) gridEl.classList.add('hidden');
        if (listEl) listEl.classList.remove('hidden');
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors text-muted-foreground hover:text-foreground h-8 w-8';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors h-8 w-8 bg-background text-foreground shadow-sm';
    }
}

async function fetchFiles() {
    const loadingEl = document.getElementById('file-loading');
    try {
        const res = await fetch('/api/file/', { method: 'GET', credentials: 'include' });
        if (!res.ok) { if (res.status === 401) return; throw new Error('Failed to fetch files'); }
        const data = await res.json();
        allFiles = data.files || [];
        allFolders = data.folders || ['/'];
        if (!allFolders.includes('/')) allFolders.unshift('/');
        if (loadingEl) loadingEl.classList.add('hidden');
        renderAll();
    } catch (err) {
        console.error(err);
        if (loadingEl) loadingEl.innerHTML = '<p class="text-sm text-destructive font-medium">Failed to load files. Please refresh.</p>';
    }
}

async function fetchRecent() {
    try {
        const res = await fetch('/api/file/recent?limit=10', { method: 'GET', credentials: 'include' });
        if (!res.ok) return;
        const data = await res.json();
        renderRecentSection('recent-played', data.recently_played || []);
        renderRecentSection('recent-uploaded', data.recent_uploads || []);
    } catch (err) {
        console.error(err);
    }
}

function renderRecentSection(containerId, files) {
    const el = document.getElementById(containerId);
    if (!el) return;
    if (files.length === 0) {
        el.classList.add('hidden');
        const heading = el.previousElementSibling;
        if (heading && heading.tagName === 'DIV') heading.classList.add('hidden');
        return;
    }
    el.classList.remove('hidden');
    const heading = el.previousElementSibling;
    if (heading && heading.tagName === 'DIV') heading.classList.remove('hidden');
    el.innerHTML = files.map(f => {
        const cat = getCategoryFromMime(f.mime_type);
        const displayName = getCleanName(f.filename);
        const badge = getTypeBadge(f.mime_type);
        const thumbHtml = cat === 'image' || cat === 'video'
            ? `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'" /><div class="w-full h-full items-center justify-center bg-muted hidden">${getFallbackIcon(f.mime_type)}</div>`
            : `<div class="w-full h-full flex items-center justify-center bg-muted">${getFallbackIcon(f.mime_type)}</div>`;
        const playBtn = cat === 'video' ? '<div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"><div class="flex h-10 w-10 items-center justify-center rounded-full bg-black/50 backdrop-blur-sm"><svg class="h-5 w-5 text-white ml-0.5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></div></div>' : '';
        return `<div onclick="openFileById('${f.id}')" class="group relative flex-shrink-0 w-48 cursor-pointer rounded-xl border bg-card text-card-foreground shadow-sm hover:shadow-md hover:border-primary/50 transition-all duration-200 overflow-hidden">
            <div class="aspect-video relative overflow-hidden bg-muted">
                ${thumbHtml}
                ${playBtn}
            </div>
            <div class="p-2 space-y-1">
                <h4 class="text-xs font-medium leading-none truncate" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                <div class="flex items-center gap-1">
                    <span class="text-[9px] font-semibold text-primary bg-primary/10 px-1 py-0.5 rounded uppercase">${badge}</span>
                    <span class="text-[10px] text-muted-foreground">${formatSize(f.size)}</span>
                </div>
            </div>
        </div>`;
    }).join('');
}

function scrollRecent(section) {
    const el = document.getElementById('recent-' + section);
    if (el) el.scrollBy({ left: 300, behavior: 'smooth' });
}

window.openFileById = function(fileId) {
    const file = allFiles.find(f => f.id === fileId);
    if (file) openFile(file);
};

function openFile(file) {
    const cat = getCategoryFromMime(file.mime_type);
    if (cat === 'video' || cat === 'image' || cat === 'document') {
        showFileViewerModal(file, cat);
    } else {
        window.location.href = '/api/file/' + file.id;
    }
}

function showFileViewerModal(file, type) {
    const root = document.getElementById('file-viewer-root');
    if (!root) return;

    const displayName = getCleanName(file.filename);
    let contentHtml = '';
    if (type === 'video') {
        contentHtml = `<video controls autoplay preload="metadata" src="/api/file/${file.id}" class="w-full h-full object-contain"></video>`;
    } else if (type === 'image') {
        contentHtml = `<img src="/api/file/${file.id}" alt="${escapeHtml(file.filename)}" class="max-w-full max-h-full object-contain" />`;
    } else if (type === 'document') {
        contentHtml = `<iframe src="/api/file/${file.id}" class="w-full h-full border-0" style="min-height:70vh"></iframe>`;
    }

    root.innerHTML = `
      <div data-viewer-overlay class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md animate-overlay-show transition-opacity"></div>
      <div data-viewer-content class="fixed inset-4 md:inset-8 lg:inset-16 z-50 flex flex-col animate-content-show">
        <div class="flex items-center justify-between mb-4 px-2">
          <div class="min-w-0 flex-1">
            <h2 class="text-lg font-semibold text-white/90 truncate">${escapeHtml(displayName)}</h2>
            <p class="text-sm text-white/50">${formatSize(file.size)} &middot; ${getTypeBadge(file.mime_type)}</p>
          </div>
          <div class="flex items-center gap-2 shrink-0 ml-4">
            <a href="/api/file/${file.id}" download class="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors bg-white/10 text-white/90 hover:bg-white/20 h-9 px-4 backdrop-blur-sm">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>
              Download
            </a>
            <button data-viewer-close class="inline-flex h-9 w-9 items-center justify-center rounded-lg text-sm font-medium transition-colors bg-white/10 text-white/90 hover:bg-white/20 backdrop-blur-sm">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6 6 18M6 6l12 12"></path></svg>
            </button>
          </div>
        </div>
        <div class="flex-1 flex items-center justify-center overflow-hidden rounded-2xl bg-black/50 backdrop-blur-sm border border-white/10">
          ${contentHtml}
        </div>
      </div>
    `;

    var overlay = root.querySelector('[data-viewer-overlay]');
    var content = root.querySelector('[data-viewer-content]');
    var closeBtn = root.querySelector('[data-viewer-close]');
    var media = root.querySelector('video, img, iframe');

    function cleanup() {
        if (overlay) overlay.style.opacity = '0';
        if (content) { content.style.opacity = '0'; content.style.transform = 'scale(0.95)'; }
        if (media && media.tagName === 'VIDEO') { media.pause(); media.removeAttribute('src'); media.load(); }
        setTimeout(function() { root.innerHTML = ''; }, 200);
        cleanupListeners();
    }
    function onKeydown(e) { if (e.key === 'Escape') { e.preventDefault(); cleanup(); } }
    function cleanupListeners() {
        document.removeEventListener('keydown', onKeydown);
        if (closeBtn) closeBtn.removeEventListener('click', cleanup);
        if (overlay) overlay.removeEventListener('click', cleanup);
    }

    document.addEventListener('keydown', onKeydown);
    if (closeBtn) closeBtn.addEventListener('click', cleanup);
    if (overlay) overlay.addEventListener('click', cleanup);
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}

function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}
