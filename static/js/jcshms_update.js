export function initJcshmsUpdate() {
    const reloadBtn = document.getElementById('reloadJcshmsBtn');
    if (!reloadBtn) return;

    reloadBtn.addEventListener('click', async () => {
        if (!confirm('SOUフォルダ内のJCSHMS.CSVとJANCODE.CSVをデータベースに再読み込みします。\nこれには数分かかることがあり、完了するまでアプリケーションは応答しなくなります。\nよろしいですか？')) {
            return;
        }
        window.showLoading();
        try {
            const res = await fetch('/api/masters/reload_jcshms', { method: 'POST' });
            const resData = await res.json();
            if (!res.ok) {
                throw new Error(resData.message || '再読み込みに失敗しました。');
            }
            window.showNotification(resData.message, 'success');
        } catch (err) {
            console.error(err);
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
        }
    });
}