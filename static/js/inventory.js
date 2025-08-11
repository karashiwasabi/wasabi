import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

// DOM要素をキャッシュ
const fileInput = document.getElementById('inventoryFileInput');
const outputContainer = document.getElementById('inventory-output-container');

async function handleInventoryUpload(event) {
    const files = event.target.files;
    if (!files.length) return;

    // テーブルの準備と処理中メッセージの表示
    outputContainer.innerHTML = createUploadTableHTML('inventory-output-table');
    const tbody = outputContainer.querySelector('tbody');
    tbody.innerHTML = `<tr><td colspan="14" class="center">Processing...</td></tr>`;
    
    window.showLoading();
    try {
        const formData = new FormData();
        for (const file of files) {
            formData.append('file', file);
        }

        const response = await fetch('/api/inventory/upload', {
            method: 'POST',
            body: formData,
        });

        const data = await response.json();
        if (!response.ok) {
            throw new Error(data.message || 'Inventory file processing failed.');
        }

        // 共通のテーブル描画関数を使って結果を表示
        renderUploadTableRows('inventory-output-table', data.details);
        window.showNotification(data.message || 'Inventory file processed successfully.', 'success');
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="14" class="center" style="color:red;">Error: ${err.message}</td></tr>`;
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        event.target.value = ''; // ファイル入力をリセット
    }
}

export function initInventoryUpload() {
    if (!fileInput) return;
    fileInput.addEventListener('change', handleInventoryUpload);
}