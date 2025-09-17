import { getDetailsData, clearDetailsTable, populateDetailsTable } from './precomp_details_table.js';

let patientNumberInput, saveBtn, loadBtn, clearBtn, exportBtn, importBtn, importInput, exportAllBtn, importAllBtn, importAllInput;

export function resetHeader() {
    if (patientNumberInput) {
        patientNumberInput.value = '';
    }
}

export function initHeader() {
    patientNumberInput = document.getElementById('precomp-patient-number');
    saveBtn = document.getElementById('precomp-save-btn');
    loadBtn = document.getElementById('precomp-load-btn');
    clearBtn = document.getElementById('precomp-clear-btn');
    exportBtn = document.getElementById('precomp-export-btn');
    importBtn = document.getElementById('precomp-import-btn');
    importInput = document.getElementById('precomp-import-input');
    exportAllBtn = document.getElementById('precomp-export-all-btn');
    importAllBtn = document.getElementById('precomp-import-all-btn');
    importAllInput = document.getElementById('precomp-import-all-input');

    // ▼▼▼【ここから追加】▼▼▼
    const toggleStatusBtn = document.getElementById('precomp-toggle-status-btn');

    if (toggleStatusBtn) {
        toggleStatusBtn.addEventListener('click', async () => {
            const patientNumber = patientNumberInput.value.trim();
            if (!patientNumber) {
                window.showNotification('患者番号を入力してください。', 'error');
                return;
            }

            // ボタンの現在のテキストに応じてAPIを決定
            const isSuspending = toggleStatusBtn.textContent === '予製中断';
            const endpoint = isSuspending ? '/api/precomp/suspend' : '/api/precomp/resume';
            const actionText = isSuspending ? '中断' : '再開';

            if (!confirm(`患者番号: ${patientNumber} の予製を${actionText}します。よろしいですか？`)) {
                return;
            }

            window.showLoading();
            try {
                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ patientNumber }),
                });

                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || `${actionText}に失敗しました。`);

                window.showNotification(resData.message, 'success');
                // 状態変更後にビューをリフレッシュ
                loadBtn.click();
            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        });
    }
    // ▲▲▲【追加ここまで】▲▲▲

    if (!patientNumberInput || !saveBtn || !loadBtn || !clearBtn) return;
    
    loadBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('患者番号を入力してください。', 'error');
            return;
        }
        window.showLoading();
        try {
            const res = await fetch(`/api/precomp/load?patientNumber=${encodeURIComponent(patientNumber)}`);
            if (!res.ok) throw new Error('データの呼び出しに失敗しました。');
            
            const responseData = await res.json();
            
            populateDetailsTable(responseData.records);

            const toggleBtn = document.getElementById('precomp-toggle-status-btn');
            const detailsContainer = document.getElementById('precomp-details-container');

            if (responseData.status === 'inactive') {
                if(toggleBtn) {
                    toggleBtn.textContent = '予製再開';
                    toggleBtn.style.backgroundColor = '#198754';
                }
                if(detailsContainer) detailsContainer.classList.add('is-inactive');
                window.showNotification('この患者の予製は中断中です。', 'success');
            } else {
                 if(toggleBtn) {
                    toggleBtn.textContent = '予製中断';
                    toggleBtn.style.backgroundColor = '';
                 }
                 if(detailsContainer) detailsContainer.classList.remove('is-inactive');
            }
        } catch (err) {
            window.showNotification(err.message, 'error');
            clearDetailsTable();
        } finally {
            window.hideLoading();
        }
    });

    clearBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('削除する患者番号を入力してください。', 'error');
            return;
        }
        if (!confirm(`患者番号: ${patientNumber} の予製データを完全に削除します。この操作は元に戻せません。よろしいですか？`)) {
            return;
        }
    
        window.showLoading();
        try {
            const res = await fetch(`/api/precomp/clear?patientNumber=${encodeURIComponent(patientNumber)}`, { method: 'DELETE' });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
            window.showNotification(resData.message, 'success');
            resetHeader();
            clearDetailsTable();
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });
    
    saveBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('患者番号を入力してください。', 'error');
            return;
        }
        const records = getDetailsData();
        if (records.length === 0 && !confirm(`保存対象の品目がありません。患者番号: ${patientNumber} の予製データをすべて削除しますがよろしいですか？`)) {
            return;
        }
        if (records.length > 0 && !confirm(`患者番号: ${patientNumber} の予製データを保存します。よろしいですか？`)) {
            return;
        }
        const payload = { patientNumber, records };
  
        window.showLoading();
        try {
            const res = await fetch('/api/precomp/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
            window.showNotification(resData.message, 'success');
            resetHeader();
            clearDetailsTable();
        } catch (err) {
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
        }
    });
    
    exportBtn.addEventListener('click', () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('エクスポートする患者番号を入力してください。', 'error');
            return;
        }
        window.location.href = `/api/precomp/export?patientNumber=${encodeURIComponent(patientNumber)}`;
    });
    
    exportAllBtn.addEventListener('click', () => {
        if (confirm('全患者の予製データをCSVファイルとしてエクスポートします。よろしいですか？')) {
            window.location.href = '/api/precomp/export_all';
        }
    });

    importBtn.addEventListener('click', () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('インポート先の患者番号を入力してください。', 'error');
            return;
        }
        importInput.click();
    });
    
    importInput.addEventListener('change', async (e) => {
        const file = e.target.files[0];
        const patientNumber = patientNumberInput.value.trim();
        if (!file || !patientNumber) return;

        const formData = new FormData();
        formData.append('file', file);
        formData.append('patientNumber', patientNumber);

        window.showLoading();
        try {
            const res = await fetch('/api/precomp/import', {
                method: 'POST',
                body: formData,
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || 'インポートに失敗しました。');
            
            window.showNotification(resData.message, 'success');
            loadBtn.click(); 
        } catch (err) {
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
            e.target.value = '';
        }
    });

    importAllBtn.addEventListener('click', () => {
        if (!confirm('複数患者の予製データを一括でインポートします。\nCSVの1列目には患者番号が必要です。\n既存のデータは上書きされます。よろしいですか？')) {
            return;
        }
        importAllInput.click();
    });

    importAllInput.addEventListener('change', async (e) => {
        const file = e.target.files[0];
        if (!file) return;

        const formData = new FormData();
        formData.append('file', file);

        window.showLoading();
        try {
            const res = await fetch('/api/precomp/import_all', {
                method: 'POST',
                body: formData,
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '全件インポートに失敗しました。');
            
            window.showNotification(resData.message, 'success');
            resetHeader();
            clearDetailsTable();
        } catch (err) {
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
            e.target.value = '';
        }
    });
}