// shadcn/ui AlertDialog - vanilla JS implementation
// Uses exact same HTML structure and Tailwind classes as shadcn/ui

function showDialog({ title, description, placeholder, defaultValue, confirmText, cancelText, variant }) {
  return new Promise((resolve) => {
    const root = document.getElementById('alert-dialog-root');
    if (!root) { resolve(null); return; }

    const btnVariant = variant === 'destructive'
      ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
      : 'bg-primary text-primary-foreground hover:bg-primary/90';

    root.innerHTML = `
      <div data-state="open" class="fixed inset-0 z-50 bg-black/80 animate-overlay-show" data-dialog-overlay></div>
      <div data-state="open" class="fixed left-[50%] top-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border bg-popover p-6 shadow-lg duration-200 animate-content-show sm:rounded-lg" data-dialog-content>
        <div class="flex flex-col space-y-2 text-center sm:text-left">
          <h2 class="text-lg font-semibold leading-none tracking-tight text-popover-foreground">${escapeHtml(title || '')}</h2>
          ${description ? `<p class="text-sm text-muted-foreground">${escapeHtml(description)}</p>` : ''}
        </div>
        ${placeholder !== undefined ? `
          <input id="dialog-input" type="${escapeHtml('text')}" placeholder="${escapeHtml(placeholder || '')}" value="${escapeHtml(defaultValue || '')}"
            class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50" />
        ` : ''}
        <div class="flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2">
          <button data-dialog-cancel class="inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-md bg-background px-4 py-2 text-sm font-medium text-popover-foreground shadow-sm transition-colors border border-input hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50">
            ${escapeHtml(cancelText || 'Cancel')}
          </button>
          <button data-dialog-confirm class="inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium shadow transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 ${btnVariant}">
            ${escapeHtml(confirmText || 'OK')}
          </button>
        </div>
      </div>
    `;

    const overlay = root.querySelector('[data-dialog-overlay]');
    const content = root.querySelector('[data-dialog-content]');
    const input = root.querySelector('#dialog-input');
    const confirmBtn = root.querySelector('[data-dialog-confirm]');
    const cancelBtn = root.querySelector('[data-dialog-cancel]');

    if (input) setTimeout(() => input.focus(), 50);

    function cleanup(result) {
      if (overlay) overlay.setAttribute('data-state', 'closed');
      if (content) content.setAttribute('data-state', 'closed');
      setTimeout(() => { root.innerHTML = ''; }, 150);
      cleanupListeners();
      resolve(result);
    }

    function onConfirm() {
      const val = input ? input.value : true;
      cleanup(val);
    }
    function onCancel() { cleanup(null); }
    function onKeydown(e) {
      if (e.key === 'Escape') { e.preventDefault(); cleanup(null); }
      if (e.key === 'Enter') { e.preventDefault(); onConfirm(); }
    }

    function cleanupListeners() {
      document.removeEventListener('keydown', onKeydown);
      if (confirmBtn) confirmBtn.removeEventListener('click', onConfirm);
      if (cancelBtn) cancelBtn.removeEventListener('click', onCancel);
      if (overlay) overlay.removeEventListener('click', onCancel);
    }

    document.addEventListener('keydown', onKeydown);
    if (confirmBtn) confirmBtn.addEventListener('click', onConfirm);
    if (cancelBtn) cancelBtn.addEventListener('click', onCancel);
    if (overlay) overlay.addEventListener('click', onCancel);
  });
}

async function showFolderPrompt() {
  const result = await showDialog({
    title: 'New folder',
    description: 'Enter a name for the new folder.',
    placeholder: 'Folder name',
    confirmText: 'Create',
    cancelText: 'Cancel',
  });
  if (result && result.trim()) {
    const clean = result.trim().replace(/[^a-zA-Z0-9_\-\s]/g, '').replace(/\s+/g, '-');
    if (clean) return clean;
  }
  return null;
}

async function showConfirmDialog({ title, description, confirmText, variant }) {
  const result = await showDialog({
    title: title || 'Are you sure?',
    description: description || '',
    confirmText: confirmText || 'Continue',
    cancelText: 'Cancel',
    variant: variant,
  });
  return result !== null;
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}
