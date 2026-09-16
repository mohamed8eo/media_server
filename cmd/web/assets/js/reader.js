const readerPage = document.getElementById('reader-page');
const cfg = {
    fileId: readerPage ? readerPage.dataset.fileId : '',
    mimeType: readerPage ? readerPage.dataset.fileMime : '',
    savedProgress: readerPage ? readerPage.dataset.fileProgress : '0',
};
if (!cfg.fileId) document.documentElement.innerHTML = '';

const fileUrl = `/api/file/${encodeURIComponent(cfg.fileId)}`;
const progressUrl = `/api/file/${encodeURIComponent(cfg.fileId)}/progress`;
const savedProgress = parseInt(cfg.savedProgress || '0', 10) || 0;

const $ = (sel) => document.querySelector(sel);
const pageview = $('#reader-pageview');
const scrollEl = $('#reader-scroll');
const canvas = pageview.querySelector('canvas');
const bookWrap = $('#reader-book');
const tapLeft = $('#reader-tap-left');
const tapRight = $('#reader-tap-right');
const prevBtn = $('#reader-prev');
const nextBtn = $('#reader-next');
const progressBar = $('#reader-progress');
const positionEl = $('#reader-position');
const loadingEl = $('#reader-loading');

const deviceRatio = Math.min(window.devicePixelRatio || 1, 2);
let currentMarker = savedProgress;
let savedMarker = savedProgress;
let lastSaveAt = 0;

function fail(message, error) {
    console.error(message, error || '');
    loadingEl.hidden = false;
    let detail = '';
    if (error && error.message) detail = `: ${error.message}`;
    else if (error && typeof error === 'string') detail = `: ${error}`;
    loadingEl.textContent = `${message}${detail}`;
}

function setPosition(text, percent) {
    positionEl.textContent = text || '\u00A0';
    progressBar.style.width = Math.max(0, Math.min(100, percent || 0)) + '%';
}

function makeMarker(percent) {
    return Math.max(0, Math.round(percent * 1000));
}

async function saveProgress(force) {
    const marker = currentMarker;
    if (marker === savedMarker) return;
    if (!force && Date.now() - lastSaveAt < 5000) return;
    savedMarker = marker;
    lastSaveAt = Date.now();
    try {
        await fetch(progressUrl, {
            method: 'PATCH',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ progress: marker }),
            keepalive: true,
        });
    } catch (e) {
        console.error('Failed to save reader progress:', e);
    }
}

window.addEventListener('pagehide', () => {
    if (currentMarker && currentMarker !== savedMarker) {
        if (typeof navigator.sendBeacon === 'function') {
            const blob = new Blob([JSON.stringify({ progress: currentMarker })], { type: 'application/json' });
            navigator.sendBeacon(progressUrl, blob);
        }
        saveProgress(true);
    }
});

function isPdf() {
    return String(cfg.mimeType || '').includes('pdf');
}

/* ---------------------------- PDF reader (continuous scroll) ---------------------------- */
const pdfState = {
    doc: null,
    pages: [],
    observer: null,
    rendering: new Map(),
    pdfjs: null,
};
let pdfRotation = 0;
let pdfResizeTimer = 0;

const rotateBtn = $('#reader-rotate');
if (rotateBtn) {
    if (!isPdf()) {
        rotateBtn.hidden = true;
    } else {
        rotateBtn.addEventListener('click', () => {
            pdfRotation = (pdfRotation + 90) % 360;
            relayoutPdf();
        });
    }
}

async function loadPdf() {
    scrollEl.hidden = false;
    const footerEl = document.querySelector('footer');
    if (footerEl) footerEl.hidden = false;

    const pdfjs = await import('/assets/vendor/pdfjs/pdf.min.mjs');
    pdfjs.GlobalWorkerOptions.workerSrc = '/assets/vendor/pdfjs/pdf.worker.min.mjs';
    pdfState.pdfjs = pdfjs;
    const doc = await pdfjs.getDocument({
        url: fileUrl,
        wasmUrl: '/assets/vendor/pdfjs/wasm/',
    }).promise;
    pdfState.doc = doc;

    const availWidth = () => Math.max(200, scrollEl.clientWidth - (window.innerWidth < 640 ? 16 : 48));
    const outScale = deviceRatio;

    pdfState.pages = [];
    let y = 24;
    for (let i = 0; i < doc.numPages; i++) {
        const page = await doc.getPage(i + 1);
        const rot = (page.rotate + pdfRotation) % 360;
        const viewBase = page.getViewport({ scale: 1, rotation: rot });
        const scale = availWidth() / viewBase.width;
        const vp = page.getViewport({ scale, rotation: rot });
        pdfState.pages.push({ num: i + 1, page, vp, y, height: vp.height });
        y += vp.height + 28;
    }

    buildPdfDom();
    attachPdfObserver();

    prevBtn.hidden = doc.numPages <= 1;
    nextBtn.hidden = doc.numPages <= 1;

    const initial = savedProgress >= 1 && savedProgress <= doc.numPages ? savedProgress : 1;
    scrollEl.scrollTop = pdfState.pages[initial - 1].y + 4;
    await renderPdfPage(initial);
    updatePdfCurrent();
    loadingEl.hidden = true;
}

function buildPdfDom() {
    const wrap = document.createElement('div');
    wrap.id = 'pdf-pages';
    scrollEl.replaceChildren(wrap);
    const frag = document.createDocumentFragment();
    for (const p of pdfState.pages) {
        const pageEl = document.createElement('div');
        pageEl.className = 'pdf-page';
        pageEl.style.height = p.height + 'px';
        const cvs = document.createElement('canvas');
        const textLayerDiv = document.createElement('div');
        textLayerDiv.className = 'textLayer';
        const lbl = document.createElement('span');
        lbl.className = 'pdf-page-num';
        lbl.textContent = String(p.num);
        pageEl.append(cvs, textLayerDiv, lbl);
        p.el = pageEl;
        p.canvas = cvs;
        p.textLayerDiv = textLayerDiv;
        p.rendered = false;
        frag.appendChild(pageEl);
    }
    wrap.appendChild(frag);
}

function attachPdfObserver() {
    if (pdfState.observer) pdfState.observer.disconnect();
    pdfState.observer = null;
    pdfState.observer = new IntersectionObserver((entries) => {
        for (const entry of entries) {
            if (!entry.isIntersecting) continue;
            const idx = Number(entry.target.dataset.idx);
            const p = pdfState.pages[idx];
            if (p && !p.rendered) renderPdfPage(p.num);
        }
    }, { root: scrollEl, rootMargin: '600px 0px' });
    pdfState.pages.forEach((p, idx) => {
        p.el.dataset.idx = String(idx);
        pdfState.observer.observe(p.el);
    });
}

function renderPdfPage(num) {
    const p = pdfState.pages[num - 1];
    if (!p || p.rendered) return Promise.resolve();
    if (p.rendering) return p.rendering;
    p.rendering = (async () => {
        const outScale = deviceRatio;
        const maxDim = 4096;
        let cW = Math.floor(p.vp.width * outScale);
        let cH = Math.floor(p.vp.height * outScale);
        let actualScale = outScale;
        if (cW > maxDim || cH > maxDim) {
            const ratio = Math.min(maxDim / cW, maxDim / cH);
            actualScale *= ratio;
            cW = Math.floor(p.vp.width * actualScale);
            cH = Math.floor(p.vp.height * actualScale);
        }
        p.canvas.width = cW;
        p.canvas.height = cH;
        p.canvas.style.width = Math.floor(p.vp.width) + 'px';
        p.canvas.style.height = Math.floor(p.vp.height) + 'px';
        const ctx = p.canvas.getContext('2d');
        ctx.setTransform(actualScale, 0, 0, actualScale, 0, 0);
        await p.page.render({ canvasContext: ctx, viewport: p.vp }).promise;
        p.rendered = true;
        p.rendering = null;

        try {
            const textContent = await p.page.getTextContent();
            p.textLayerDiv.replaceChildren();
            if (pdfState.pdfjs && pdfState.pdfjs.renderTextLayer) {
                await pdfState.pdfjs.renderTextLayer({
                    textContentSource: textContent,
                    container: p.textLayerDiv,
                    viewport: p.vp,
                }).promise;
            }
        } catch (err) {
            console.error('Text layer render error:', err);
        }
    })().catch(e => {
        p.rendering = null;
        console.error('PDF page render error:', e);
    });
    return p.rendering;
}

function pdfPageAtScroll() {
    const mid = scrollEl.scrollTop + scrollEl.clientHeight / 2;
    let lo = 0, hi = pdfState.pages.length - 1, best = 0;
    while (lo <= hi) {
        const m = (lo + hi) >> 1;
        if (pdfState.pages[m].y <= mid) {
            best = m;
            lo = m + 1;
        } else {
            hi = m - 1;
        }
    }
    return best;
}

let pdfScrollRaf = 0;
function updatePdfCurrent() {
    if (!pdfState.pages.length) return;
    const p = pdfState.pages[pdfPageAtScroll()];
    if (!p) return;
    setPosition(`Page ${p.num} of ${pdfState.doc.numPages}`, (p.num / pdfState.doc.numPages) * 100);
    currentMarker = p.num;
    saveProgress();
}

scrollEl.addEventListener('scroll', () => {
    if (!isPdf()) return;
    if (!pdfState.pages.length) return;
    if (pdfScrollRaf) return;
    pdfScrollRaf = requestAnimationFrame(() => {
        pdfScrollRaf = 0;
        updatePdfCurrent();
    });
});

function pdfGoto(num, behavior) {
    const idx = Math.max(0, Math.min(pdfState.doc.numPages - 1, num - 1));
    const top = pdfState.pages[idx].y + 4;
    if (behavior === 'instant') {
        scrollEl.scrollTop = top;
        updatePdfCurrent();
    } else {
        scrollEl.scrollTo({ top, behavior: 'auto' });
    }
}

function relayoutPdf() {
    const doc = pdfState.doc;
    pdfState.pages = [];
    let y = 24;
    Promise.all(Array.from({ length: doc.numPages }, (_, i) => doc.getPage(i + 1))).then(async (pageObjs) => {
        const width = availWidth();
        for (let i = 0; i < doc.numPages; i++) {
            const rot = (pageObjs[i].rotate + pdfRotation) % 360;
            const viewBase = pageObjs[i].getViewport({ scale: 1, rotation: rot });
            const scale = width / viewBase.width;
            const vp = pageObjs[i].getViewport({ scale, rotation: rot });
            pdfState.pages.push({ num: i + 1, page: pageObjs[i], vp, y, height: vp.height });
            y += vp.height + 28;
        }
        buildPdfDom();
        attachPdfObserver();
        pdfGoto(currentMarker, 'instant');
        await renderPdfPage(currentMarker);
    });
}

function navPrev() {
    if (isPdf()) {
        pdfGoto(currentMarker - 1);
    } else if (epubRendition) {
        epubRendition.prev().catch(err => console.error('EPUB prev error:', err));
    }
}

function navNext() {
    if (isPdf()) {
        pdfGoto(currentMarker + 1);
    } else if (epubRendition) {
        epubRendition.next().catch(err => console.error('EPUB next error:', err));
    }
}

prevBtn.addEventListener('click', navPrev);
nextBtn.addEventListener('click', navNext);
tapLeft.addEventListener('click', navPrev);
tapRight.addEventListener('click', navNext);

/* ---------------------------- EPUB reader (temporarily disabled) ---------------------------- */
let epubRendition = null;

async function loadEpub() {
    throw new Error('EPUB reader is temporarily disabled for maintenance.');
}

/* ---------------------------- shared controls ---------------------------- */
document.addEventListener('keydown', (e) => {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
    if (isPdf()) {
        if (e.code === 'Space' || e.key === 'ArrowRight' || e.key === 'PageDown') {
            e.preventDefault();
            nextBtn.click();
        } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
            e.preventDefault();
            prevBtn.click();
        }
    } else if (epubRendition) {
        if (e.code === 'Space' || e.key === 'ArrowRight' || e.key === 'PageDown') {
            e.preventDefault();
            epubRendition.next();
        } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
            e.preventDefault();
            epubRendition.prev();
        }
    }
});

window.addEventListener('resize', () => {
    if (isPdf()) {
        clearTimeout(pdfResizeTimer);
        pdfResizeTimer = setTimeout(relayoutPdf, 250);
    } else if (epubRendition) {
        epubRendition.resize();
    }
});

window.addEventListener('load', () => {
    if (isPdf()) {
        loadPdf().catch((e) => fail('Could not open this PDF', e));
    } else {
        loadEpub().catch((e) => fail('Could not open this EPUB', e));
    }
});