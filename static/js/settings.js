// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\settings.js

let view, userIDInput, passwordInput, saveBtn, usageFolderPathInput, calculationPeriodDaysInput, edgePathInput;
let wholesalerCodeInput, wholesalerNameInput, addWholesalerBtn, wholesalersTableBody;
/**
 * サーバーから設定を読み込み、画面の入力欄に反映させます。
 */
async function loadSettings() {
    try {
        const res = await fetch('/api/settings/get');
        if (!res.ok) throw new Error('設定の読み込みに失敗しました。');
        const settings = await res.json();
        userIDInput.value = settings.emednetUserId || '';
        passwordInput.value = settings.emednetPassword || '';
        if (usageFolderPathInput) {
            usageFolderPathInput.value = settings.usageFolderPath || '';
        }
        if (calculationPeriodDaysInput) {
            calculationPeriodDaysInput.value = settings.calculationPeriodDays || 90;
        }
        if (edgePathInput) {
            edgePathInput.value = settings.edgePath || '';
        }
        // Edgeのパスが設定されていなければボタンを無効化する
        const edgeBtn = document.getElementById('edgeDownloadBtn');
        if(edgeBtn) {
            edgeBtn.disabled = !settings.edgePath;
        }

    } catch (err) {
         console.error(err);
        window.showNotification(err.message, 'error');
    }
}


/**
 * 現在の画面の入力内容をサーバーに送信して保存します。
 */
async function saveSettings() {
    window.showLoading();
    try {
        const currentSettingsRes = await fetch('/api/settings/get');
        if (!currentSettingsRes.ok) throw new Error('現在の設定の読み込みに失敗しました。');
        const currentSettings = await currentSettingsRes.json();

        const userId = userIDInput.value;
        const password = passwordInput.value;
        const usagePath = usageFolderPathInput.value;
        const periodDays = parseInt(calculationPeriodDaysInput.value, 10);
        const edgePath = edgePathInput.value.trim(); // Edgeパスの値を取得

        const newSettings = {
             ...currentSettings,
            emednetUserId: userId,
            emednetPassword: password,
            edeUserId: userId,
            edePassword: password,
            usageFolderPath: usagePath,
            calculationPeriodDays: periodDays,
            edgePath: edgePath, // ペイロードに追加
        };

        const res = await fetch('/api/settings/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newSettings),
        });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '設定の保存に失敗しました。');
        
        window.showNotification(resData.message, 'success');
        
        // 保存後にボタンの状態を更新
        const edgeBtn = document.getElementById('edgeDownloadBtn');
        if(edgeBtn) {
            edgeBtn.disabled = !newSettings.edgePath;
        }

    } catch (err) {
         console.error(err);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

/**
 * 卸業者リストを描画します。 (変更なし)
 * @param {Array} wholesalers - 卸業者の配列
 */
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

/**
 * サーバーから卸業者リストを読み込みます。(変更なし)
 */
async function loadWholesalers() {
    try {
        const res = await fetch('/api/settings/wholesalers');
        if (!res.ok) throw new Error('卸業者リストの読み込みに失敗しました。');
        const data = await res.json();
        renderWholesalers(data);
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    }
}

/**
 * 新しい卸業者を追加します。(変更なし)
 */
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
        loadWholesalers(); 
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}


/**
 * 設定画面の初期化処理
 */
export function initSettings() {
    view = document.getElementById('settings-view');
    if (!view) return;

    // 各種DOM要素の取得
    userIDInput = document.getElementById('emednetUserID');
    passwordInput = document.getElementById('emednetPassword');
    saveBtn = document.getElementById('saveSettingsBtn');
    usageFolderPathInput = document.getElementById('usageFolderPath');
    calculationPeriodDaysInput = document.getElementById('calculationPeriodDays');
    edgePathInput = document.getElementById('edgePath');
    wholesalerCodeInput = document.getElementById('wholesalerCode');
    wholesalerNameInput = document.getElementById('wholesalerName');
    addWholesalerBtn = document.getElementById('addWholesalerBtn');
    wholesalersTableBody = document.querySelector('#wholesalers-table tbody');
    const clearTransactionsBtn = document.getElementById('clearAllTransactionsBtn');
    const clearMastersBtn = document.getElementById('clearAllMastersBtn');
    
    // イベントリスナーの設定
    clearMastersBtn.addEventListener('click', async () => {
        if (!confirm('本当に全ての製品マスターを削除しますか？\n\nJCSHMSマスターも削除されるため、再読み込みするまで品目情報が失われます。この操作は元に戻せません。')) {
            return;
        }
        window.showLoading();
        try {
            const res = await fetch('/api/masters/clear_all', { method: 'POST' });
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

    saveBtn.addEventListener('click', saveSettings);
    addWholesalerBtn.addEventListener('click', addWholesaler);
    
    clearTransactionsBtn.addEventListener('click', async () => {
        if (!confirm('本当にすべての取引履歴（入出庫、納品、処方、棚卸など）を削除しますか？\n\nこの操作は元に戻せません。')) {
            return;
        }
        window.showLoading();
        try {
            const res = await fetch('/api/transactions/clear_all', { method: 'POST' });
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

    wholesalersTableBody.addEventListener('click', async (e) => {
        if (e.target.classList.contains('delete-wholesaler-btn')) {
            const row = e.target.closest('tr');
            const code = row.dataset.code;
            if (!confirm(`卸コード [${code}] を削除します。よろしいですか？`)) {
                return;
            }
            window.showLoading();
            try {
                const res = await fetch(`/api/settings/wholesalers/${code}`, { method: 'DELETE' });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
                window.showNotification(resData.message, 'success');
                loadWholesalers(); 
            } catch (err) {
                console.error(err); 
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        }
    });
}

/**
 * 設定画面が表示されたときに呼ばれる関数
 */
export function onViewShow() {
    loadSettings();
    loadWholesalers();
}