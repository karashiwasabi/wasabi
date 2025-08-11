import { initInOut, resetInOutView } from './inout.js';
import { initDatUpload } from './dat.js';
import { initUsageUpload } from './usage.js';
import { initInventoryUpload } from './inventory.js';
import { initAggregation } from './aggregation.js'; // 集計モジュールをインポート

// Global UI Elements
const loadingOverlay = document.getElementById('loading-overlay');
const notificationBox = document.getElementById('notification-box');

// Global helper functions
window.showLoading = () => loadingOverlay.classList.remove('hidden');
window.hideLoading = () => loadingOverlay.classList.add('hidden');
window.showNotification = (message, type = 'success') => {
    notificationBox.textContent = message;
    notificationBox.className = 'notification-box';
    notificationBox.classList.add(type, 'show');
    setTimeout(() => { notificationBox.classList.remove('show'); }, 3000);
};

document.addEventListener('DOMContentLoaded', () => {
    // --- DOM Elements ---
    const allViews = document.querySelectorAll('main > div[id$="-view"]');
    const inOutBtn = document.getElementById('inOutViewBtn');
    const datBtn = document.getElementById('datBtn');
    const usageBtn = document.getElementById('usageBtn');
    const inventoryBtn = document.getElementById('inventoryBtn');
    const aggregationBtn = document.getElementById('aggregationBtn'); // 集計ボタンを取得
    
    const datFileInput = document.getElementById('datFileInput');
    const usageFileInput = document.getElementById('usageFileInput');
    const inventoryFileInput = document.getElementById('inventoryFileInput');

    // --- Initialize all modules ---
    initInOut();
    initDatUpload();
    initUsageUpload();
    initInventoryUpload();
    initAggregation(); // 集計モジュールを初期化

    // --- View Switching Logic ---
    function showView(viewIdToShow) {
        allViews.forEach(view => {
            view.classList.toggle('hidden', view.id !== viewIdToShow);
        });
    }

    // --- Event Listeners ---
    inOutBtn.addEventListener('click', () => {
        showView('in-out-view');
        resetInOutView();
    });
    datBtn.addEventListener('click', () => {
        showView('upload-view');
        document.getElementById('upload-view-title').textContent = `DAT File Upload`;
        document.getElementById('upload-output-container').innerHTML = `<p>ファイルを選択してください...</p>`;
        datFileInput.click();
    });
    usageBtn.addEventListener('click', () => {
        showView('upload-view');
        document.getElementById('upload-view-title').textContent = `USAGE File Upload`;
        document.getElementById('upload-output-container').innerHTML = `<p>ファイルを選択してください...</p>`;
        usageFileInput.click();
    });
    inventoryBtn.addEventListener('click', () => {
        showView('inventory-view');
        document.getElementById('inventory-output-container').innerHTML = `<p>棚卸ファイルを選択してください...</p>`;
        inventoryFileInput.click();
    });
    aggregationBtn.addEventListener('click', () => {
        showView('aggregation-view');
    });

    // --- Initial State ---
    showView('in-out-view');
    resetInOutView();
});