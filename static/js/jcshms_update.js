export function initJcshmsUpdate() {
    const reloadBtn = document.getElementById('reloadJcshmsBtn');
    if (!reloadBtn) return;

    reloadBtn.addEventListener('click', async () => {
        if (!confirm('SOUフォルダ内のJCSHMS.CSVとJANCODE.CSVをデータベースに再読み込みします。\nこれには数分かかることがあり、完了するまでアプリケーションは応答しなくなります。\nよろしいですか？')) {
            return;
        }
        window.showLoading();
        // ▼▼▼ [修正点] 結果表示用のコンテナを取得し、処理中メッセージを表示 ▼▼▼
        const resultContainer = document.getElementById('upload-output-container');
        const uploadView = document.getElementById('upload-view');
        const activeView = document.querySelector('main > div:not(.hidden)');
        
        if (uploadView && activeView) {
            activeView.classList.add('hidden');
            uploadView.classList.remove('hidden');
        }
        resultContainer.innerHTML = `<h3>JCSHMSマスター更新処理中...</h3><p>ブラウザを閉じないでください。</p>`;
        // ▲▲▲ 修正ここまで ▲▲▲

        try {
            const res = await fetch('/api/masters/reload_jcshms', { method: 'POST' });
            const resData = await res.json();
            if (!res.ok) {
                throw new Error(resData.message || '再読み込みに失敗しました。');
            }
            
            // ▼▼▼ [修正点] 結果表示ロジックを全面的に書き換え ▼▼▼
            let resultHTML = `<h3>${resData.message}</h3>`;

            if (resData.updatedProducts && resData.updatedProducts.length > 0) {
                resultHTML += `<p style="margin-top: 10px;">以下の${resData.updatedProducts.length}件が更新されました。</p>
                               <table class="data-table"><thead><tr><th>JAN</th><th>製品名</th></tr></thead><tbody>`;
                resData.updatedProducts.forEach(p => {
                    resultHTML += `<tr><td>${p.productCode}</td><td class="left">${p.productName}</td></tr>`;
                });
                resultHTML += `</tbody></table>`;
            }

            if (resData.orphanedProducts && resData.orphanedProducts.length > 0) {
                resultHTML += `<p style="margin-top: 10px;">以下の${resData.orphanedProducts.length}件がJCSHMSから削除されたため、手動管理に移行しました。</p>
                               <table class="data-table"><thead><tr><th>JAN</th><th>製品名</th></tr></thead><tbody>`;
                resData.orphanedProducts.forEach(p => {
                    resultHTML += `<tr><td>${p.productCode}</td><td class="left">${p.productName}</td></tr>`;
                });
                resultHTML += `</tbody></table>`;
            }

            resultContainer.innerHTML = resultHTML;
            window.showNotification(resData.message, 'success');
            // ▲▲▲ 修正ここまで ▲▲▲
        } catch (err) {
            console.error(err);
            resultContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`; // エラーもコンテナに表示
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
        }
    });
}