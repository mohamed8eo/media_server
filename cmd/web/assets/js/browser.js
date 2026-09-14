let allFiles = [];
let allFolders = ['/'];
let folderItems = [];
let allJobs = [];
let selectedItems = new Set();
let selectionAnchor = null;
let isTrashView = new URLSearchParams(window.location.search).get('trash') === '1';
let currentView = localStorage.getItem('file_view') || 'grid';
let currentFilter = new URLSearchParams(window.location.search).get('type') || 'all';
let currentFolder = new URLSearchParams(window.location.search).get('folder') || '/';

document.addEventListener('mediaUpdated', () => {
    if (typeof fetchFiles === 'function') {
        fetchFiles();
    }
});

document.addEventListener('keydown', (e) => {
    const isInput = e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable;

    if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'n') {
        if (!isTrashView) {
            e.preventDefault();
            createNewFolder();
        }
        return;
    }

    if (((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') || (e.key === '/' && !isInput)) {
        const searchInput = document.getElementById('file-search');
        if (searchInput) {
            e.preventDefault();
            searchInput.focus();
            searchInput.select();
        }
        return;
    }

    if (isInput) return;

    if (e.key === 'Escape') {
        if (selectedItems.size > 0) {
            clearSelection();
            renderAll();
            e.preventDefault();
        }
        return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
        e.preventDefault();
        selectedItems.clear();
        const files = getFilesInFolder(currentFolder);
        const folders = getFoldersInFolder(currentFolder);
        folders.forEach(f => {
            const folderId = (folderItems.find(item => item.path === f) || {}).id;
            if (folderId) selectedItems.add(itemKey('folder', folderId));
        });
        files.forEach(file => {
            selectedItems.add(itemKey('file', file.id));
        });
        renderAll();
        return;
    }

    if ((e.key === 'Delete' || e.key === 'Backspace') && selectedItems.size > 0) {
        e.preventDefault();
        if (isTrashView) {
            batchPurge();
        } else {
            batchDelete();
        }
        return;
    }
});

function initFileBrowser() {
    isTrashView = new URLSearchParams(window.location.search).get('trash') === '1';
    currentFilter = new URLSearchParams(window.location.search).get('type') || 'all';
    currentFolder = new URLSearchParams(window.location.search).get('folder') || '/';
    setFileView(currentView, false);
    setFilter(currentFilter, false);
    if (typeof updateSidebarActive === 'function') updateSidebarActive();
    if (typeof updateNavActive === 'function') updateNavActive();
    if (allFiles.length > 0) {
        renderAll();
        fetchFiles();
    } else {
        fetchFiles();
    }

    const searchInput = document.getElementById('file-search');
    if (searchInput) searchInput.addEventListener('input', () => renderFiles());
    const sortSelect = document.getElementById('file-sort');
    if (sortSelect) sortSelect.addEventListener('change', () => renderFiles());

    document.addEventListener('click', (e) => {
        if (!e.target.closest('[data-dropdown-menu]')) {
            document.querySelectorAll('[data-dropdown-content]').forEach(el => {
                el.classList.add('hidden');
                const parentCard = el.closest('[data-file-id]') || el.closest('.folder-card') || el.closest('.group');
                if (parentCard) parentCard.classList.remove('z-30', 'z-[100]', 'relative');
            });
        }
    });
}

function itemKey(kind, id) { return kind + ':' + id; }

const GRADIENT_PALETTE = [
    'from-blue-500 to-cyan-400',
    'from-red-500 to-orange-400',
    'from-emerald-500 to-green-400',
    'from-purple-500 to-fuchsia-400',
    'from-amber-500 to-yellow-400',
    'from-pink-500 to-rose-400',
    'from-cyan-500 to-sky-400',
    'from-indigo-500 to-violet-400',
];

function gradientForKey(key) {
    let hash = 0;
    const str = String(key || '');
    for (let i = 0; i < str.length; i++) {
        hash = (hash * 31 + str.charCodeAt(i)) >>> 0;
    }
    return GRADIENT_PALETTE[hash % GRADIENT_PALETTE.length];
}

function visibleItems() { return [...getFoldersInFolder(currentFolder).map(p => ({kind:'folder',id:(folderItems.find(f=>f.path===p)||{}).id})).filter(x=>x.id), ...getFilesInFolder(currentFolder).map(f=>({kind:'file',id:f.id}))]; }
function toggleSelection(kind,id,checked,shift) { const items=visibleItems(),key=itemKey(kind,id),i=items.findIndex(x=>itemKey(x.kind,x.id)===key); if(shift&&selectionAnchor!==null&&i>=0){const [a,b]=[Math.min(selectionAnchor,i),Math.max(selectionAnchor,i)];items.slice(a,b+1).forEach(x=>checked?selectedItems.add(itemKey(x.kind,x.id)):selectedItems.delete(itemKey(x.kind,x.id)));}else{checked?selectedItems.add(key):selectedItems.delete(key);selectionAnchor=i;}renderAll(); }
function clearSelection(){selectedItems.clear();selectionAnchor=null;renderAll();}
function selectedPayload(){return [...selectedItems].map(key=>{const [kind,id]=key.split(':');return {kind,id};});}
function renderSelectionBar(){const bar=document.getElementById('selection-bar'),count=document.getElementById('selection-count');if(!bar||!count)return;const n=selectedItems.size;bar.classList.toggle('hidden',!n);bar.classList.toggle('flex',!!n);count.textContent=n+' selected';const b=bar.querySelectorAll('button');if(isTrashView){b[0].textContent='Restore';b[0].onclick=batchRestore;b[1].textContent='Permanently delete';b[1].onclick=batchPurge;b[2].classList.add('hidden');}else{b[0].textContent='Move';b[0].onclick=batchMove;b[1].textContent='Download ZIP';b[1].onclick=batchDownload;b[2].classList.remove('hidden');}}
async function batchRequest(url,method,extra={}){try{const res=await fetch(url,{method,credentials:'include',headers:{'Content-Type':'application/json'},body:JSON.stringify({items:selectedPayload(),...extra})});if(!res.ok)throw new Error();clearSelection();await fetchFiles();showToast('Operation completed');}catch(e){showToast('Operation failed');}}
async function batchDelete(){if(await showConfirmModal('Move to Recycle Bin',`Move ${selectedItems.size} item(s) to the recycle bin?`))await batchRequest('/api/file/batch/delete','POST');}
async function batchMove(){const folder=await showFolderSelectModal('/');if(folder!==null)await batchRequest('/api/file/batch/move','PATCH',{folder});}
async function batchDownload(){const res=await fetch('/api/file/batch/download',{method:'POST',credentials:'include',headers:{'Content-Type':'application/json'},body:JSON.stringify({items:selectedPayload()})});if(!res.ok){showToast('Download failed');return;}const blob=await res.blob(),a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='media-download.zip';a.click();URL.revokeObjectURL(a.href);}
async function batchRestore(){await batchRequest('/api/file/trash/restore','POST');}
async function batchPurge(){if(await showConfirmModal('Permanently delete','This cannot be undone.'))await batchRequest('/api/file/trash/purge','POST');}

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
        document.body.dispatchEvent(new CustomEvent('mediaUpdated'));
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
    if (isTrashView) return allFolders.filter(f => f !== '/').sort();
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
    const folderId = (folderItems.find(f => f.path === folder) || {}).id;
    const checked = folderId && selectedItems.has(itemKey('folder', folderId)) ? 'checked' : '';
    return `<div data-folder-path="${escapeHtml(folder)}" x-data="{ menuOpen: false }" :class="menuOpen ? 'z-30 relative' : ''" class="group relative cursor-pointer rounded-xl border bg-card border-border text-card-foreground p-4 shadow-sm transition hover:-translate-y-0.5 hover:border-primary/40 flex items-center justify-between folder-card">
		${folderId ? `<input type="checkbox" data-select-kind="folder" data-select-id="${folderId}" ${checked} class="absolute left-3 top-3 h-4 w-4 z-10" />` : ''}
        <div class="flex items-center gap-3.5 min-w-0">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-accent text-primary">
                <svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M2.75 12.75V12A2.25 2.25 0 0 1 5 9.75h14A2.25 2.25 0 0 1 21.25 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z"/></svg>
            </div>
            <div class="min-w-0">
                <h4 class="text-sm font-semibold truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(folder)}">${escapeHtml(name)}</h4>
                <p class="text-xs text-muted-foreground">${count} items</p>
            </div>
        </div>
        ${folderId ? `<div class="relative shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" :class="menuOpen ? 'opacity-100' : ''">
            <button @click.stop="menuOpen = !menuOpen" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
            </button>
            <div x-show="menuOpen" @click.outside="menuOpen = false" x-transition class="absolute right-0 top-full mt-1 w-40 rounded-xl bg-card border border-border dark:border-slate-800 shadow-2xl py-1.5 z-[100] text-xs font-medium">
                <button data-action="rename-folder" @click="menuOpen = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2">Rename</button>
                <button data-action="delete-folder" @click="menuOpen = false" class="w-full text-left px-3.5 py-2 text-destructive hover:bg-destructive/10 flex items-center gap-2">Delete folder</button>
            </div>
        </div>` : ''}
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

function handleFolderAction(action, folderPath) {
    if (action === 'rename-folder') {
        showRenameFolderPrompt(folderPath);
    } else if (action === 'delete-folder') {
        showDeleteFolderConfirm(folderPath);
    }
}

async function showRenameFolderPrompt(folderPath) {
    const folderId = (folderItems.find(f => f.path === folderPath) || {}).id;
    if (!folderId) return;
    const currentName = folderPath.split('/').pop();
    const newName = await showPromptModal('Rename Folder', currentName);
    if (!newName || newName === currentName) return;
    try {
        const res = await fetch(`/api/file/folder/${folderId}/rename`, {
            method: 'PATCH',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: newName })
        });
        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || 'Failed to rename folder');
        }
        await fetchFiles();
        showToast('Folder renamed successfully');
    } catch (err) {
        showToast(err.message || 'Failed to rename folder');
    }
}

async function showDeleteFolderConfirm(folderPath) {
    const folderId = (folderItems.find(f => f.path === folderPath) || {}).id;
    if (!folderId) return;
    const name = folderPath.split('/').pop();
    const confirmed = await showConfirmModal('Delete Folder', `Move "${name}" and everything inside it to the recycle bin?`);
    if (!confirmed) return;
    try {
        const res = await fetch('/api/file/batch/delete', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ items: [{ kind: 'folder', id: folderId }] })
        });
        if (!res.ok) throw new Error('Failed');
        selectedItems.delete(itemKey('folder', folderId));
        if (currentFolder === folderPath || currentFolder.startsWith(folderPath + '/')) {
            const parent = folderPath.split('/').slice(0, -1).join('/');
            navigateToFolder(parent || '/');
        }
        await fetchFiles();
        showToast('Folder moved to recycle bin');
    } catch (err) {
        showToast('Failed to delete folder');
    }
}

function renderAll() {
    renderSelectionBar();
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
    if (isTrashView) return allFiles;
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
    if (mime.includes('zip') || mime.includes('tar') || mime.includes('7z') || mime.includes('rar') || mime.includes('gzip') || mime.includes('x-compress')) return 'archive';
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
    if (typeof updateNavActive === 'function') updateNavActive();
    renderFiles();
}

function updateActiveFilterTab() {
    const titles = { all: 'All Media', video: 'Videos', image: 'Images', document: 'Documents', audio: 'Audio' };
    const titleEl = document.getElementById('section-title');
    if (titleEl) titleEl.textContent = isTrashView ? 'Recycle Bin' : (titles[currentFilter] || 'All Media');
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

    const totalCount = folders.length + files.length + allJobs.length;
    if (fileCountEl) fileCountEl.textContent = totalCount + ' item' + (totalCount !== 1 ? 's' : '');
    if (folderCountEl) folderCountEl.textContent = folders.length + ' folder' + (folders.length !== 1 ? 's' : '');
    if (fileItemsCountEl) fileItemsCountEl.textContent = (files.length + allJobs.length) + ' file' + ((files.length + allJobs.length) !== 1 ? 's' : '');

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
					var checkbox = e.target.closest('[data-select-kind]');
					if (checkbox) { e.stopPropagation(); toggleSelection(checkbox.dataset.selectKind, checkbox.dataset.selectId, checkbox.checked, e.shiftKey); return; }
                    var folderCard = e.target.closest('[data-folder-path]');
                    if (!folderCard) return;
                    var action = e.target.closest('[data-action]');
                    if (action) {
                        e.stopPropagation();
                        handleFolderAction(action.dataset.action, folderCard.dataset.folderPath);
                        return;
                    }
                    navigateToFolder(folderCard.dataset.folderPath);
                });
                folderGridEl._delegationBound = true;
            }
        }
    } else {
        if (folderSectionEl) folderSectionEl.classList.add('hidden');
        if (folderGridEl) folderGridEl.innerHTML = '';
    }

    if (files.length > 0 || allJobs.length > 0) {
        if (fileSectionEl) fileSectionEl.classList.remove('hidden');
        const foldersListHtml = folders.map(f => renderFolderRow(f)).join('');
        const jobsHtml = allJobs.map(j => renderJobCard(j)).join('');
        const jobsListHtml = allJobs.map(j => renderJobRow(j)).join('');
        const filesHtml = jobsHtml + files.map(f => renderGridCard(f)).join('');
        const filesListHtml = jobsListHtml + files.map(f => renderListRow(f)).join('');

        if (gridEl) {
            gridEl.innerHTML = filesHtml;
            if (!gridEl._delegationBound) {
                gridEl.addEventListener('click', function(e) {
					var checkbox = e.target.closest('[data-select-kind]');
					if (checkbox) { e.stopPropagation(); toggleSelection(checkbox.dataset.selectKind, checkbox.dataset.selectId, checkbox.checked, e.shiftKey); return; }
                    var card = e.target.closest('[data-file-id]');
                    if (!card) return;
                    var action = e.target.closest('[data-action]');
                    if (action) {
                        e.stopPropagation();
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
					var checkbox = e.target.closest('[data-select-kind]');
					if (checkbox) { e.stopPropagation(); toggleSelection(checkbox.dataset.selectKind, checkbox.dataset.selectId, checkbox.checked, e.shiftKey); return; }
                    var row = e.target.closest('[data-file-id]');
                    if (!row) return;
                    var action = e.target.closest('[data-action]');
                    if (action) {
                        e.stopPropagation();
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

    return `<div data-file-id="${f.id}" 
        x-data="{ showActions: window.matchMedia('(hover: none)').matches, menuOpen: false }"
        @mouseenter="showActions = true"
        @mouseleave="if (!menuOpen) showActions = false"
        class="bg-card border border-border shadow-sm group relative min-w-0 overflow-hidden rounded-xl cursor-pointer"
        :class="menuOpen ? 'z-50 relative' : 'z-10 relative'">
        <input type="checkbox" data-select-kind="file" data-select-id="${f.id}" ${selectedItems.has(itemKey('file', f.id)) ? 'checked' : ''} class="absolute left-3 top-3 z-30 h-4 w-4" />
        <div class="aspect-[16/10] relative overflow-hidden flex items-center justify-center">
            ${thumbHtml}
            <div class="absolute inset-0 bg-gradient-to-t from-black/70 via-black/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200"></div>
        </div>
        <div 
            x-show="showActions || menuOpen"
            x-transition:enter="transition ease-out duration-150"
            x-transition:enter-start="opacity-0 translate-y-1"
            x-transition:enter-end="opacity-100 translate-y-0"
            class="absolute right-2 top-2 z-30 flex max-w-[calc(100%-1rem)] items-center gap-1 rounded-xl border border-border bg-background/90 p-1 shadow-lg backdrop-blur-md dark:border-slate-800 dark:bg-slate-900/90">
            ${actions}
        </div>
        <div class="p-4 space-y-1">
            <div class="flex min-w-0 items-start justify-between gap-2">
                <div class="min-w-0">
                    <h4 class="text-sm font-semibold truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
                    <p class="mt-1 truncate text-xs text-muted-foreground">${formatSize(f.size)} &middot; ${formatDate(f.created_at)}</p>
                </div>
                <span class="shrink-0 rounded-md bg-accent px-2 py-1 text-[10px] font-bold text-accent-foreground">${badge}</span>
            </div>
            <p class="mt-3 truncate text-xs text-muted-foreground">MediaVault ${escapeHtml(normalizeFolder(f.folder || '/').replace(/^\//, '/ ').replace(/\//g, ' / '))}</p>
        </div>
    </div>`;
}

function renderListRow(f) {
    const thumbHtml = getThumbnailHtmlSmall(f);
    const badge = getTypeBadge(f.mime_type);
    const displayName = getCleanName(f.filename);
    const actions = getFileActionsInline(f);

    return `<div data-file-id="${f.id}" 
        x-data="{ showActions: window.matchMedia('(hover: none)').matches, menuOpen: false }"
        @mouseenter="showActions = true"
        @mouseleave="if (!menuOpen) showActions = false"
        class="relative group flex min-w-0 items-center gap-3 px-3 py-3 transition-colors hover:bg-accent/50 sm:gap-4 sm:px-4 cursor-pointer border-b border-border/70 dark:border-slate-800/60 last:border-0"
        :class="menuOpen ? 'z-50 relative' : 'z-0 relative'">
        <input type="checkbox" data-select-kind="file" data-select-id="${f.id}" ${selectedItems.has(itemKey('file', f.id)) ? 'checked' : ''} class="h-4 w-4 shrink-0 rounded accent-primary" />
        <div class="w-11 h-11 sm:w-12 sm:h-12 rounded-xl overflow-hidden shrink-0 flex items-center justify-center shadow-sm">
            ${thumbHtml}
        </div>
        <div class="min-w-0 flex-1">
            <h4 class="text-sm font-semibold truncate text-slate-900 dark:text-slate-100" title="${escapeHtml(f.filename)}">${escapeHtml(displayName)}</h4>
            <p class="mt-0.5 text-xs text-muted-foreground truncate tabular-nums">${formatSize(f.size)} <span class="hidden sm:inline">&middot; ${formatDate(f.created_at)}</span></p>
        </div>
        <span class="hidden shrink-0 rounded-md bg-accent px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-accent-foreground sm:inline-flex">${badge}</span>
        <div 
            x-show="showActions || menuOpen"
            x-transition:enter="transition ease-out duration-150"
            x-transition:enter-start="opacity-0 translate-y-1"
            x-transition:enter-end="opacity-100 translate-y-0"
            class="z-30 flex shrink-0 items-center gap-0.5 sm:gap-1">
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
        if (f.mime_type && f.mime_type.includes('pdf')) {
            // PDFs get a real page-1 cover; fall back to the stylized card if generation failed
            return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getDocumentFallback('${f.id}','${f.mime_type}')" />`;
        }
        return getDocumentFallback(f.id, f.mime_type);
    }
    if (cat === 'archive') {
        // Archive files get their own amber "packed box" card instead of the generic file icon
        return `<div class="w-full h-full flex flex-col items-center justify-center bg-gradient-to-br from-amber-500 to-orange-600 text-white p-4 text-center gap-1.5">
            <svg class="h-10 w-10" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></svg>
            <span class="text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-black/20 backdrop-blur-sm">${getTypeBadge(f.mime_type)}</span>
        </div>`;
    }
    return `<div class="w-full h-full flex items-center justify-center">${getFallbackIcon(f.mime_type)}</div>`;
}

window.getDocumentFallback = function(fileId, mime) {
    const gradient = gradientForKey(fileId);
    return `<div class="w-full h-full flex flex-col items-center justify-center bg-gradient-to-br ${gradient} text-white p-4 text-center gap-1.5">
        <svg class="h-10 w-10" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>
        <span class="text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-black/20 backdrop-blur-sm">${getTypeBadge(mime)}</span>
    </div>`;
};

function getThumbnailHtmlSmall(f) {
    const cat = getCategoryFromMime(f.mime_type);
    const isPdf = f.mime_type && f.mime_type.includes('pdf');
    if (cat === 'image' || cat === 'video' || (cat === 'document' && isPdf)) {
        return `<img src="/api/file/${f.id}/thumb" alt="" loading="lazy" class="w-full h-full object-cover" onerror="this.parentElement.innerHTML=getColorFallbackSm('${f.id}','${f.mime_type}')" />`;
    }
    return getColorFallbackSm(f.id, f.mime_type);
}

window.getColorIconSm = function(mime) {
    const cat = getCategoryFromMime(mime);
    const icons = {
        video: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.777-.416L16 11"/><rect width="14" height="12" x="2" y="6" rx="2"/></svg>',
        image: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>',
        audio: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
        document: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>',
        archive: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></svg>',
        other: '<svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>'
    };
    return icons[cat] || icons.other;
};

window.getColorFallbackSm = function(fileId, mime) {
    const cat = getCategoryFromMime(mime);
    const gradient = cat === 'archive' ? 'from-amber-500 to-orange-600' : gradientForKey(fileId);
    return `<div class="w-full h-full flex items-center justify-center bg-gradient-to-br ${gradient}">${getColorIconSm(mime)}</div>`;
};

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
    if (cat === 'video' || cat === 'image' || cat === 'document' || cat === 'audio') {
        html += `<button data-action="play" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors"><svg class="h-4 w-4 pointer-events-none" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors"><svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="move" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors"><svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" x2="12" y1="11" y2="17"/><line x1="9" x2="15" y1="14" y2="14"/></svg></button>`;
    
    // More dropdown
    html += `<div data-dropdown-menu class="relative">
        <button data-action="more" @click="menuOpen = !menuOpen" class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-accent text-slate-700 dark:text-slate-200 transition-colors">
            <svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
        </button>
        <div x-show="menuOpen" @click.outside="menuOpen = false; showActions = false" x-transition class="absolute right-0 bottom-full mb-2 sm:bottom-auto sm:top-full sm:mt-2 w-48 rounded-xl bg-card border border-border dark:border-slate-800 shadow-2xl z-[100] p-1 text-xs font-medium">
            <button data-action="rename" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 rounded-lg hover:bg-accent hover:text-accent-foreground flex items-center gap-2">Rename</button>
            <button data-action="info" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 rounded-lg hover:bg-accent hover:text-accent-foreground flex items-center gap-2">More info</button>
            <button data-action="delete" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 rounded-lg text-destructive hover:bg-destructive/10 flex items-center gap-2">Delete file</button>
        </div>
    </div>`;

    return html;
}

function getFileActionsInline(f) {
    const cat = getCategoryFromMime(f.mime_type);
    const btnCls = 'inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors text-muted-foreground hover:bg-accent hover:text-foreground h-8 w-8';
    let html = '';
    if (cat === 'video' || cat === 'image' || cat === 'document' || cat === 'audio') {
        html += `<button data-action="play" class="${btnCls} hidden sm:inline-flex"><svg class="h-4 w-4 pointer-events-none" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg></button>`;
    }
    html += `<button data-action="download" class="${btnCls} hidden sm:inline-flex"><svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg></button>`;
    html += `<button data-action="move" class="${btnCls} hidden sm:inline-flex"><svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" x2="12" y1="11" y2="17"/><line x1="9" x2="15" y1="14" y2="14"/></svg></button>`;

    html += `<div data-dropdown-menu class="relative">
        <button data-action="more" @click="menuOpen = !menuOpen" class="${btnCls}">
            <svg class="h-4 w-4 pointer-events-none" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
        </button>
        <div x-show="menuOpen" @click.outside="menuOpen = false; showActions = false" x-transition class="absolute right-0 bottom-full mb-2 sm:bottom-auto sm:top-full sm:mt-1 w-44 rounded-xl bg-card border border-border dark:border-slate-800 shadow-2xl py-1.5 z-[100] text-xs font-medium">
            <button data-action="play" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2 sm:hidden">Open</button>
            <button data-action="download" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2 sm:hidden">Download</button>
            <button data-action="move" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2 sm:hidden">Move</button>
            <button data-action="rename" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2">Rename</button>
            <button data-action="info" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 hover:bg-accent hover:text-accent-foreground flex items-center gap-2">More info</button>
            <button data-action="delete" @click="menuOpen = false; showActions = false" class="w-full text-left px-3.5 py-2 text-destructive hover:bg-destructive/10 flex items-center gap-2">Delete file</button>
        </div>
    </div>`;

    return html;
}

function handleFileAction(action, fileId, e) {
    if (action === 'more') {
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
    dialog.className = 'fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-3 backdrop-blur-sm animate-in fade-in sm:p-4';
    dialog.innerHTML = `
        <div class="my-auto w-full max-w-md space-y-4 rounded-2xl border border-border bg-card p-4 shadow-2xl animate-in zoom-in-95 dark:border-slate-800 sm:p-6">
            <div class="flex items-center justify-between">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">File Information</h3>
                <button id="info-close" class="text-slate-500 hover:text-slate-700 dark:hover:text-slate-300">
                    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12"/></svg>
                </button>
            </div>
            <div class="space-y-3 text-sm text-slate-700 dark:text-slate-300">
                <div class="flex flex-col gap-1 border-b border-border/60 py-1 min-[400px]:flex-row min-[400px]:justify-between">
                    <span class="font-medium text-slate-500">Filename</span>
                    <span class="truncate max-w-[240px]" title="${escapeHtml(file.filename)}">${escapeHtml(file.filename)}</span>
                </div>
                <div class="flex flex-col gap-1 border-b border-border/60 py-1 min-[400px]:flex-row min-[400px]:justify-between">
                    <span class="font-medium text-slate-500">Size</span>
                    <span>${formatSize(file.size)}</span>
                </div>
                <div class="flex flex-col gap-1 border-b border-border/60 py-1 min-[400px]:flex-row min-[400px]:justify-between">
                    <span class="font-medium text-slate-500">Type</span>
                    <span>${escapeHtml(file.mime_type)}</span>
                </div>
                <div class="flex flex-col gap-1 border-b border-border/60 py-1 min-[400px]:flex-row min-[400px]:justify-between">
                    <span class="font-medium text-slate-500">Folder</span>
                    <span>${escapeHtml(file.folder)}</span>
                </div>
                <div class="flex flex-col gap-1 border-b border-border/60 py-1 min-[400px]:flex-row min-[400px]:justify-between">
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
    const confirmed = await showConfirmModal('Delete File', `Are you sure you want to delete "${file.filename}"?`);
    if (!confirmed) return;
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

function showConfirmModal(title, message) {
    return new Promise(resolve => {
        const dialog = document.createElement('div');
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-3 backdrop-blur-sm animate-in fade-in sm:p-4';
        dialog.innerHTML = `
            <div class="my-auto w-full max-w-sm space-y-4 rounded-2xl border border-border bg-card p-4 shadow-2xl animate-in zoom-in-95 dark:border-slate-800 sm:p-6">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">${title}</h3>
                <p class="text-sm text-slate-600 dark:text-slate-400">${escapeHtml(message)}</p>
                <div class="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end sm:gap-2.5">
                    <button id="confirm-cancel" class="inline-flex items-center justify-center rounded-xl text-sm font-medium border border-input bg-background h-9 px-4 hover:bg-accent">Cancel</button>
                    <button id="confirm-ok" class="inline-flex items-center justify-center rounded-xl text-sm font-medium bg-destructive text-destructive-foreground shadow h-9 px-4 hover:bg-destructive/90">Delete</button>
                </div>
            </div>
        `;
        document.body.appendChild(dialog);
        const cancelBtn = dialog.querySelector('#confirm-cancel');
        const okBtn = dialog.querySelector('#confirm-ok');

        function done(val) {
            dialog.remove();
            resolve(val);
        }

        cancelBtn.onclick = () => done(false);
        okBtn.onclick = () => done(true);
        dialog.onclick = (e) => { if (e.target === dialog) done(false); };
    });
}

function showPromptModal(title, initialValue) {
    return new Promise(resolve => {
        const dialog = document.createElement('div');
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-3 backdrop-blur-sm animate-in fade-in sm:p-4';
        dialog.innerHTML = `
            <div class="my-auto w-full max-w-sm space-y-4 rounded-2xl border border-border bg-card p-4 shadow-2xl animate-in zoom-in-95 dark:border-slate-800 sm:p-6">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">${title}</h3>
                <input type="text" id="prompt-input" value="${escapeHtml(initialValue)}" class="flex h-10 w-full rounded-xl border border-input dark:border-slate-800 bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" autofocus />
                <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end sm:gap-2.5">
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
        dialog.className = 'fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-3 backdrop-blur-sm animate-in fade-in sm:p-4';
        const optionsHtml = allFolders.map(f => `<option value="${escapeHtml(f)}" ${f === currentFolder ? 'selected' : ''}>${escapeHtml(f)}</option>`).join('');
        dialog.innerHTML = `
            <div class="my-auto w-full max-w-sm space-y-4 rounded-2xl border border-border bg-card p-4 shadow-2xl animate-in zoom-in-95 dark:border-slate-800 sm:p-6">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">Move to Folder</h3>
                <select id="folder-select" class="flex h-10 w-full rounded-xl border border-input dark:border-slate-800 bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
                    ${optionsHtml}
                </select>
                <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end sm:gap-2.5">
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
    toast.className = 'fixed bottom-20 left-3 right-3 z-[100] break-words rounded-xl bg-foreground px-4 py-2.5 text-sm font-medium text-background shadow-2xl animate-in fade-in slide-in-from-bottom-4 sm:bottom-4 sm:left-auto sm:right-4 sm:max-w-sm';
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
    if (allFiles.length === 0 && loadingEl) {
        loadingEl.classList.remove('hidden');
    }
    try {
        const res = await fetch(isTrashView ? '/api/file/trash' : '/api/file/', { method: 'GET', credentials: 'include' });
        if (!res.ok) { if (res.status === 401) return; throw new Error('Failed to fetch files'); }
        const data = await res.json();
        let filesChanged = false;
        let jobsCountChanged = false;
        if (isTrashView) {
            const items = data.items || [];
            const newFiles = items.filter(x => x.kind === 'file').map(x => ({...x, filename:x.name, folder:'/', mime_type:'application/octet-stream', created_at:x.deleted_at}));
            filesChanged = JSON.stringify(newFiles) !== JSON.stringify(allFiles);
            allFiles = newFiles;
            folderItems = items.filter(x => x.kind === 'folder').map(x => ({...x, path:x.path}));
            allFolders = folderItems.map(x => x.path);
        } else {
            const newFiles = data.files || [];
            const newFolders = (data.folders || []).map(x => typeof x === 'string' ? {id: x, path: x} : x);
            const newJobs = data.jobs || [];

            filesChanged = JSON.stringify(newFiles) !== JSON.stringify(allFiles) || JSON.stringify(newFolders) !== JSON.stringify(folderItems);
            jobsCountChanged = newJobs.length !== allJobs.length;

            allFiles = newFiles;
            folderItems = newFolders;
            allFolders = folderItems.map(x => x.path);
            allJobs = newJobs;
        }
        if (!allFolders.includes('/')) allFolders.unshift('/');
        if (loadingEl) loadingEl.classList.add('hidden');

        if (filesChanged || jobsCountChanged || !document.querySelector('[data-job-id]')) {
            renderAll();
        } else {
            updateJobsInPlace();
        }

        if (allJobs.length > 0) {
            setTimeout(() => { fetchFiles(); }, 2000);
        }
    } catch (err) {
        console.error(err);
        if (allFiles.length === 0 && loadingEl) {
            loadingEl.innerHTML = '<p class="text-sm text-destructive font-medium">Failed to load files. Please refresh.</p>';
        }
    }
}

function updateJobsInPlace() {
    allJobs.forEach(j => {
        const pct = j.progress || 0;
        document.querySelectorAll(`[data-job-id="${j.id}"]`).forEach(el => {
            el.querySelectorAll('[data-job-progress-bar]').forEach(bar => {
                bar.style.width = pct + '%';
            });
            el.querySelectorAll('[data-job-progress-text]').forEach(txt => {
                txt.textContent = pct + '%';
            });
            el.querySelectorAll('[data-job-progress-badge]').forEach(badge => {
                badge.textContent = pct + '%';
            });
        });
    });
}

window.openFileById = function(fileId) {
    const file = allFiles.find(f => f.id === fileId);
    if (file) openFile(file);
};

function openFile(file) {
    const cat = getCategoryFromMime(file.mime_type);
    if (cat === 'video') {
        window.location.href = '/watch/' + encodeURIComponent(file.id);
    } else if (cat === 'audio') {
        // Audio player - navigate to play page or direct link
        window.location.href = '/api/file/' + encodeURIComponent(file.id);
    } else if (file.mime_type && file.mime_type.toLowerCase().includes('pdf')) {
        // Mobile and tablet browsers commonly cannot render PDFs nested in an iframe.
        // Use the browser's native PDF reader (top-level navigation) for mobile/tablet,
        // and keep the viewer modal for desktop PC.
        const isMobileOrTablet = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini|Tablet|Mobile/i.test(navigator.userAgent) || window.innerWidth < 1024;
        if (isMobileOrTablet) {
            window.location.href = '/api/file/' + encodeURIComponent(file.id) + '/' + encodeURIComponent(file.filename);
        } else {
            showFileViewerModal(file, 'document');
        }
    } else if (cat === 'image' || cat === 'document') {
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
    } else if (type === 'audio') {
        contentHtml = `<audio controls preload="metadata" src="/api/file/${file.id}" class="w-full h-max max-h-[75vh] sm:max-h-[82vh] object-contain rounded-xl"></audio>`;
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

function renderJobCard(j) {
    const pct = j.progress || 0;
    const isMediaFix = j.task_type === 'media_fix';
    const badgeText = isMediaFix ? 'Media Fix' : `Downloading (${j.task_type})`;
    const titleText = isMediaFix ? 'Optimizing Video (Media Fix)...' : 'URL Download in progress...';
    return `<div data-job-id="${j.id}" class="relative group rounded-2xl border bg-card border-border dark:border-slate-800/80 text-card-foreground shadow-sm flex flex-col w-full p-5 space-y-4">
        <div class="flex items-center justify-between">
            <span class="inline-flex items-center rounded-md bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 px-2 py-0.5 text-xs font-semibold uppercase">${badgeText}</span>
            <span data-job-progress-text class="text-xs font-bold tabular-nums text-muted-foreground">${pct}%</span>
        </div>
        <div class="space-y-1">
            <h4 class="text-sm font-medium truncate">${titleText}</h4>
            <p class="text-xs text-muted-foreground capitalize">Status: ${j.status}</p>
        </div>
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-primary/20">
            <div data-job-progress-bar class="h-full rounded-full bg-primary transition-all duration-300" style="width: ${pct}%"></div>
        </div>
    </div>`;
}

function renderJobRow(j) {
    const pct = j.progress || 0;
    const isMediaFix = j.task_type === 'media_fix';
    const titleText = isMediaFix ? 'Media Fix (Optimizing Video)' : `URL Download (${j.task_type})`;
    const descText = isMediaFix ? 'Background video transcoding/remuxing' : 'Background yt-dlp task';
    return `<div data-job-id="${j.id}" class="flex items-center justify-between p-4">
        <div class="flex items-center gap-3">
            <div data-job-progress-badge class="h-10 w-10 rounded-xl bg-amber-500/10 text-amber-600 flex items-center justify-center font-bold text-xs">${pct}%</div>
            <div>
                <h4 class="text-sm font-medium">${titleText} - ${j.status}</h4>
                <p class="text-xs text-muted-foreground">${descText}</p>
            </div>
        </div>
        <div class="flex items-center gap-4">
            <div class="w-32 h-1.5 overflow-hidden rounded-full bg-primary/20">
                <div data-job-progress-bar class="h-full rounded-full bg-primary transition-all duration-300" style="width: ${pct}%"></div>
            </div>
            <span data-job-progress-text class="text-xs font-bold tabular-nums text-muted-foreground w-8 text-right">${pct}%</span>
        </div>
    </div>`;
}
