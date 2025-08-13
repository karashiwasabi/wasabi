import { initInOut, resetInOutView } from './inout.js';
import { initDatUpload } from './dat.js';
import { initUsageUpload } from './usage.js';
import { initInventoryUpload } from './inventory.js';
import { initAggregation } from './aggregation.js';
import { initMasterEdit, resetMasterEditView } from './master_edit.js';
import { initReprocessButton } from './reprocess.js';
import { initBackupButtons } from './backup.js'; // ★インポート追加

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
    const datFileInput = document.getElementById('datFileInput');
    const usageFileInput = document.getElementById('usageFileInput');
    const inventoryFileInput = document.getElementById('inventoryFileInput');

    // --- Initialize all modules ---
    initInOut();
    initDatUpload();
    initUsageUpload();
    initInventoryUpload();
    initAggregation();
    initMasterEdit();
    initReprocessButton();
    initBackupButtons(); // ★初期化

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
        datFileInput.click();
    });
    usageBtn.addEventListener('click', () => {
        showView('upload-view');
        document.getElementById('upload-view-title').textContent = `USAGE File Upload`;
        usageFileInput.click();
    });
    inventoryBtn.addEventListener('click', () => {
        showView('inventory-view');
        inventoryFileInput.click();
    });
    aggregationBtn.addEventListener('click', () => { showView('aggregation-view'); });
    masterEditBtn.addEventListener('click', () => { showView('master-edit-view'); resetMasterEditView(); });

    // --- Initial State ---
    showView('in-out-view');
    resetInOutView();
});