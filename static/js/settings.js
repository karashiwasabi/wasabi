let view, userIDInput, passwordInput, saveBtn;
let wholesalerCodeInput, wholesalerNameInput, addWholesalerBtn, wholesalersTableBody;

async function loadSettings() {
    try {
        const res = await fetch('/api/settings/get');
        if (!res.ok) throw new Error('設定の読み込みに失敗しました。');
        const settings = await res.json();
        userIDInput.value = settings.emednetUserId || '';
        passwordInput.value = settings.emednetPassword || '';
    } catch (err) {
        console.error(err); // ★ エラー詳細をコンソールに出力
        window.showNotification(err.message, 'error');
    }
}

async function saveSettings() {
    const settings = {
        emednetUserId: userIDInput.value,
        emednetPassword: passwordInput.value,
    };
    window.showLoading();
    try {
        const res = await fetch('/api/settings/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings),
        });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '設定の保存に失敗しました。');
        window.showNotification(resData.message, 'success');
    } catch (err) {
        console.error(err); // ★ エラー詳細をコンソールに出力
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

// 卸業者リストを描画する関数
function renderWholesalers(wholesalers) {
    if (!wholesalers) {
        wholesalersTableBody.innerHTML = '<tr><td colspan="3">登録されている卸業者がありません。</td></tr>';
        return;
    }
    wholesalersTableBody.innerHTML = wholesalers.map(w => `
        <tr data-code="${w.code}">
            <td>${w.code}</td>
            <td class="left">${w.name}</td>
            <td class="center"><button class="delete-wholesaler-btn btn">削除</button></td>
        </tr>
    `).join('');
}

// 卸業者リストをサーバーから読み込む関数
async function loadWholesalers() {
    try {
        const res = await fetch('/api/settings/wholesalers');
        if (!res.ok) throw new Error('卸業者リストの読み込みに失敗しました。');
        const data = await res.json();
        renderWholesalers(data);
    } catch (err) {
        console.error(err); // ★ エラー詳細をコンソールに出力
        window.showNotification(err.message, 'error');
    }
}

// 卸業者を追加する関数
async function addWholesaler() {
    const code = wholesalerCodeInput.value.trim();
    const name = wholesalerNameInput.value.trim();

    if (!code || !name) {
        window.showNotification('卸コードと卸業者名の両方を入力してください。', 'error');
        return;
    }

    window.showLoading();
    try {
        const res = await fetch('/api/settings/wholesalers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code, name }),
        });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '卸業者の追加に失敗しました。');
        
        window.showNotification(resData.message, 'success');
        wholesalerCodeInput.value = '';
        wholesalerNameInput.value = '';
        loadWholesalers(); // リストを再読み込み
    } catch (err) {
        console.error(err); // ★ エラー詳細をコンソールに出力
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

export function initSettings() {
    view = document.getElementById('settings-view');
    if (!view) return;

    userIDInput = document.getElementById('emednetUserID');
    passwordInput = document.getElementById('emednetPassword');
    saveBtn = document.getElementById('saveSettingsBtn');
    
    wholesalerCodeInput = document.getElementById('wholesalerCode');
    wholesalerNameInput = document.getElementById('wholesalerName');
    addWholesalerBtn = document.getElementById('addWholesalerBtn');
    wholesalersTableBody = document.querySelector('#wholesalers-table tbody');
    const clearTransactionsBtn = document.getElementById('clearAllTransactionsBtn');
    
    // ▼▼▼【ここからが、本来の追加箇所です】▼▼▼
    const clearMastersBtn = document.getElementById('clearAllMastersBtn');
    
    clearMastersBtn.addEventListener('click', async () => {
        if (!confirm('本当に全ての製品マスターを削除しますか？\n\nJCSHMSマスターも削除されるため、再読み込みするまで品目情報が失われます。この操作は元に戻せません。')) {
            return;
        }

        window.showLoading();
        try {
            const res = await fetch('/api/masters/clear_all', {
                method: 'POST',
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '製品マスターの削除に失敗しました。');
            
            window.showNotification(resData.message, 'success');
        } catch (err) {
            console.error(err);
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });
    // ▲▲▲【ここまでが、本来の追加箇所です】▲▲▲

    saveBtn.addEventListener('click', saveSettings);
    addWholesalerBtn.addEventListener('click', addWholesaler);
    


    // ▼▼▼【ここから追加】▼▼▼
    clearTransactionsBtn.addEventListener('click', async () => {
        if (!confirm('本当にすべての取引履歴（入出庫、納品、処方、棚卸など）を削除しますか？\n\nこの操作は元に戻せません。')) {
            return;
        }

        window.showLoading();
        try {
            const res = await fetch('/api/transactions/clear_all', {
                method: 'POST',
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '取引データの削除に失敗しました。');
            
            window.showNotification(resData.message, 'success');
        } catch (err) {
            console.error(err);
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });
    // ▲▲▲【追加ここまで】▲▲▲

    wholesalersTableBody.addEventListener('click', async (e) => {
        if (e.target.classList.contains('delete-wholesaler-btn')) {
            const row = e.target.closest('tr');
            const code = row.dataset.code;
            if (!confirm(`卸コード [${code}] を削除します。よろしいですか？`)) {
                return;
            }

            window.showLoading();
            try {
                const res = await fetch(`/api/settings/wholesalers/${code}`, {
                    method: 'DELETE',
                });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
                window.showNotification(resData.message, 'success');
                loadWholesalers(); // リストを再読み込み
            } catch (err) {
                console.error(err); // ★ エラー詳細をコンソールに出力
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        }
    });
}

// 画面が表示されるたびに最新の設定を読み込むための関数
export function onViewShow() {
    loadSettings();
    loadWholesalers();
}