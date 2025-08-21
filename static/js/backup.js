/**
 * ファイルアップロードを処理し、サーバーに送信する共通関数
 * @param {Event} event - ファイル入力のchangeイベント
 * @param {string} url - アップロード先のAPIエンドポイント
 */
async function handleFileUpload(event, url) {
    const fileInput = event.target;
    const file = fileInput.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('file', file);
    
    window.showLoading();
    try {
        const res = await fetch(url, {
            method: 'POST',
            body: formData,
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || 'インポートに失敗しました。');
        }
        window.showNotification(resData.message, 'success');
    } catch (err) {
        console.error(err);
        window.showNotification(`エラー: ${err.message}`, 'error');
    } finally {
        window.hideLoading();
        fileInput.value = ''; // 次回同じファイルを選択できるようにリセット
    }
}

/**
 * 全てのエクスポート・インポートボタンの機能を初期化する
 */
export function initBackupButtons() {
    // DOM要素を取得
    const exportClientsBtn = document.getElementById('exportClientsBtn');
    const importClientsBtn = document.getElementById('importClientsBtn');
    const importClientsInput = document.getElementById('importClientsInput');
    const exportProductsBtn = document.getElementById('exportProductsBtn');
    const importProductsBtn = document.getElementById('importProductsBtn');
    const importProductsInput = document.getElementById('importProductsInput');

    // 得意先エクスポート
    if (exportClientsBtn) {
        exportClientsBtn.addEventListener('click', () => {
            window.location.href = '/api/clients/export';
        });
    }

    // 得意先インポート
    if (importClientsBtn && importClientsInput) {
        importClientsBtn.addEventListener('click', () => {
            importClientsInput.click();
        });
        importClientsInput.addEventListener('change', (event) => {
            handleFileUpload(event, '/api/clients/import');
        });
    }

    // 製品エクスポート
    if (exportProductsBtn) {
        exportProductsBtn.addEventListener('click', () => {
            window.location.href = '/api/products/export';
        });
    }

    // 製品インポート
    if (importProductsBtn && importProductsInput) {
        importProductsBtn.addEventListener('click', () => {
            importProductsInput.click();
        });
        importProductsInput.addEventListener('change', (event) => {
            handleFileUpload(event, '/api/products/import');
        });
    }
}