let allFiles = [];
let allFolders = ['/'];
let currentView = localStorage.getItem('file_view') || 'grid';
let currentFilter = new URLSearchParams(window.location.search).get('type') || 'all';
let currentFolder = new URLSearchParams(window.location.search).get('folder') || '/';

function initFileBrowser() {
    setFileView(currentView, false);
    setFilter(currentFilter, false);
    fetchFiles();
    fetchRecent();

    const searchInput = document.getElementById('file-search');
    if (searchInput) searchInput.addEventListener('input', () => renderFiles());
    const sortSelect = document.getElementById('file-sort');
    if (sortSelect) sortSelect.addEventListener('change', () => renderFiles());

    document.addEventListener('click', (e) => {
        if (!e.target.closest('[data-dropdown-menu]')) {
            document.querySelectorAll('[data-dropdown-content]').forEach(el => el.classList.add('hidden'));
        }
    });
}

async function createNewFolder() {
    const name = await showFolderPrompt();
    if (!name) return;
    const prefix = currentFolder === '/' ? '' : currentFolder.replace(/\/$/, '');
    const fullPath = prefix ? prefix + '/' + name : name;
    try {
        const res = await fetch('/api/file/mkdir', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: fullPath })
        });
        if (!res.ok) throw new Error('Failed');
        const data = await res.json();
        if (!allFolders.includes(data.path)) allFolders.push(data.path);
        renderAll();
        showToast('Folder created successfully');
    } catch (e) {
        showToast('Failed to create folder');
    }
}

function showFolderPrompt() {
    return new Promise(resolve => {
        const dialog = document.createElement('div');
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-in fade-in';
        dialog.innerHTML = `
            <div class="bg-card border border-border dark:border-slate-800 rounded-2xl p-6 shadow-2xl max-w-sm w-full mx-4 space-y-4 animate-in zoom-in-95">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">Create New Folder</h3>
                <input type="text" id="new-folder-name" placeholder="Folder name" class="flex h-10 w-full rounded-xl border border-input dark:border-slate-800 bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" autofocus />
                <div class="flex justify-end gap-2.5">
                    <button id="folder-cancel" class="inline-flex items-center justify-center rounded-xl text-sm font-medium border border-input bg-background h-9 px-4 hover:bg-accent">Cancel</button>
                    <button id="folder-create" class="inline-flex items-center justify-center rounded-xl text-sm font-medium bg-primary text-primary-foreground shadow h-9 px-4 hover:bg-primary/90">Create</button>
                </div>
            </div>
        `;
        document.body.appendChild(dialog);
        const input = dialog.querySelector('#new-folder-name');
        const cancelBtn = dialog.querySelector('#folder-cancel');
        const createBtn = dialog.querySelector('#folder-create');

        function done(val) {
            dialog.remove();
            resolve(val ? val.trim() : null);
        }

        cancelBtn.onclick = () => done(null);
        createBtn.onclick = () => done(input.value);
        input.onkeydown = (e) => {
            if (e.key === 'Enter') done(input.value);
            if (e.key === 'Escape') done(null);
        };
        input.focus();
    });
}

function getFoldersInFolder(folderPath) {
    const prefix = folderPath === '/' ? '/' : folderPath.replace(/\/$/, '') + '/';
    return allFolders.filter(f => {
        if (f === folderPath) return false;
        if (folderPath === '/') {
            const parts = f.replace(/^\//, '').split('/');
            return parts.length === 1 && f !== '/';
        }
        if (!f.startsWith(prefix)) return false;
        const rest = f.slice(prefix.length);
        return rest.length > 0 && !rest.includes('/');
    }).sort();
}

function getFolderItemCount(folderPath) {
    const prefix = folderPath === '/' ? '/' : folderPath.replace(/\/$/, '') + '/';
    const subFiles = allFiles.filter(f => {
        const fp = normalizeFolder(f.folder || '/');
        return fp === folderPath || fp.startsWith(prefix);
    });
    const subFolders = allFolders.filter(f => f !== folderPath && f.startsWith(prefix));
    return subFiles.length + subFolders.length;
}

function renderFolderCard(folder) {
    const name = folder.split('/').pop();
    const count = getFolderItemCount(folder);
    return `<div data-folder-path="${escapeHtml(folder)}" class="group relative cursor-pointer rounded-2xl border bg-slate-50 dark:bg-slate-900/60 border-slate-200/80 dark:border-slate-800 text-card-foreground p-4 hover:shadow-md hover:border-indigo-500/50 transition-all duration-200 flex items-center justify-between folder-card">
        <div class="flex items-center gap-3.5 min-w-0">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-indigo-500/10 dark:bg-indigo-500/15 text-indigo-600 dark:text-indigo-400 border border-indigo-500/20 shadow-sm">
                <svg class="h-5 w-5 fill-indigo-500/20" fill="currentColor" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M2.75 12.75V12A2.25 2.25 0 0 1 5 9.75h14A2.25 2.25 0 0 1 21.25 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z"/></svg>
            </div>
            <div class="min-w-0">
                <h4 class="text-sm font-semibold truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(folder)}">${escapeHtml(name)}</h4>
                <p class="text-xs text-slate-500 dark:text-slate-400">Folder</p>
            </div>
        </div>
        <span class="inline-flex items-center rounded-full bg-slate-200/70 dark:bg-slate-800 px-3 py-1 text-xs font-medium text-slate-700 dark:text-slate-300 tabular-nums shrink-0">
            ${count} item${count !== 1 ? 's' : ''}
        </span>
    </div>`;
}

function renderFolderRow(folder) {
    return renderFolderCard(folder);
}

function renderBreadcrumbs() {
    const el = document.getElementById('breadcrumbs');
    if (!el) return;
    const parts = currentFolder === '/' ? [] : currentFolder.replace(/^\//, '').split('/');
    let html = `<button onclick="navigateToFolder('/')" class="hover:text-foreground transition-colors font-medium">Home</button>`;
    let path = '';
    parts.forEach((part, i) => {
        path += '/' + part;
        const isLast = i === parts.length - 1;
        html += `<span class="text-muted-foreground">/</span>`;
        if (isLast) {
            html += `<span class="font-medium text-foreground">${escapeHtml(part)}</span>`;
        } else {
            const p = path;
            html += `<button onclick="navigateToFolder('${escapeHtml(p)}')" class="hover:text-foreground transition-colors">${escapeHtml(part)}</button>`;
        }
    });
    el.innerHTML = html;
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
    renderBreadcrumbs();
    renderFiles();
    updateActiveFilterTab();
    updateUploadLinks();
}

function updateUploadLinks() {
    const uploadLinks = document.querySelectorAll('a[href^="/upload"], a[href="/upload"]');
    uploadLinks.forEach(link => {
        if (currentFolder && currentFolder !== '/') {
            link.href = '/upload?folder=' + encodeURIComponent(currentFolder);
        } else {
            link.href = '/upload';
        }
    });
}

function normalizeFolder(path) {
    if (!path || path === '') return '/';
    let cleaned = path.replace(/\/+/g, '/');
    if (cleaned !== '/' && cleaned.endsWith('/')) {
        cleaned = cleaned.slice(0, -1);
    }
    if (!cleaned.startsWith('/')) {
        cleaned = '/' + cleaned;
    }
    return cleaned;
}

function getFilesInFolder(folderPath) {
    const target = normalizeFolder(folderPath);
    let files = allFiles.filter(f => normalizeFolder(f.folder || '/') === target);
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
            tab.className = 'inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg text-sm font-medium transition-colors px-3 h-8 bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900 shadow-sm';
        } else {
            tab.className = 'inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg text-sm font-medium transition-colors px-3 h-8 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100';
        }
    });
    const titles = { all: 'All Media', video: 'Videos', image: 'Images', document: 'Documents', audio: 'Audio' };
    const titleEl = document.getElementById('section-title');
    if (titleEl) titleEl.textContent = titles[currentFilter] || 'All Media';
    const recentSection = document.getElementById('recent-section');
    if (recentSection) {
        recentSection.classList.toggle('hidden', currentFilter && currentFilter !== 'all');
    }
}

function renderFiles() {
    const emptyEl = document.getElementById('file-empty');
    const folderSectionEl = document.getElementById('folder-section');
    const fileSectionEl = document.getElementById('file-section');
    const folderGridEl = document.getElementById('folder-grid');
    const gridEl = document.getElementById('file-grid');
    const listEl = document.getElementById('file-list');
    const fileCountEl = document.getElementById('file-count');
    const folderCountEl = document.getElementById('folder-count');
    const fileItemsCountEl = document.getElementById('file-items-count');
    const searchInput = document.getElementById('file-search');
    const sortSelect = document.getElementById('file-sort');

    const query = searchInput ? searchInput.value.toLowerCase().trim() : '';
    const sortBy = sortSelect ? sortSelect.value : 'date-desc';

    let files = getFilesInFolder(currentFolder);
    let folders = getFoldersInFolder(currentFolder);

    if (query) {
        files = allFiles.filter(f => f.filename.toLowerCase().includes(query));
        if (currentFilter && currentFilter !== 'all') {
            files = files.filter(f => getCategoryFromMime(f.mime_type) === currentFilter);
        }
        folders = allFolders.filter(f => f !== '/' && f.toLowerCase().includes(query));
    }

    // In category views, filter out folders that contain no matching items
    if (currentFilter && currentFilter !== 'all') {
        folders = folders.filter(f => {
            const prefix = f.replace(/\/$/, '') + '/';
            const matchingFiles = allFiles.filter(file => {
                const fp = normalizeFolder(file.folder || '/');
                return (fp === f || fp.startsWith(prefix)) && getCategoryFromMime(file.mime_type) === currentFilter;
            });
            return matchingFiles.length > 0;
        });
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

    const totalCount = folders.length + files.length;
    if (fileCountEl) fileCountEl.textContent = totalCount + ' item' + (totalCount !== 1 ? 's' : '');
    if (folderCountEl) folderCountEl.textContent = folders.length + ' folder' + (folders.length !== 1 ? 's' : '');
    if (fileItemsCountEl) fileItemsCountEl.textContent = files.length + ' file' + (files.length !== 1 ? 's' : '');

    if (totalCount === 0) {
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (folderSectionEl) folderSectionEl.classList.add('hidden');
        if (fileSectionEl) fileSectionEl.classList.add('hidden');
        if (folderGridEl) folderGridEl.innerHTML = '';
        if (gridEl) gridEl.innerHTML = '';
        if (listEl) listEl.innerHTML = '';
        return;
    }

    if (emptyEl) emptyEl.classList.add('hidden');

    if (folders.length > 0) {
        if (folderSectionEl) folderSectionEl.classList.remove('hidden');
        if (folderGridEl) {
            folderGridEl.innerHTML = folders.map(f => renderFolderCard(f)).join('');
            if (!folderGridEl._delegationBound) {
                folderGridEl.addEventListener('click', function(e) {
                    var folderCard = e.target.closest('[data-folder-path]');
                    if (folderCard) navigateToFolder(folderCard.dataset.folderPath);
                });
                folderGridEl._delegationBound = true;
            }
        }
    } else {
        if (folderSectionEl) folderSectionEl.classList.add('hidden');
        if (folderGridEl) folderGridEl.innerHTML = '';
    }

    if (files.length > 0) {
        if (fileSectionEl) fileSectionEl.classList.remove('hidden');
        const foldersListHtml = folders.map(f => renderFolderRow(f)).join('');
        const filesHtml = files.map(f => renderGridCard(f)).join('');
        const filesListHtml = files.map(f => renderListRow(f)).join('');

        if (gridEl) {
            gridEl.innerHTML = filesHtml;
            if (!gridEl._delegationBound) {
                gridEl.addEventListener('click', function(e) {
                    var card = e.target.closest('[data-file-id]');
                    if (!card) return;
                    var action = e.target.closest('[data-action]');
                    if (action) {
                        handleFileAction(action.dataset.action, card.dataset.fileId, e);
                        return;
                    }
                    var file = allFiles.find(function(f) { return f.id === card.dataset.fileId; });
                    if (file) openFile(file);
                });
                gridEl._delegationBound = true;
            }
        }

        if (listEl) {
            listEl.innerHTML = filesListHtml;
            if (!listEl._delegationBound) {
                listEl.addEventListener('click', function(e) {
                    var row = e.target.closest('[data-file-id]');
                    if (!row) return;
                    var action = e.target.closest('[data-action]');
                    if (action) {
                        handleFileAction(action.dataset.action, row.dataset.fileId, e);
                        return;
                    }
                    var file = allFiles.find(function(f) { return f.id === row.dataset.fileId; });
                    if (file) openFile(file);
                });
                listEl._delegationBound = true;
            }
        }
    } else {
        if (fileSectionEl) {
            fileSectionEl.classList.add('hidden');
            if (gridEl) gridEl.innerHTML = '';
            if (listEl) listEl.innerHTML = '';
        }
    }
}

function renderGridCard(f) {
    const thumbHtml = getThumbnailHtml(f);
    const badge = getTypeBadge(f.mime_type);
    const displayName = getCleanName(f.filename);
    const actions = getFileActions(f);
    const cat = getCategoryFromMime(f.mime_type);

    let extraBadge = '';
    if (cat === 'document') {
        extraBadge = `<span class="inline-flex items-center rounded-md bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 px-1.5 py-0.5 text-[9px] font-semibold uppercase">${formatSize(f.size)}</span>`;
    }

    return `<div data-file-id="${f.id}" class="group relative cursor-pointer rounded-2xl border bg-card border-border dark:border-slate-800/80 text-card-foreground shadow-sm hover:shadow-xl hover:border-indigo-500/50 transition-all duration-200 overflow-hidden flex flex-col">
        <div class="aspect-video relative overflow-hidden bg-slate-100 dark:bg-slate-900/60 flex items-center justify-center">
            ${thumbHtml}
            <div class="absolute inset-0 bg-gradient-to-t from-black/70 via-black/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200"></div>
            <div class="absolute bottom-2 left-2 right-2 p-1 flex items-center justify-center gap-1.5 opacity-0 group-hover:opacity-100 translate-y-2 group-hover:translate-y-0 transition-all duration-200 bg-background/90 dark:bg-slate-900/90 backdrop-blur-md rounded-xl border border-border dark:border-slate-800 shadow-lg z-10">
                ${actions}
            </div>
        </div>
        <div class="p-3.5 space-y-2 flex-1 flex flex-col justify-between">
            <div class="space-y-1">
                <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md bg-indigo-500/10 dark:bg-indigo-500/15 text-indigo-600 dark:text-indigo-400 border border-indigo-500/20 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider shrink-0">${badge}</span>
                    <h4 class="text-sm font-medium leading-snug truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                </div>
            </div>
            <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 pt-1 border-t border-border/60">
                <span class="tabular-nums">${formatSize(f.size)}</span>
                <span class="tabular-nums">${formatDate(f.created_at)}</span>
            </div>
        </div>
    </div>`;
}

function renderListRow(f) {
    const thumbHtml = getThumbnailHtmlSmall(f);
    const badge = getTypeBadge(f.mime_type);
    const displayName = getCleanName(f.filename);
    const actions = getFileActionsInline(f);

    return `<div data-file-id="${f.id}" class="group flex items-center gap-3 sm:gap-4 px-3 sm:px-4 py-3 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors border-b border-border dark:border-slate-800/60 last:border-0">
        <div class="w-11 h-11 rounded-xl overflow-hidden bg-slate-100 dark:bg-slate-900/60 shrink-0 flex items-center justify-center border border-border dark:border-slate-800">
            ${thumbHtml}
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
                <h4 class="text-sm font-medium truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                <span class="inline-flex items-center rounded-md bg-indigo-500/10 dark:bg-indigo-500/15 text-indigo-600 dark:text-indigo-400 border border-indigo-500/20 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider shrink-0">${badge}</span>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400 truncate tabular-nums">${formatSize(f.size)} <span class="hidden sm:inline">&middot; ${formatDate(f.created_at)}</span></p>
        </div>
        <div class="flex items-center gap-1.5 opacity-90 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity shrink-0">
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
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIcon('${f.mime_type}')" /><div class="absolute inset-0 flex items-center justify-center"><div class="flex h-12 w-12 items-center justify-center rounded-full bg-black/60 backdrop-blur-md shadow-lg text-white"><svg class="h-6 w-6 ml-0.5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></div></div>`;
    }
    if (cat === 'document') {
        // Document / PDF stylized preview card with thumbnail or document badge
        return `<div class="w-full h-full flex flex-col items-center justify-center bg-gradient-to-br from-amber-500/10 to-orange-500/10 text-amber-600 dark:text-amber-400 p-4 text-center">
            <svg class="h-10 w-10 mb-1" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>
            <span class="text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-amber-500/20 text-amber-700 dark:text-amber-300">${getTypeBadge(f.mime_type)}</span>
        </div>`;
    }
    return `<div class="w-full h-full flex items-center justify-center">${getFallbackIcon(f.mime_type)}</div>`;
}

function getThumbnailHtmlSmall(f) {
    const cat = getCategoryFromMime(f.mime_type);
    if (cat === 'image' || cat === 'video') {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getFallbackIconSm('${f.mime_type}')" />`;
    }
    if (cat === 'document') {
        return `<div class="w-full h-full flex items-center justify-center bg-amber-500/10 text-amber-600 dark:text-amber-400"><svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg></div>`;
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
    if (cat === 'video' || cat === 'image' || cat === 'document') {
        html += `<button data-action="play" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors" title="Play / Preview"><svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors" title="Download"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="move" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors" title="Move to Folder"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" x2="12" y1="11" y2="17"/><line x1="9" x2="15" y1="14" y2="14"/></svg></button>`;
    
    // More dropdown
    html += `<div data-dropdown-menu class="relative">
        <button data-action="more" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors" title="More options">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
        </button>
        <div data-dropdown-content class="hidden absolute right-0 bottom-full mb-2 w-40 rounded-xl bg-card border border-border dark:border-slate-800 shadow-xl py-1.5 z-50 text-xs font-medium">
            <button data-action="info" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2">More info</button>
            <button data-action="delete" class="w-full text-left px-3.5 py-2 text-destructive hover:bg-destructive/10 flex items-center gap-2">Delete file</button>
        </div>
    </div>`;

    return html;
}

function getFileActionsInline(f) {
    const cat = getCategoryFromMime(f.mime_type);
    let html = '';
    if (cat === 'video' || cat === 'image' || cat === 'document') {
        html += `<button data-action="play" class="inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Play / Preview"><svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Download"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="move" class="inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="Move to Folder"><svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" x2="12" y1="11" y2="17"/><line x1="9" x2="15" y1="14" y2="14"/></svg></button>`;
    
    html += `<div data-dropdown-menu class="relative">
        <button data-action="more" class="inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground h-8 w-8" title="More options">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
        </button>
        <div data-dropdown-content class="hidden absolute right-0 top-full mt-2 w-40 rounded-xl bg-card border border-border dark:border-slate-800 shadow-xl py-1.5 z-50 text-xs font-medium">
            <button data-action="info" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2">More info</button>
            <button data-action="delete" class="w-full text-left px-3.5 py-2 text-destructive hover:bg-destructive/10 flex items-center gap-2">Delete file</button>
        </div>
    </div>`;

    return html;
}

function handleFileAction(action, fileId, e) {
    if (action === 'more') {
        const container = e.target.closest('[data-dropdown-menu]');
        if (container) {
            const menu = container.querySelector('[data-dropdown-content]');
            if (menu) {
                document.querySelectorAll('[data-dropdown-content]').forEach(el => {
                    if (el !== menu) el.classList.add('hidden');
                });
                menu.classList.toggle('hidden');
            }
        }
        e.stopPropagation();
        return;
    }

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
    } else if (action === 'rename') {
        showRenamePrompt(file);
    } else if (action === 'move') {
        showMovePrompt(file);
    } else if (action === 'info') {
        showFileInfoModal(file);
    } else if (action === 'delete') {
        showDeleteConfirm(file);
    }
}

function showFileInfoModal(file) {
    const dialog = document.createElement('div');
    dialog.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-in fade-in';
    dialog.innerHTML = `
        <div class="bg-card border border-border dark:border-slate-800 rounded-2xl p-6 shadow-2xl max-w-md w-full mx-4 space-y-4 animate-in zoom-in-95">
            <div class="flex items-center justify-between">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">File Information</h3>
                <button id="info-close" class="text-slate-500 hover:text-slate-700 dark:hover:text-slate-300">
                    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12"/></svg>
                </button>
            </div>
            <div class="space-y-3 text-sm text-slate-700 dark:text-slate-300">
                <div class="flex justify-between py-1 border-b border-border/60">
                    <span class="font-medium text-slate-500">Filename</span>
                    <span class="truncate max-w-[240px]" title="${escapeHtml(file.filename)}">${escapeHtml(file.filename)}</span>
                </div>
                <div class="flex justify-between py-1 border-b border-border/60">
                    <span class="font-medium text-slate-500">Size</span>
                    <span>${formatSize(file.size)}</span>
                </div>
                <div class="flex justify-between py-1 border-b border-border/60">
                    <span class="font-medium text-slate-500">Type</span>
                    <span>${escapeHtml(file.mime_type)}</span>
                </div>
                <div class="flex justify-between py-1 border-b border-border/60">
                    <span class="font-medium text-slate-500">Folder</span>
                    <span>${escapeHtml(file.folder)}</span>
                </div>
                <div class="flex justify-between py-1 border-b border-border/60">
                    <span class="font-medium text-slate-500">Uploaded</span>
                    <span>${formatDate(file.created_at)}</span>
                </div>
            </div>
            <div class="flex justify-end pt-2">
                <button id="info-ok" class="inline-flex items-center justify-center rounded-xl text-sm font-medium bg-primary text-primary-foreground shadow h-9 px-4 hover:bg-primary/90">Close</button>
            </div>
        </div>
    `;
    document.body.appendChild(dialog);
    const closeBtn = dialog.querySelector('#info-close');
    const okBtn = dialog.querySelector('#info-ok');
    function close() { dialog.remove(); }
    closeBtn.onclick = close;
    okBtn.onclick = close;
    dialog.onclick = (e) => { if (e.target === dialog) close(); };
}

async function showRenamePrompt(file) {
    const newName = await showPromptModal('Rename File', file.filename);
    if (!newName || newName === file.filename) return;
    try {
        const res = await fetch(`/api/file/${file.id}/rename`, {
            method: 'PATCH',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ filename: newName })
        });
        if (!res.ok) throw new Error('Failed');
        file.filename = newName;
        renderFiles();
        showToast('File renamed successfully');
    } catch (err) {
        showToast('Failed to rename file');
    }
}

async function showMovePrompt(file) {
    const targetFolder = await showFolderSelectModal(file.folder || '/');
    if (targetFolder === null || targetFolder === file.folder) return;
    try {
        const res = await fetch(`/api/file/${file.id}/move`, {
            method: 'PATCH',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ folder: targetFolder })
        });
        if (!res.ok) throw new Error('Failed');
        file.folder = targetFolder;
        renderFiles();
        showToast('File moved successfully');
    } catch (err) {
        showToast('Failed to move file');
    }
}

async function showDeleteConfirm(file) {
    if (!confirm(`Are you sure you want to delete "${file.filename}"?`)) return;
    try {
        const res = await fetch(`/api/file/${file.id}`, {
            method: 'DELETE',
            credentials: 'include'
        });
        if (!res.ok) throw new Error('Failed');
        allFiles = allFiles.filter(f => f.id !== file.id);
        renderFiles();
        showToast('File deleted');
    } catch (err) {
        showToast('Failed to delete file');
    }
}

function showPromptModal(title, initialValue) {
    return new Promise(resolve => {
        const dialog = document.createElement('div');
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-in fade-in';
        dialog.innerHTML = `
            <div class="bg-card border border-border dark:border-slate-800 rounded-2xl p-6 shadow-2xl max-w-sm w-full mx-4 space-y-4 animate-in zoom-in-95">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">${title}</h3>
                <input type="text" id="prompt-input" value="${escapeHtml(initialValue)}" class="flex h-10 w-full rounded-xl border border-input dark:border-slate-800 bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" autofocus />
                <div class="flex justify-end gap-2.5">
                    <button id="prompt-cancel" class="inline-flex items-center justify-center rounded-xl text-sm font-medium border border-input bg-background h-9 px-4 hover:bg-accent">Cancel</button>
                    <button id="prompt-ok" class="inline-flex items-center justify-center rounded-xl text-sm font-medium bg-primary text-primary-foreground shadow h-9 px-4 hover:bg-primary/90">Save</button>
                </div>
            </div>
        `;
        document.body.appendChild(dialog);
        const input = dialog.querySelector('#prompt-input');
        const cancelBtn = dialog.querySelector('#prompt-cancel');
        const okBtn = dialog.querySelector('#prompt-ok');

        function done(val) {
            dialog.remove();
            resolve(val ? val.trim() : null);
        }

        cancelBtn.onclick = () => done(null);
        okBtn.onclick = () => done(input.value);
        input.onkeydown = (e) => {
            if (e.key === 'Enter') done(input.value);
            if (e.key === 'Escape') done(null);
        };
        input.focus();
        input.select();
    });
}

function showFolderSelectModal(currentFolder) {
    return new Promise(resolve => {
        const dialog = document.createElement('div');
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-in fade-in';
        const optionsHtml = allFolders.map(f => `<option value="${escapeHtml(f)}" ${f === currentFolder ? 'selected' : ''}>${escapeHtml(f)}</option>`).join('');
        dialog.innerHTML = `
            <div class="bg-card border border-border dark:border-slate-800 rounded-2xl p-6 shadow-2xl max-w-sm w-full mx-4 space-y-4 animate-in zoom-in-95">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">Move to Folder</h3>
                <select id="folder-select" class="flex h-10 w-full rounded-xl border border-input dark:border-slate-800 bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
                    ${optionsHtml}
                </select>
                <div class="flex justify-end gap-2.5">
                    <button id="folder-sel-cancel" class="inline-flex items-center justify-center rounded-xl text-sm font-medium border border-input bg-background h-9 px-4 hover:bg-accent">Cancel</button>
                    <button id="folder-sel-ok" class="inline-flex items-center justify-center rounded-xl text-sm font-medium bg-primary text-primary-foreground shadow h-9 px-4 hover:bg-primary/90">Move</button>
                </div>
            </div>
        `;
        document.body.appendChild(dialog);
        const select = dialog.querySelector('#folder-select');
        const cancelBtn = dialog.querySelector('#folder-sel-cancel');
        const okBtn = dialog.querySelector('#folder-sel-ok');

        function done(val) {
            dialog.remove();
            resolve(val);
        }

        cancelBtn.onclick = () => done(null);
        okBtn.onclick = () => done(select.value);
    });
}

function showToast(message) {
    const toast = document.createElement('div');
    toast.className = 'fixed bottom-4 right-4 z-[100] rounded-xl bg-foreground text-background px-4 py-2.5 text-sm font-medium shadow-2xl animate-in fade-in slide-in-from-bottom-4';
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
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors h-8 w-8 bg-background text-foreground shadow-sm';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors text-muted-foreground hover:text-foreground h-8 w-8';
    } else {
        if (gridEl) gridEl.classList.add('hidden');
        if (listEl) listEl.classList.remove('hidden');
        if (btnGrid) btnGrid.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors text-muted-foreground hover:text-foreground h-8 w-8';
        if (btnList) btnList.className = 'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors h-8 w-8 bg-background text-foreground shadow-sm';
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
        const playBtn = cat === 'video' ? '<div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"><div class="flex h-10 w-10 items-center justify-center rounded-full bg-black/60 backdrop-blur-md shadow-lg text-white"><svg class="h-5 w-5 ml-0.5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></div></div>' : '';
        return `<div onclick="openFileById('${f.id}')" class="group relative flex-shrink-0 w-48 cursor-pointer rounded-2xl border bg-card border-border dark:border-slate-800/80 text-card-foreground shadow-sm hover:shadow-xl hover:border-indigo-500/50 transition-all duration-200 overflow-hidden">
            <div class="aspect-video relative overflow-hidden bg-slate-100 dark:bg-slate-900/60">
                ${thumbHtml}
                ${playBtn}
            </div>
            <div class="p-3 space-y-1">
                <h4 class="text-xs font-semibold leading-snug truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                <div class="flex items-center gap-1.5">
                    <span class="text-[9px] font-semibold text-indigo-600 dark:text-indigo-400 bg-indigo-500/10 dark:bg-indigo-500/15 border border-indigo-500/20 px-1.5 py-0.5 rounded uppercase">${badge}</span>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 tabular-nums">${formatSize(f.size)}</span>
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
        contentHtml = `<video controls autoplay preload="metadata" src="/api/file/${file.id}" class="w-full h-full max-h-[75vh] sm:max-h-[82vh] object-contain rounded-xl"></video>`;
    } else if (type === 'image') {
        contentHtml = `<img src="/api/file/${file.id}" alt="${escapeHtml(file.filename)}" class="max-w-full max-h-[75vh] sm:max-h-[82vh] object-contain rounded-xl" />`;
    } else if (type === 'document') {
        contentHtml = `<iframe src="/api/file/${file.id}" class="w-full h-full border-0 rounded-xl" style="min-height:65vh"></iframe>`;
    }

    root.innerHTML = `
      <div data-viewer-overlay class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md animate-in fade-in transition-opacity"></div>
      <div data-viewer-content class="fixed inset-2 sm:inset-4 md:inset-8 lg:inset-16 z-50 flex flex-col max-h-[96vh] sm:max-h-[90vh] animate-in zoom-in-95">
        <div class="flex items-center justify-between mb-3 px-2">
          <div class="min-w-0 flex-1 pr-2">
            <h2 class="text-base sm:text-lg font-semibold text-white/95 truncate">${escapeHtml(displayName)}</h2>
            <p class="text-xs sm:text-sm text-white/60 tabular-nums">${formatSize(file.size)} &middot; ${getTypeBadge(file.mime_type)}</p>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <a href="/api/file/${file.id}" download class="inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-xl text-xs sm:text-sm font-medium transition-colors bg-white/10 text-white/95 hover:bg-white/20 h-9 px-4 backdrop-blur-md border border-white/10">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>
              <span class="hidden sm:inline">Download</span>
            </a>
            <button data-viewer-close class="inline-flex h-9 w-9 items-center justify-center rounded-xl text-sm font-medium transition-colors bg-white/10 text-white/95 hover:bg-white/20 backdrop-blur-md border border-white/10 cursor-pointer" title="Close">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6 6 18M6 6l12 12"></path></svg>
            </button>
          </div>
        </div>
        <div class="flex-1 flex items-center justify-center overflow-hidden rounded-2xl bg-black/60 backdrop-blur-xl border border-white/15 p-2 sm:p-4 shadow-2xl">
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
