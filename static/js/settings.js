let view, userIDInput, passwordInput, saveBtn;

async function loadSettings() {
    try {
        const res = await fetch('/api/settings/get');
        if (!res.ok) throw new Error('設定の読み込みに失敗しました。');
        const settings = await res.json();
        userIDInput.value = settings.emednetUserId || '';
        passwordInput.value = settings.emednetPassword || '';
    } catch (err) {
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

    saveBtn.addEventListener('click', saveSettings);
}

// 画面が表示されるたびに最新の設定を読み込むための関数
export function onViewShow() {
    loadSettings();
}