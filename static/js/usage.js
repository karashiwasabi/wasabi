// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\usage.js
// このファイル全体を以下の内容で置き換えてください。

import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

// 共通のエラーハンドリング関数
async function handleResponseError(res) {
    const errorText = await res.text();
    try {
        // エラーメッセージがJSON形式の場合
        const errorJson = JSON.parse(errorText);
        return new Error(errorJson.message || '不明なエラーが発生しました。');
    } catch (e) {
        // エラーメッセージがテキスト形式の場合
        return new Error(errorText || `サーバーエラーが発生しました (HTTP ${res.status})`);
    }
}

// 自動インポート用の関数
async function handleAutomaticUsageImport() {
    const uploadContainer = document.getElementById('upload-output-container');
    uploadContainer.innerHTML = `<p>設定されたパスからUSAGEファイルを読み込んでいます...</p>`;
    window.showLoading();

    try {
        const res = await fetch('/api/usage/upload', { method: 'POST' });
        if (!res.ok) {
            throw await handleResponseError(res);
        }
        const data = await res.json();

        const tableShell = createUploadTableHTML('upload-output-table');
        const tableBodyContent = renderUploadTableRows(data.records);
        uploadContainer.innerHTML = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);
        window.showNotification('USAGEファイルのインポートが完了しました。', 'success');
    } catch (err) {
        const errorRow = `<tr><td colspan="14" style="color:red; text-align:center;">エラー: ${err.message}</td></tr>`;
        uploadContainer.innerHTML = createUploadTableHTML('upload-output-table').replace('<tbody></tbody>', `<tbody>${errorRow}</tbody>`);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

// 手動ファイルアップロード用の関数
async function handleManualUsageUpload(event) {
    const files = event.target.files;
    if (!files.length) return;

    const uploadContainer = document.getElementById('upload-output-container');
    uploadContainer.innerHTML = `<p>USAGEファイルをアップロードしています...</p>`;
    window.showLoading();
    
    try {
        const formData = new FormData();
        formData.append('file', files[0]);

        const res = await fetch('/api/usage/upload', {
            method: 'POST',
            body: formData,
        });
        if (!res.ok) {
            throw await handleResponseError(res);
        }
        const data = await res.json();

        const tableShell = createUploadTableHTML('upload-output-table');
        const tableBodyContent = renderUploadTableRows(data.records);
        uploadContainer.innerHTML = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);
        window.showNotification('USAGEファイルのインポートが完了しました。', 'success');
    } catch (err) {
        const errorRow = `<tr><td colspan="14" style="color:red; text-align:center;">エラー: ${err.message}</td></tr>`;
        uploadContainer.innerHTML = createUploadTableHTML('upload-output-table').replace('<tbody></tbody>', `<tbody>${errorRow}</tbody>`);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        event.target.value = '';
    }
}

export function initUsageUpload() {
    document.addEventListener('importUsageFromPath', handleAutomaticUsageImport);

    const usageInput = document.getElementById('usageFileInput');
    if (usageInput) {
        usageInput.addEventListener('change', handleManualUsageUpload);
    }
}