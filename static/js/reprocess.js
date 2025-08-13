export function initReprocessButton() {
    const reprocessBtn = document.getElementById('reprocessBtn');
    if (!reprocessBtn) return;

    reprocessBtn.addEventListener('click', async () => {
        if (!confirm('仮登録状態の取引データを、最新のマスター情報で更新します。よろしいですか？')) {
            return;
        }

        window.showLoading();
        try {
            const res = await fetch('/api/transactions/reprocess', {
                method: 'POST',
            });
            const data = await res.json();
            if (!res.ok) {
                throw new Error(data.message || '処理に失敗しました。');
            }
            window.showNotification(data.message, 'success');
        } catch (err) {
            console.error(err);
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
        }
    });
}