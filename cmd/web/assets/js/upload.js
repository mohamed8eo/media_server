function initUploadPage() {
    const dropzone = document.getElementById('dropzone');
    const fileInput = document.getElementById('file-input');
    const folderSelect = document.getElementById('upload-folder-select');

    if (folderSelect) {
        loadFolderOptions(folderSelect);
    }

    if (!dropzone || !fileInput) return;

    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        dropzone.addEventListener(eventName, preventDefaults, false);
        document.body.addEventListener(eventName, preventDefaults, false);
    });

    ['dragenter', 'dragover'].forEach(eventName => {
        dropzone.classList.add('border-primary', 'bg-muted/50');
    });

    ['dragleave', 'drop'].forEach(eventName => {
        dropzone.classList.remove('border-primary', 'bg-muted/50');
    });

    dropzone.addEventListener('drop', (e) => {
        handleFiles(e.dataTransfer.files);
    }, false);

    fileInput.addEventListener('change', (e) => {
        handleFiles(e.target.files);
    });
}

async function loadFolderOptions(selectEl) {
    try {
        const res = await fetch('/api/file/', { method: 'GET', credentials: 'include' });
        if (!res.ok) return;
        const data = await res.json();
        const folders = data.folders || ['/'];
        if (!folders.includes('/')) folders.unshift('/');

        const urlFolder = new URLSearchParams(window.location.search).get('folder');

        let html = '';
        folders.forEach(folder => {
            const selected = folder === urlFolder ? ' selected' : '';
            const depth = folder === '/' ? 0 : folder.split('/').length - 1;
            const prefix = depth > 0 ? '&nbsp;&nbsp;'.repeat(depth) + '↳ ' : '';
            const displayLabel = folder === '/' ? '/' : prefix + folder.split('/').pop();
            html += '<option value="' + escapeHtml(folder) + '"' + selected + '>' + displayLabel + '</option>';
        });
        selectEl.innerHTML = html;
    } catch (e) {}
}

function setupDropOverlay() {
    const overlay = document.getElementById('drop-overlay');
    if (!overlay) return;
    let dragCount = 0;
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        document.addEventListener(eventName, preventDefaults, false);
    });
    document.addEventListener('dragenter', () => { dragCount++; if (dragCount === 1) overlay.classList.remove('hidden'); });
    document.addEventListener('dragleave', () => { dragCount--; if (dragCount <= 0) { dragCount = 0; overlay.classList.add('hidden'); } });
    document.addEventListener('drop', (e) => {
        dragCount = 0; overlay.classList.add('hidden');
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            window.location.href = '/upload';
        }
    });
}

function preventDefaults(e) { e.preventDefault(); e.stopPropagation(); }

let uploadedCount = 0;
let totalUploads = 0;

function handleFiles(files) {
    if (!files || files.length === 0) return;
    const container = document.getElementById('upload-queue-container');
    const listEl = document.getElementById('upload-queue-list');
    if (container) container.classList.remove('hidden');

    const newFiles = Array.from(files);
    totalUploads += newFiles.length;

    newFiles.forEach((file, index) => {
        const id = 'file-item-' + Date.now() + '-' + index;
        if (listEl) listEl.insertAdjacentHTML('beforeend',
            '<div id="' + id + '" class="rounded-xl border bg-card text-card-foreground shadow-sm p-4 space-y-3"><div class="flex items-center justify-between"><div class="flex items-center gap-3 min-w-0"><div class="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground shrink-0"><svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path></svg></div><div class="min-w-0"><h4 class="text-sm font-medium truncate">' + escapeHtml(file.name) + '</h4><p class="text-xs text-muted-foreground">' + formatSize(file.size) + '</p></div></div><span id="' + id + '-status" class="text-xs text-muted-foreground">Waiting</span></div><div class="h-1.5 w-full overflow-hidden rounded-full bg-primary/20"><div id="' + id + '-progress" class="h-full w-0 rounded-full bg-primary transition-all duration-300"></div></div></div>'
        );
        uploadFile(file, id);
    });
}

function uploadFile(file, itemId) {
    const statusEl = document.getElementById(itemId + '-status');
    const progressEl = document.getElementById(itemId + '-progress');
    const folderSelect = document.getElementById('upload-folder-select');
    const folder = folderSelect ? folderSelect.value : '/';

    const formData = new FormData();
    formData.append('file', file);
    formData.append('folder', folder);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/file/', true);
    xhr.withCredentials = true;

    xhr.upload.onprogress = function(e) {
        if (e.lengthComputable) {
            const pct = Math.round((e.loaded / e.total) * 100);
            if (progressEl) progressEl.style.width = pct + '%';
            if (statusEl) statusEl.textContent = pct + '%';
        }
    };

    xhr.onload = function() {
        if (xhr.status >= 200 && xhr.status < 300) {
            if (statusEl) { statusEl.className = 'text-xs font-medium text-emerald-600 dark:text-emerald-400'; statusEl.textContent = 'Done'; }
            if (progressEl) progressEl.className = 'h-full rounded-full bg-emerald-500 transition-all duration-300';
        } else if (xhr.status === 401) {
            window.location.href = '/sign-in'; return;
        } else {
            let errText = 'Failed';
            try { const r = JSON.parse(xhr.responseText); if (r.error) errText = r.error; } catch (e) {}
            if (statusEl) { statusEl.className = 'text-xs font-medium text-destructive'; statusEl.textContent = errText; }
            if (progressEl) progressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
        }
        uploadedCount++;
        updateUploadStatus();
    };

    xhr.onerror = function() {
        if (statusEl) { statusEl.className = 'text-xs font-medium text-destructive'; statusEl.textContent = 'Network error'; }
        if (progressEl) progressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
        uploadedCount++;
        updateUploadStatus();
    };

    xhr.send(formData);
}

function updateUploadStatus() {
    const el = document.getElementById('upload-status-text');
    if (el) el.textContent = uploadedCount + ' of ' + totalUploads + ' uploaded';
}

function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}
