// C:\Dev\WASABI\static\js\app.js

import { initInOut, resetInOutView } from './inout.js';
import { initDatUpload } from './dat.js';
import { initUsageUpload } from './usage.js';
import { initInventoryUpload } from './inventory.js';
import { initAggregation } from './aggregation.js';
import { initMasterEdit, resetMasterEditView } from './master_edit.js';
import { initReprocessButton } from './reprocess.js';
import { initBackupButtons } from './backup.js';
import { initModal } from './inout_modal.js'; 
import { initDeadStock } from './deadstock.js'; 
import { initSettings, onViewShow as onSettingsViewShow } from './settings.js';
import { initMedrec } from './medrec.js'; // ▼▼▼ [修正点] 追加 ▼▼▼
import { initManualInventory } from './manual_inventory.js'; // ▼▼▼ [修正点] 追加 ▼▼▼
import { initPrecomp } from './precomp.js';


// (Global UI Elements and helper functions are unchanged)
window.showLoading = () => document.getElementById('loading-overlay').classList.remove('hidden');
window.hideLoading = () => document.getElementById('loading-overlay').classList.add('hidden');
window.showNotification = (message, type = 'success') => {
    const notificationBox = document.getElementById('notification-box');
    notificationBox.textContent = message;
    notificationBox.className = 'notification-box';
    notificationBox.classList.add(type, 'show');
    setTimeout(() => { notificationBox.classList.remove('show'); }, 3000);
};

document.addEventListener('DOMContentLoaded', () => {
    // (DOM Elements are unchanged)
    const allViews = document.querySelectorAll('main > div[id$="-view"]');
    const inOutBtn = document.getElementById('inOutViewBtn');
    const datBtn = document.getElementById('datBtn');
    const usageBtn = document.getElementById('usageBtn');
    const inventoryBtn = document.getElementById('inventoryBtn');
    const aggregationBtn = document.getElementById('aggregationBtn');
    const masterEditBtn = document.getElementById('masterEditViewBtn');
    const settingsBtn = document.getElementById('settingsBtn'); // ▼▼▼ [修正点] 追加 ▼▼▼
    const datFileInput = document.getElementById('datFileInput');
    const usageFileInput = document.getElementById('usageFileInput');
    const inventoryFileInput = document.getElementById('inventoryFileInput');
    const uploadOutputContainer = document.getElementById('upload-output-container');
    const inventoryOutputContainer = document.getElementById('inventory-output-container');
    const aggregationOutputContainer = document.getElementById('aggregation-output-container');
    const deadStockBtn = document.getElementById('deadStockBtn'); // ▼▼▼ [修正点] 追加 ▼▼▼
    const deadstockOutputContainer = document.getElementById('deadstock-output-container'); // ▼▼▼ [修正点] 追加 ▼▼▼
    const precompBtn = document.getElementById('precompBtn');

    // --- Initialize all modules ---
    initInOut();
    initDatUpload();
    initUsageUpload();
    initInventoryUpload();
    initAggregation();
    initMasterEdit();
    initReprocessButton();
    initBackupButtons();
    initModal(); // ▼▼▼ [修正点] モーダルをここで一度だけ初期化 ▼▼▼
    initDeadStock(); // ▼▼▼ [修正点] この一行が抜けていました ▼▼▼
    initSettings(); // ▼▼▼ [修正点] 追加 ▼▼▼
    initMedrec(); // ▼▼▼ [修正点] 追加 ▼▼▼
    initManualInventory(); // ▼▼▼ [修正点] 追加 ▼▼▼
    initPrecomp();

    // (View Switching Logic and Event Listeners are unchanged)
    function showView(viewIdToShow) {
        allViews.forEach(view => {
            view.classList.toggle('hidden', view.id !== viewIdToShow);
        });
    }

    inOutBtn.addEventListener('click', () => { showView('in-out-view'); resetInOutView(); });
    datBtn.addEventListener('click', () => {
        showView('upload-view');
        document.getElementById('upload-view-title').textContent = `DAT File Upload`;
        if (uploadOutputContainer) uploadOutputContainer.innerHTML = '';
        datFileInput.click();
    });
    usageBtn.addEventListener('click', () => {
        showView('upload-view');
        document.getElementById('upload-view-title').textContent = `USAGE File Upload`;
        if (uploadOutputContainer) uploadOutputContainer.innerHTML = '';
        usageFileInput.click();
    });
    inventoryBtn.addEventListener('click', () => {
        showView('inventory-view');
        if (inventoryOutputContainer) inventoryOutputContainer.innerHTML = '';
        inventoryFileInput.click();
    });
        // ▼▼▼ [修正点] 手入力棚卸ボタンのリスナーを追加 ▼▼▼
    manualInventoryBtn.addEventListener('click', () => {
        showView('manual-inventory-view');
        // Dispatch a custom event to trigger loading
        document.getElementById('manual-inventory-view').dispatchEvent(new Event('show'));
    });
    // ▲▲▲ 修正ここまで ▲▲▲
    aggregationBtn.addEventListener('click', () => {
        if (aggregationOutputContainer) aggregationOutputContainer.innerHTML = '';
        showView('aggregation-view');
    });
    deadStockBtn.addEventListener('click', () => {
        if(deadstockOutputContainer) deadstockOutputContainer.innerHTML = '';
        showView('deadstock-view');
    });
    masterEditBtn.addEventListener('click', () => { showView('master-edit-view'); resetMasterEditView(); });
        // ▼▼▼ [修正点] 設定ボタンのリスナーを追加 ▼▼▼
    settingsBtn.addEventListener('click', () => {
        showView('settings-view');
        onSettingsViewShow(); // 画面表示時に設定を読み込む
    });

    precompBtn.addEventListener('click', () => showView('precomp-view'));
    // ▲▲▲ 修正ここまで ▲▲▲
    // --- Initial State ---
    showView('in-out-view');
    resetInOutView();
});