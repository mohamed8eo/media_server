// Trove Offline Storage via IndexedDB
const TroveOffline = {
    dbName: 'TroveOfflineDB',
    storeName: 'mediaCache',
    db: null,

    async init() {
        if (this.db) return this.db;
        return new Promise((resolve, reject) => {
            const request = indexedDB.open(this.dbName, 1);
            request.onerror = () => reject(request.error);
            request.onsuccess = () => {
                this.db = request.result;
                resolve(this.db);
            };
            request.onupgradeneeded = (event) => {
                const db = event.target.result;
                if (!db.objectStoreNames.contains(this.storeName)) {
                    db.createObjectStore(this.storeName, { keyPath: 'id' });
                }
            };
        });
    },

    async saveMedia(id, type, dataBlob, metadata) {
        try {
            const db = await this.init();
            const tx = db.transaction(this.storeName, 'readwrite');
            const store = tx.objectStore(this.storeName);
            await store.put({
                id,
                type, // 'document' | 'audio' | 'metadata'
                blob: dataBlob,
                metadata: metadata || {},
                savedAt: new Date().toISOString()
            });
        } catch (e) {
            console.error('Failed to save offline media:', e);
        }
    },

    async getMedia(id) {
        try {
            const db = await this.init();
            return new Promise((resolve, reject) => {
                const tx = db.transaction(this.storeName, 'readonly');
                const store = tx.objectStore(this.storeName);
                const req = store.get(id);
                req.onsuccess = () => resolve(req.result);
                req.onerror = () => reject(req.error);
            });
        } catch (e) {
            console.error('Failed to get offline media:', e);
            return null;
        }
    },

    async listCached() {
        try {
            const db = await this.init();
            return new Promise((resolve, reject) => {
                const tx = db.transaction(this.storeName, 'readonly');
                const store = tx.objectStore(this.storeName);
                const req = store.getAll();
                req.onsuccess = () => resolve(req.result || []);
                req.onerror = () => reject(req.error);
            });
        } catch (e) {
            console.error('Failed to list cached media:', e);
            return [];
        }
    }
};

window.TroveOffline = TroveOffline;
