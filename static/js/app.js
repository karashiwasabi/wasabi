// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\app.js

import { loadMasterData } from './master_data.js';
import { initInOut, resetInOutView } from './inout.js';
import { initDatUpload } from './dat.js';
import { initUsageUpload } from './usage.js';
import { initInventoryUpload } from './inventory.js';
import { initInventoryAdjustment } from './inventory_adjustment.js';
import { initAggregation } from './aggregation.js';
import { initMasterEdit, resetMasterEditView } from './master_edit.js';
import { initReprocessButton } from './reprocess.js';
import { initBackupButtons } from './backup.js';
import { initModal } from './inout_modal.js';
import { initDeadStock } from './deadstock.js';
import { initSettings, onViewShow as onSettingsViewShow } from './settings.js';
import { initMedrec } from './medrec.js';
import { initManualInventory } from './manual_inventory.js';
import { initPrecomp, resetPrecompView } from './precomp.js';
import { initOrders } from './orders.js';
import { initJcshmsUpdate } from './jcshms_update.js';
import { initBackorderView } from './backorder.js';
import { initValuationView } from './valuation.js';
import { initPricingView } from './pricing.js';
import { initReturnsView } from './returns.js';
// ▼▼▼ この行を追加 ▼▼▼
import { initEdge } from './edge.js';
// ▲▲▲ 追加ここまで ▲▲▲

window.showLoading = () => document.getElementById('loading-overlay').classList.remove('hidden');
window.hideLoading = () => document.getElementById('loading-overlay').classList.add('hidden');
window.showNotification = (message, type = 'success') => {
    const notificationBox = document.getElementById('notification-box');
    notificationBox.textContent = message;
    notificationBox.className = 'notification-box';
    notificationBox.classList.add(type, 'show');
    setTimeout(() => { notificationBox.classList.remove('show'); }, 3000);
};
document.addEventListener('DOMContentLoaded', async () => {
    
    await loadMasterData();
    const allViews = document.querySelectorAll('main > div[id$="-view"]');
    const inOutBtn = document.getElementById('inOutViewBtn');
    const datBtn = document.getElementById('datBtn');
    const usageBtn = document.getElementById('usageBtn');
    const inventoryBtn = document.getElementById('inventoryBtn');
    const inventoryAdjustmentBtn = document.getElementById('inventoryAdjustmentBtn');
    const manualInventoryBtn = document.getElementById('manualInventoryBtn');
    const aggregationBtn = document.getElementById('aggregationBtn');
    const masterEditBtn = document.getElementById('masterEditViewBtn');
    const settingsBtn = document.getElementById('settingsBtn');
    const datFileInput = document.getElementById('datFileInput');
    const usageFileInput = document.getElementById('usageFileInput');
 
   const inventoryFileInput = document.getElementById('inventoryFileInput');
    const uploadOutputContainer = document.getElementById('upload-output-container');
    const inventoryOutputContainer = document.getElementById('inventory-output-container');
const aggregationOutputContainer = document.getElementById('aggregation-output-container');
    const deadStockBtn = document.getElementById('deadStockBtn');
    const deadstockOutputContainer = document.getElementById('deadstock-output-container');
    const precompBtn = document.getElementById('precompBtn');
    const orderBtn = document.getElementById('orderBtn');
const backorderBtn = document.getElementById('backorderBtn');
    const valuationBtn = document.getElementById('valuationBtn');
    const pricingBtn = document.getElementById('pricingBtn');
    const returnsBtn = document.getElementById('returnsBtn');
// ▼▼▼ [修正点] 以下のコードをこのあたりに追記 ▼▼▼
    // アプリケーション内の全入力欄でブラウザのオートコンプリートを無効にする
    document.querySelectorAll('input[type="text"], input[type="password"], input[type="number"], input[type="date"]').forEach(input => {
        input.setAttribute('autocomplete', 'off');
    });
// ▲▲▲ 修正ここまで ▲▲▲

    initInOut();
    initDatUpload();
    initUsageUpload();
    initInventoryUpload();
    initInventoryAdjustment();
    initAggregation();
    initMasterEdit();
    initReprocessButton();
    initBackupButtons();
    initModal();
    initDeadStock();
    initSettings();
    initMedrec();
initManualInventory();
    initPrecomp();
    initOrders();
    initJcshmsUpdate();
    initBackorderView();
    initValuationView();
    initPricingView();
    initReturnsView();
    // ▼▼▼ この行を追加 ▼▼▼
    initEdge();
    // ▲▲▲ 追加ここまで ▲▲▲

    function showView(viewIdToShow) {
        const notificationBox = document.getElementById('notification-box');
if (notificationBox) {
            notificationBox.classList.remove('show');
}

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
manualInventoryBtn.addEventListener('click', () => {
        showView('manual-inventory-view');
        document.getElementById('manual-inventory-view').dispatchEvent(new Event('show'));
    });
inventoryAdjustmentBtn.addEventListener('click', () => {
        showView('inventory-adjustment-view');
    });
aggregationBtn.addEventListener('click', () => {
        if (aggregationOutputContainer) aggregationOutputContainer.innerHTML = '';
        showView('aggregation-view');
    });
deadStockBtn.addEventListener('click', () => {
        if(deadstockOutputContainer) deadstockOutputContainer.innerHTML = '';
        showView('deadstock-view');
    });
masterEditBtn.addEventListener('click', () => { showView('master-edit-view'); resetMasterEditView(); });
    settingsBtn.addEventListener('click', () => {
        showView('settings-view');
        onSettingsViewShow();
    });
precompBtn.addEventListener('click', () => {
    showView('precomp-view');
    resetPrecompView();
    });
    orderBtn.addEventListener('click', () => showView('order-view'));
backorderBtn.addEventListener('click', () => {
        showView('backorder-view');
        document.getElementById('backorder-view').dispatchEvent(new Event('show'));
    });
valuationBtn.addEventListener('click', () => {
        showView('valuation-view');
    });
pricingBtn.addEventListener('click', () => {
        showView('pricing-view');
        document.getElementById('pricing-view').dispatchEvent(new Event('show'));
    });
returnsBtn.addEventListener('click', () => showView('returns-view'));
    
    showView('in-out-view');
    resetInOutView();
});