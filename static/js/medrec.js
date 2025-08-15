// C:\Dev\WASABI\static\js\medrec.js

export function initMedrec() {
    const downloadBtn = document.getElementById('medrecDownloadBtn');
    if (!downloadBtn) return;

    downloadBtn.addEventListener('click', async () => {
        if (!confirm('e-mednet.jpにログインし、納品データをダウンロードします。よろしいですか？')) {
            return;
        }

        window.showLoading();
        try {
            const res = await fetch('/api/medrec/download', {
                method: 'POST',
            });
            
            const resData = await res.json();
            if (!res.ok) {
                // res.json() failed or server returned an error message
                throw new Error(resData.message || `ダウンロードに失敗しました (HTTP ${res.status})`);
            }
            window.showNotification(resData.message, 'success');
        } catch (err) {
            // This catches network errors or errors thrown from the !res.ok check
            console.error('Download failed:', err);
            // Attempt to get a more specific error message if the response was text
            const errorMessage = err.message || 'サーバーとの通信に失敗しました。設定画面でID/パスワードが正しいか確認してください。';
            window.showNotification(errorMessage, 'error');
        } finally {
            window.hideLoading();
        }
    });
}