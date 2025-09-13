// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\usage.js

import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

async function handleUsageImport() {
    const uploadContainer = document.getElementById('upload-output-container');
    uploadContainer.innerHTML = `<p>設定されたパスからUSAGEファイルを読み込んでいます...</p>`;
    window.showLoading();

    try {
        const res = await fetch('/api/usage/upload', { method: 'POST' });
        const data = await res.json();
        if (!res.ok) {
            throw new Error(data.message || 'USAGEファイルの処理に失敗しました。');
        }

        const tableShell = createUploadTableHTML('upload-output-table');
        const tableBodyContent = renderUploadTableRows(data.records);
        const fullTableHtml = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);
        uploadContainer.innerHTML = fullTableHtml;

        window.showNotification('USAGEファイルのインポートが完了しました。', 'success');

    } catch (err) {
        const tableShell = createUploadTableHTML('upload-output-table');
        const errorRow = `<tr><td colspan="14" style="color:red; text-align:center;">エラー: ${err.message}</td></tr>`;
        uploadContainer.innerHTML = tableShell.replace('<tbody></tbody>', `<tbody>${errorRow}</tbody>`);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

export function initUsageUpload() {
    // app.jsからのカスタムイベントをリッスンする
    document.addEventListener('importUsageFromPath', handleUsageImport);
}