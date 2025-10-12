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
    // ▼▼▼【ここから修正】▼▼▼
    const exportCustomersBtn = document.getElementById('exportCustomersBtn');
    const importCustomersBtn = document.getElementById('importCustomersBtn');
    const importCustomersInput = document.getElementById('importCustomersInput');
    // ▲▲▲【修正ここまで】▲▲▲
    const exportProductsBtn = document.getElementById('exportProductsBtn');
    const importProductsBtn = document.getElementById('importProductsBtn');
    const importProductsInput = document.getElementById('importProductsInput');
    const exportPricingBtn = document.getElementById('exportPricingBtn');

    // ▼▼▼【ここから修正】▼▼▼
    // 得意先・卸業者（顧客マスター）エクスポート
    if (exportCustomersBtn) {
        exportCustomersBtn.addEventListener('click', () => {
            window.location.href = '/api/customers/export';
        });
    }

    // 得意先・卸業者（顧客マスター）インポート
    if (importCustomersBtn && importCustomersInput) {
        importCustomersBtn.addEventListener('click', () => {
            importCustomersInput.click();
        });
        importCustomersInput.addEventListener('change', (event) => {
            handleFileUpload(event, '/api/customers/import');
        });
    }
    // ▲▲▲【修正ここまで】▲▲▲

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

    // 価格情報バックアップ
    if (exportPricingBtn) {
        exportPricingBtn.addEventListener('click', () => {
            window.location.href = '/api/pricing/backup_export';
        });
    }
}