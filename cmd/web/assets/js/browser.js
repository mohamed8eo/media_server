let allFiles = [];
let allFolders = ['/'];
let currentView = localStorage.getItem('file_view') || 'grid';

function initFileBrowser() {
    setFileView(currentView, false);
    fetchFiles();

    const searchInput = document.getElementById('file-search');
    if (searchInput) searchInput.addEventListener('input', () => renderFiles());
    const sortSelect = document.getElementById('file-sort');
    if (sortSelect) sortSelect.addEventListener('change', () => renderFiles());
}

function getCurrentFolder() {
    const params = new URLSearchParams(window.location.search);
    return params.get('folder') || '/';
}

function navigateToFolder(path) {
    if (path === '/') {
        window.history.pushState({}, '', '/');
    } else {
        window.history.pushState({}, '', '?folder=' + encodeURIComponent(path));
    }
    renderAll();
}

window.addEventListener('popstate', () => renderAll());

function renderAll() {
    renderBreadcrumbs();
    renderFolders();
    renderFiles();
}

function getSubfolders(parentPath) {
    const prefix = parentPath === '/' ? '/' : parentPath + '/';
    const matched = new Set();
    allFolders.forEach(f => {
        if (f === parentPath) return;
        if (f === '/') return;
        if (f === prefix.slice(0, -1)) return;
        if (f.startsWith(prefix)) {
            const relative = f.slice(prefix.length);
            const child = relative.split('/')[0];
            if (child) matched.add(prefix + child);
        } else if (parentPath !== '/' && f.startsWith('/') && !f.includes('/', 1)) {
            if (parentPath.split('/').length === 1) {
                matched.add(f);
            }
        }
    });
    if (parentPath === '/') {
        allFolders.forEach(f => {
            if (f === '/') return;
            const parts = f.split('/').filter(Boolean);
            if (parts.length === 1) matched.add('/' + parts[0]);
        });
    }
    return Array.from(matched).sort();
}

function getFilesInFolder(folderPath) {
    return allFiles.filter(f => (f.folder || '/') === folderPath);
}

function renderBreadcrumbs() {
    const el = document.getElementById('breadcrumbs');
    if (!el) return;
    const current = getCurrentFolder();
    if (current === '/') {
        el.innerHTML = '<span class="text-sm font-medium">My files</span>';
        return;
    }
    const parts = current.split('/').filter(Boolean);
    let path = '';
    let html = '<button onclick="navigateToFolder(\'/\')" class="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors">My files</button>';
    parts.forEach((part, i) => {
        path += '/' + part;
        const isLast = i === parts.length - 1;
        html += '<svg class="h-4 w-4 text-muted-foreground/50 mx-1 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path></svg>';
        if (isLast) {
            html += '<span class="text-sm font-medium truncate">' + escapeHtml(part) + '</span>';
        } else {
            const p = path;
            html += '<button onclick="navigateToFolder(\'' + escapeHtml(p) + '\')" class="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors">' + escapeHtml(part) + '</button>';
        }
    });
    el.innerHTML = html;
}

function renderFolders() {
    const folderListEl = document.getElementById('folder-list');
    if (!folderListEl) return;
    const current = getCurrentFolder();
    const subfolders = getSubfolders(current);

    let html = '';
    if (current !== '/') {
        const parent = current.split('/').slice(0, -1).join('/') || '/';
        html += '<button onclick="navigateToFolder(\'' + escapeHtml(parent) + '\')" class="w-full flex items-center px-3 py-2 rounded-lg text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"><svg class="h-4 w-4 mr-2 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"></path></svg>Back</button>';
    }

    subfolders.forEach(folder => {
        const name = folder.split('/').pop();
        const count = allFiles.filter(f => {
            const ff = f.folder || '/';
            return ff === folder || ff.startsWith(folder + '/');
        }).length;
        html += '<button onclick="navigateToFolder(\'' + escapeHtml(folder) + '\')" class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"><span class="flex items-center truncate"><svg class="h-4 w-4 mr-2 shrink-0 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg><span class="truncate">' + escapeHtml(name) + '</span></span><span class="text-xs rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground tabular-nums">' + count + '</span></button>';
    });

    if (subfolders.length === 0 && current === '/') {
        html += '<p class="text-xs text-muted-foreground px-3 py-2">No folders yet</p>';
    }
    folderListEl.innerHTML = html;
}

async function createNewFolder() {
    const current = getCurrentFolder();
    const clean = await showFolderPrompt();
    if (!clean) return;
    const newPath = current === '/' ? '/' + clean : current + '/' + clean;

    try {
        const res = await fetch('/api/file/mkdir', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: newPath }),
        });
        if (!res.ok) throw new Error('Failed to create folder');
        await fetchFiles();
        navigateToFolder(newPath);
    } catch (err) {
        console.error(err);
    }
}

function renderFiles() {
    const current = getCurrentFolder();
    const emptyEl = document.getElementById('file-empty');
    const gridEl = document.getElementById('file-grid');
    const listEl = document.getElementById('file-list');
    const fileCountEl = document.getElementById('file-count');
    const searchInput = document.getElementById('file-search');
    const sortSelect = document.getElementById('file-sort');

    const query = searchInput ? searchInput.value.toLowerCase().trim() : '';
    const sortBy = sortSelect ? sortSelect.value : 'date-desc';

    let files = getFilesInFolder(current);

    if (query) {
        files = allFiles.filter(f => f.filename.toLowerCase().includes(query));
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

    if (fileCountEl) fileCountEl.textContent = files.length + ' file' + (files.length !== 1 ? 's' : '');

    if (files.length === 0) {
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (gridEl) gridEl.innerHTML = '';
        if (listEl) listEl.innerHTML = '';
        return;
    }

    if (emptyEl) emptyEl.classList.add('hidden');

    if (gridEl) {
        gridEl.innerHTML = files.map(f => '<div class="group rounded-xl border bg-card text-card-foreground shadow-sm p-4 hover:shadow-md transition-all flex flex-col justify-between space-y-3"><div class="flex items-start justify-between"><div class="flex h-12 w-12 items-center justify-center rounded-lg bg-muted text-muted-foreground group-hover:bg-primary group-hover:text-primary-foreground transition-colors">' + getMimeIconSvg(f.mime_type) + '</div><span class="text-[10px] font-medium text-muted-foreground bg-muted px-2 py-0.5 rounded-md uppercase tracking-wider">' + formatMimeBadge(f.mime_type) + '</span></div><div class="space-y-1 overflow-hidden"><h4 class="text-sm font-medium leading-none truncate" title="' + escapeHtml(f.filename) + '">' + escapeHtml(f.filename) + '</h4><div class="flex items-center justify-between text-xs text-muted-foreground"><span>' + formatSize(f.size) + '</span><span>' + formatDate(f.created_at) + '</span></div></div></div>').join('');
    }

    if (listEl) {
        listEl.innerHTML = files.map(f => '<div class="flex items-center justify-between px-4 py-3 hover:bg-muted/50 transition-colors"><div class="flex items-center gap-3 min-w-0 flex-1 mr-4"><div class="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground shrink-0">' + getMimeIconSvg(f.mime_type) + '</div><div class="min-w-0 flex-1"><h4 class="text-sm font-medium truncate" title="' + escapeHtml(f.filename) + '">' + escapeHtml(f.filename) + '</h4><p class="text-xs text-muted-foreground">' + formatMimeBadge(f.mime_type) + '</p></div></div><div class="flex items-center gap-6 text-xs text-muted-foreground shrink-0"><span>' + formatSize(f.size) + '</span><span class="w-24 text-right">' + formatDate(f.created_at) + '</span></div></div>').join('');
    }
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
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm text-sm font-medium transition-colors bg-accent text-accent-foreground h-7 w-7';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground h-7 w-7';
    } else {
        if (gridEl) gridEl.classList.add('hidden');
        if (listEl) listEl.classList.remove('hidden');
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground h-7 w-7';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm text-sm font-medium transition-colors bg-accent text-accent-foreground h-7 w-7';
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

function formatMimeBadge(mime) {
    if (!mime) return 'file';
    if (mime.includes('image')) return 'image';
    if (mime.includes('pdf')) return 'pdf';
    if (mime.includes('video')) return 'video';
    if (mime.includes('audio')) return 'audio';
    if (mime.includes('text') || mime.includes('json') || mime.includes('javascript')) return 'code';
    if (mime.includes('zip') || mime.includes('tar') || mime.includes('compressed')) return 'archive';
    return mime.split('/')[1] || 'file';
}

function getMimeIconSvg(mime) {
    if (mime === 'inode/directory') {
        return '<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.75" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg>';
    }
    return '<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.75" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path></svg>';
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}
