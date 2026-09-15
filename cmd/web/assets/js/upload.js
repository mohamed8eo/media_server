function initUploadPage() {
    const dropzone = document.getElementById('dropzone');
    const fileInput = document.getElementById('file-input');
    const folderSelect = document.getElementById('upload-folder-select');

    if (folderSelect) {
        loadFolderOptions(folderSelect);
    }

    initDownloadURL();

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

    dropzone.addEventListener('click', () => {
        fileInput.click();
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
        const rawFolders = data.folders || ['/'];
        const folders = rawFolders.map(x => typeof x === 'string' ? x : x.path);
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

function preventDefaults(e) { e.preventDefault(); e.stopPropagation(); }

let uploadedCount = 0;
let totalUploads = 0;
let uploadQueue = [];
let activeUploads = 0;
const MAX_CONCURRENT_UPLOADS = 4;

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
            '<div id="' + id + '" class="space-y-3 rounded-xl border bg-card p-3 text-card-foreground shadow-sm sm:p-4"><div class="flex min-w-0 flex-col gap-2 min-[400px]:flex-row min-[400px]:items-center min-[400px]:justify-between"><div class="flex min-w-0 items-center gap-3"><div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"><svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path></svg></div><div class="min-w-0"><h4 class="truncate text-sm font-medium">' + escapeHtml(file.name) + '</h4><p class="text-xs text-muted-foreground">' + formatSize(file.size) + '</p></div></div><span id="' + id + '-status" class="self-start text-xs text-muted-foreground min-[400px]:self-auto">Waiting</span></div><div class="h-1.5 w-full overflow-hidden rounded-full bg-primary/20"><div id="' + id + '-progress" class="h-full w-0 rounded-full bg-primary transition-all duration-300"></div></div></div>'
        );
        uploadQueue.push({ file, id });
    });

    processQueue();
}

function processQueue() {
    while (activeUploads < MAX_CONCURRENT_UPLOADS && uploadQueue.length > 0) {
        const item = uploadQueue.shift();
        activeUploads++;
        uploadFile(item.file, item.id, () => {
            activeUploads--;
            processQueue();
        });
    }
}

function uploadFile(file, itemId, onComplete) {
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

    const finish = () => {
        uploadedCount++;
        updateUploadStatus();
        if (typeof onComplete === 'function') onComplete();
    };

    xhr.onload = function() {
        if (xhr.status >= 200 && xhr.status < 300) {
            if (statusEl) { statusEl.className = 'text-xs font-medium text-emerald-600 dark:text-emerald-400'; statusEl.textContent = 'Done'; }
            if (progressEl) progressEl.className = 'h-full rounded-full bg-emerald-500 transition-all duration-300';
            document.body.dispatchEvent(new CustomEvent('mediaUpdated'));
        } else if (xhr.status === 401) {
            window.location.href = '/sign-in'; return;
        } else {
            let errText = 'Failed';
            try { const r = JSON.parse(xhr.responseText); if (r.error) errText = r.error; } catch (e) {}
            if (statusEl) { statusEl.className = 'text-xs font-medium text-destructive'; statusEl.textContent = errText; }
            if (progressEl) progressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
        }
        finish();
    };

    xhr.onerror = function() {
        if (statusEl) { statusEl.className = 'text-xs font-medium text-destructive'; statusEl.textContent = 'Network error'; }
        if (progressEl) progressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
        finish();
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

function initDownloadURL() {
    const btn = document.getElementById('download-url-btn');
    const input = document.getElementById('download-url-input');
    const qualitySelect = document.getElementById('download-quality-select');
    const folderSelect = document.getElementById('upload-folder-select');
    const statusEl = document.getElementById('download-url-status');

    if (!btn || !input) return;

    btn.addEventListener('click', async () => {
        const url = input.value.trim();
        if (!url) {
            if (statusEl) {
                statusEl.textContent = 'Please enter a valid URL';
                statusEl.className = 'text-xs font-medium text-destructive';
                statusEl.classList.remove('hidden');
            }
            return;
        }

        const quality = qualitySelect ? qualitySelect.value : 'best';
        const folder = folderSelect ? folderSelect.value : '/';

        btn.disabled = true;
        if (statusEl) {
            statusEl.textContent = 'Starting download...';
            statusEl.className = 'text-xs font-medium text-muted-foreground';
            statusEl.classList.remove('hidden');
        }

        const queueContainer = document.getElementById('upload-queue-container');
        const queueList = document.getElementById('upload-queue-list');
        if (queueContainer) queueContainer.classList.remove('hidden');

        totalUploads++;
        const itemId = 'url-item-' + Date.now();
        if (queueList) {
            queueList.insertAdjacentHTML('beforeend',
                '<div id="' + itemId + '" class="space-y-3 rounded-xl border bg-card p-3 text-card-foreground shadow-sm sm:p-4"><div class="flex min-w-0 flex-col gap-2 min-[400px]:flex-row min-[400px]:items-center min-[400px]:justify-between"><div class="flex min-w-0 items-center gap-3"><div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"><svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path></svg></div><div class="min-w-0"><h4 class="truncate text-sm font-medium">' + escapeHtml(url) + '</h4><p class="text-xs text-muted-foreground">Quality: ' + escapeHtml(quality) + ' (yt-dlp)</p></div></div><span id="' + itemId + '-status" class="self-start text-xs font-medium text-amber-600 dark:text-amber-400 min-[400px]:self-auto">0%</span></div><div class="h-1.5 w-full overflow-hidden rounded-full bg-primary/20"><div id="' + itemId + '-progress" class="h-full w-0 rounded-full bg-primary transition-all duration-300"></div></div></div>'
            );
        }
        updateUploadStatus();

        const itemStatusEl = document.getElementById(itemId + '-status');
        const itemProgressEl = document.getElementById(itemId + '-progress');

        try {
            const res = await fetch('/api/file/download-url', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({ url, quality, folder })
            });

            if (res.ok) {
                const data = await res.json();
                input.value = '';
                if (data.job_id) {
                    pollJobStatus(data.job_id, itemStatusEl, itemProgressEl, statusEl, () => {
                        uploadedCount++;
                        updateUploadStatus();
                    });
                } else {
                    uploadedCount++;
                    updateUploadStatus();
                }
            } else if (res.status === 401) {
                window.location.href = '/sign-in';
            } else {
                let errText = 'Failed to start download';
                try {
                    const r = await res.json();
                    if (r.error) errText = r.error;
                } catch (e) {}
                if (statusEl) {
                    statusEl.textContent = errText;
                    statusEl.className = 'text-xs font-medium text-destructive';
                    statusEl.classList.remove('hidden');
                }
                if (itemStatusEl) {
                    itemStatusEl.textContent = errText;
                    itemStatusEl.className = 'text-xs font-medium text-destructive';
                }
                if (itemProgressEl) {
                    itemProgressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
                }
                uploadedCount++;
                updateUploadStatus();
            }
        } catch (e) {
            if (statusEl) {
                statusEl.textContent = 'Network error';
                statusEl.className = 'text-xs font-medium text-destructive';
                statusEl.classList.remove('hidden');
            }
            if (itemStatusEl) {
                itemStatusEl.textContent = 'Network error';
                itemStatusEl.className = 'text-xs font-medium text-destructive';
            }
            if (itemProgressEl) {
                itemProgressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
            }
            uploadedCount++;
            updateUploadStatus();
        } finally {
            btn.disabled = false;
        }
    });
}

function pollJobStatus(jobId, itemStatusEl, itemProgressEl, statusEl, onFinished) {
    let finished = false;
    const interval = setInterval(async () => {
        if (finished) return;
        try {
            const res = await fetch('/api/file/jobs/' + jobId, {
                method: 'GET',
                credentials: 'include'
            });
            if (!res.ok) return;
            const job = await res.json();

            if (job.status === 'processing' || job.status === 'pending') {
                const pct = job.progress || 0;
                if (itemProgressEl) itemProgressEl.style.width = pct + '%';
                if (itemStatusEl) {
                    itemStatusEl.textContent = pct + '%';
                    itemStatusEl.className = 'text-xs font-medium text-amber-600 dark:text-amber-400';
                }
                if (statusEl) {
                    statusEl.textContent = 'Downloading... ' + pct + '%';
                    statusEl.className = 'text-xs font-medium text-muted-foreground';
                    statusEl.classList.remove('hidden');
                }
            } else if (job.status === 'completed') {
                finished = true;
                clearInterval(interval);
                if (itemProgressEl) {
                    itemProgressEl.style.width = '100%';
                    itemProgressEl.className = 'h-full rounded-full bg-emerald-500 transition-all duration-300';
                }
                if (itemStatusEl) {
                    itemStatusEl.textContent = 'Done';
                    itemStatusEl.className = 'text-xs font-medium text-emerald-600 dark:text-emerald-400';
                }
                if (statusEl) {
                    statusEl.textContent = 'Download completed successfully!';
                    statusEl.className = 'text-xs font-medium text-emerald-600 dark:text-emerald-400';
                    statusEl.classList.remove('hidden');
                }
                if (typeof onFinished === 'function') onFinished();
                document.body.dispatchEvent(new CustomEvent('mediaUpdated'));
            } else if (job.status === 'failed') {
                finished = true;
                clearInterval(interval);
                if (itemProgressEl) {
                    itemProgressEl.className = 'h-full rounded-full bg-destructive transition-all duration-300';
                }
                const err = job.error || 'Download failed';
                if (itemStatusEl) {
                    itemStatusEl.textContent = 'Failed';
                    itemStatusEl.className = 'text-xs font-medium text-destructive';
                }
                if (statusEl) {
                    statusEl.textContent = err;
                    statusEl.className = 'text-xs font-medium text-destructive';
                    statusEl.classList.remove('hidden');
                }
                if (typeof onFinished === 'function') onFinished();
            }
        } catch (e) {}
    }, 1500);
}

document.addEventListener('DOMContentLoaded', () => {
    if (document.getElementById('dropzone') || document.getElementById('file-input')) {
        initUploadPage();
    }
});
