import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

export function initUsageUpload() {
    const usageInput = document.getElementById('usageFileInput');
    if (!usageInput) return;

    usageInput.addEventListener('change', async (e) => {
        const files = e.target.files;
        if (!files.length) return;

        const uploadContainer = document.getElementById('upload-output-container');
        // ▼▼▼【ここからが修正箇所です】▼▼▼

        // 先に処理中メッセージだけ表示する
        uploadContainer.innerHTML = `<p>Processing...</p>`;
        window.showLoading();

        try {
            const formData = new FormData();
            for (const file of files) formData.append('file', file);
            
            const res = await fetch('/api/usage/upload', { method: 'POST', body: formData });
            const data = await res.json();
            if (!res.ok) throw new Error(data.message || 'USAGE file processing failed.');
            
            // --- 修正後の描画ロジック ---
            // 1. データを取得した後に、テーブルの枠と中身をそれぞれ文字列として生成
            const tableShell = createUploadTableHTML('upload-output-table');
            const tableBodyContent = renderUploadTableRows(data.records);
            
            // 2. 文字列を結合して完全なHTMLを作成
            const fullTableHtml = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);

            // 3. 完成したHTMLを一度だけDOMに書き込む
            uploadContainer.innerHTML = fullTableHtml;

            window.showNotification('USAGE file processed successfully.', 'success');

        } catch (err) {
            // エラー時も同様に、テーブルの枠を作ってからエラーメッセージを表示すると確実
            const tableShell = createUploadTableHTML('upload-output-table');
            const errorRow = `<tr><td colspan="14" style="color:red; text-align:center;">Error: ${err.message}</td></tr>`;
            uploadContainer.innerHTML = tableShell.replace('<tbody></tbody>', `<tbody>${errorRow}</tbody>`);

            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
            e.target.value = ''; // Reset file input
        }
        // ▲▲▲【修正ここまで】▲▲▲
    });
}