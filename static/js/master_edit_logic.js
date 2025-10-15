// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/master_edit_logic.js

import { showModal } from './inout_modal.js';
import { hiraganaToKatakana } from './utils.js';
import { renderMasters, updateShelfScannedList, createMasterRowHTML, formatPackageSpecForRow } from './master_edit_ui.js';

let view, tableContainer, refreshBtn, addRowBtn, kanaNameInput, dosageFormInput, shelfNumberInput;
let allMasters = [];
let shelfModal, closeShelfModalBtn, bulkShelfNumberBtn;
let shelfBarcodeForm, shelfBarcodeInput, shelfScannedList, shelfScannedCount;
let shelfNumberInputForBulk, shelfRegisterBtn, shelfClearBtn;
let shelfScanPool = [];

function applyFiltersAndRender() {
    const kanaFilter = hiraganaToKatakana(kanaNameInput.value).toLowerCase();
    const dosageFilter = dosageFormInput.value;
    const shelfFilter = shelfNumberInput ? shelfNumberInput.value.trim().toLowerCase() : '';
    let filteredMasters = allMasters;

    if (kanaFilter) {
        filteredMasters = filteredMasters.filter(p => 
            (p.productCode && p.productCode.toLowerCase().includes(kanaFilter)) ||
            (p.productName && p.productName.toLowerCase().includes(kanaFilter)) || 
            (p.kanaName && p.kanaName.toLowerCase().includes(kanaFilter))
        );
    }
    
    if (dosageFilter) {
        filteredMasters = filteredMasters.filter(p => 
            p.usageClassification && p.usageClassification.trim() === dosageFilter
        );
    }

    if (shelfFilter) {
        filteredMasters = filteredMasters.filter(p =>
            p.shelfNumber && p.shelfNumber.toLowerCase().includes(shelfFilter)
        );
    }
    
    renderMasters(filteredMasters, tableContainer);
}

async function loadAndRenderMasters() {
    tableContainer.innerHTML = `<p>読み込み中...</p>`;
    window.showLoading();
    try {
        const res = await fetch('/api/masters/editable');
        if (!res.ok) throw new Error('マスターの読み込みに失敗しました。');
        allMasters = await res.json();
        applyFiltersAndRender();
    } catch (err) {
        tableContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

function populateFormWithJcshms(selectedProduct, tbody) {
    if (!tbody) return;
    const setVal = (name, value) => {
        const el = tbody.querySelector(`[name="${name}"]`);
        if (el) el.value = value !== undefined ? value : '';
    };
    const productCodeInput = tbody.querySelector('[name="productCode"]');
    if (productCodeInput && !productCodeInput.readOnly) {
        productCodeInput.value = selectedProduct.productCode || '';
    }
    setVal('yjCode', selectedProduct.yjCode); setVal('productName', selectedProduct.productName);
    setVal('kanaName', selectedProduct.kanaName); setVal('makerName', selectedProduct.makerName);
    setVal('usageClassification', selectedProduct.usageClassification); setVal('packageForm', selectedProduct.packageForm);
    setVal('nhiPrice', selectedProduct.nhiPrice); setVal('flagPoison', selectedProduct.flagPoison);
    setVal('flagDeleterious', selectedProduct.flagDeleterious); setVal('flagNarcotic', selectedProduct.flagNarcotic);
    setVal('flagPsychotropic', selectedProduct.flagPsychotropic); setVal('flagStimulant', selectedProduct.flagStimulant);
    setVal('flagStimulantRaw', selectedProduct.flagStimulantRaw); setVal('yjUnitName', selectedProduct.yjUnitName);
    setVal('yjPackUnitQty', selectedProduct.yjPackUnitQty);
    setVal('janPackInnerQty', selectedProduct.janPackInnerQty);
    setVal('janUnitCode', selectedProduct.janUnitCode); setVal('janPackUnitQty', selectedProduct.janPackUnitQty);
	setVal('specification', selectedProduct.specification);
	setVal('gs1Code', selectedProduct.gs1Code);
    formatPackageSpecForRow(tbody);
}

async function resolveProductNameForShelfScan(gs1) {
    try {
        const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1}`);
        if (res.ok) {
            const product = await res.json();
            const poolItem = shelfScanPool.find(item => item.gs1 === gs1);
            if (poolItem) {
                poolItem.name = product.productName;
                updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
            }
        } else if (res.status === 404) {
            if (confirm(`このGS1コード[${gs1}]はマスターに登録されていません。\n新規に仮マスターを作成しますか？`)) {
                const newMaster = await createProvisionalMasterForShelf(gs1);
                const poolItem = shelfScanPool.find(item => item.gs1 === gs1);
                 if (poolItem) {
                    poolItem.name = newMaster.productName;
                    updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
                }
            }
        }
    } catch (err) {
        console.warn(`品目名の解決に失敗: ${gs1}`, err);
        window.showNotification(`品目名の解決に失敗しました: ${gs1}`, 'error');
    }
}

async function createProvisionalMasterForShelf(gs1) {
    window.showLoading('新規マスターを作成中...');
    try {
        const productCode = gs1.length === 14 ? gs1.substring(1) : gs1;

        const res = await fetch('/api/master/create_provisional', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ gs1Code: gs1, productCode: productCode }),
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || 'マスターの作成に失敗しました。');
        }
        window.showNotification(`新規マスターを作成しました (YJ: ${resData.yjCode})`, 'success');

        const fetchRes = await fetch(`/api/product/by_gs1?gs1_code=${gs1}`);
        if (!fetchRes.ok) {
            throw new Error('作成したマスター情報の取得に失敗しました。');
        }
        return await fetchRes.json();
    } catch (err) {
        window.showNotification(err.message, 'error');
        throw err;
    } finally {
        window.hideLoading();
    }
}

async function handleShelfRegister() {
    const shelfNumber = shelfNumberInputForBulk.value.trim();
    if (!shelfNumber) {
        window.showNotification('棚番を入力してください。', 'error');
        return;
    }
    if (shelfScanPool.length === 0) {
        window.showNotification('登録する品目がスキャンされていません。', 'error');
        return;
    }

    const gs1Codes = shelfScanPool.map(item => item.gs1);

    if (!confirm(`スキャンした ${gs1Codes.length} 件の品目の棚番を「${shelfNumber}」として一括登録します。\nよろしいですか？`)) {
        return;
    }

    window.showLoading('棚番を一括更新中...');
    try {
        const payload = {
            shelfNumber: shelfNumber,
            gs1Codes: gs1Codes
        };
        const res = await fetch('/api/masters/bulk_update_shelf_numbers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!res.ok) {
            const errorText = await res.text();
            let errorMessage = '棚番の更新に失敗しました。';
            try {
                const errorJson = JSON.parse(errorText);
                errorMessage = errorJson.message || errorMessage;
            } catch (e) {
                errorMessage = errorText || errorMessage;
            }
            throw new Error(errorMessage);
        }

        const resData = await res.json();
        window.showNotification(resData.message, 'success');
        
        shelfScanPool = [];
        updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
        shelfNumberInputForBulk.value = '';
        shelfBarcodeInput.focus();

        loadAndRenderMasters();

    } catch (err) {
        window.showNotification(`エラー: ${err.message}`, 'error');
    } finally {
        window.hideLoading();
    }
}

export function initLogic() {
    view = document.getElementById('master-edit-view');
    if (!view) return;
    tableContainer = document.getElementById('master-edit-container');
    refreshBtn = document.getElementById('refreshMastersBtn');
    addRowBtn = document.getElementById('addMasterRowBtn');
    kanaNameInput = document.getElementById('master-edit-kanaName');
    dosageFormInput = document.getElementById('master-edit-dosageForm');
    shelfNumberInput = document.getElementById('master-edit-shelfNumber');

    shelfModal = document.getElementById('bulk-shelf-number-modal');
    closeShelfModalBtn = document.getElementById('close-shelf-modal-btn');
    bulkShelfNumberBtn = document.getElementById('bulkShelfNumberBtn');
    shelfBarcodeForm = document.getElementById('shelf-barcode-form');
    shelfBarcodeInput = document.getElementById('shelf-barcode-input');
    shelfScannedList = document.getElementById('shelf-scanned-list');
    shelfScannedCount = document.getElementById('shelf-scanned-count');
    shelfNumberInputForBulk = document.getElementById('shelf-number-input');
    shelfRegisterBtn = document.getElementById('shelf-register-btn');
    shelfClearBtn = document.getElementById('shelf-clear-btn');

    bulkShelfNumberBtn.addEventListener('click', () => {
        shelfScanPool = [];
        updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
        shelfNumberInputForBulk.value = '';
        shelfModal.classList.remove('hidden');
        document.body.classList.add('modal-open');
        setTimeout(() => shelfBarcodeInput.focus(), 100);
    });

    closeShelfModalBtn.addEventListener('click', () => {
        shelfModal.classList.add('hidden');
        document.body.classList.remove('modal-open');
    });

    shelfBarcodeForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const barcode = shelfBarcodeInput.value.trim();
        if (barcode) {
            if (!shelfScanPool.some(item => item.gs1 === barcode)) {
                shelfScanPool.push({ gs1: barcode, name: null });
                updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
                resolveProductNameForShelfScan(barcode);
            } else {
                window.showNotification('このバーコードは既にリストにあります。', 'error');
            }
        }
        shelfBarcodeInput.value = '';
    });

    shelfRegisterBtn.addEventListener('click', handleShelfRegister);

    shelfClearBtn.addEventListener('click', () => {
        if (confirm('スキャン済みリストをクリアしますか？')) {
            shelfScanPool = [];
            updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool);
            shelfBarcodeInput.focus();
        }
    });

    view.addEventListener('filterMasterEdit', async (e) => {
        const { productCode } = e.detail;
        if (productCode) {
            kanaNameInput.value = productCode;
            await loadAndRenderMasters(); 
            const targetRow = tableContainer.querySelector(`[data-record-id="${productCode}"]`);
            if (targetRow) {
                targetRow.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }
    });

    refreshBtn.addEventListener('click', loadAndRenderMasters);

    addRowBtn.addEventListener('click', async () => {
        window.showLoading();
        try {
            const res = await fetch('/api/sequence/next/MA2Y');
            const data = await res.json();
            if (!res.ok) throw new Error(data.message || 'YJコードの採番に失敗しました。');
            
            const newMasterData = { yjCode: data.nextCode };
            const newRowContent = createMasterRowHTML(newMasterData);
            const newTableHTML = `<table class="data-table master-edit-table">${newRowContent}</table>`;

            tableContainer.insertAdjacentHTML('beforeend', newTableHTML);
            const newTable = tableContainer.lastElementChild;
            
            if (newTable) {
                newTable.scrollIntoView({ behavior: 'smooth', block: 'end' });
                const firstInput = newTable.querySelector('input[name="productCode"]');
                if (firstInput) firstInput.focus();
            }
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });

    kanaNameInput.addEventListener('change', applyFiltersAndRender);
    dosageFormInput.addEventListener('change', applyFiltersAndRender);
    shelfNumberInput.addEventListener('input', applyFiltersAndRender);
    
    tableContainer.addEventListener('input', (e) => {
        const tbody = e.target.closest('tbody[data-record-id]');
        if (tbody) {
            formatPackageSpecForRow(tbody);
        }
    });

    tableContainer.addEventListener('click', async (e) => {
        const target = e.target;
        const tbody = target.closest('tbody[data-record-id]');
        if (!tbody) return;

        if (target.classList.contains('save-master-btn')) {
            const isNew = tbody.dataset.recordId.startsWith('new-');
            const data = {};
            tbody.querySelectorAll('input, select, textarea').forEach(el => {
                if(el.name){
                    const name = el.name;
                    const value = el.value;
                    
                    const numericSelects = [
                        'janUnitCode', 'flagPoison', 'flagDeleterious', 'flagNarcotic', 
                        'flagPsychotropic', 'flagStimulant', 'flagStimulantRaw', 'isOrderStopped'
                    ];

                    if (el.type === 'number' || (el.tagName === 'SELECT' && numericSelects.includes(name))) {
                        const numValue = parseFloat(value);
                        data[name] = !isNaN(numValue) ? numValue : 0;
                    } else {
                        data[name] = value;
                    }
                }
            });
      
            data.origin = data.origin || "PROVISIONAL";

            if (!data.productCode) {
                window.showNotification('製品コード(JAN)は必須です。', 'error');
                return;
            }
            window.showLoading();
            try {
                const res = await fetch('/api/master/update', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data),
                });

                if (!res.ok) {
                    let errorMsg = `保存に失敗しました (HTTP ${res.status})`;
                    try {
                        const errData = await res.json();
                        errorMsg = errData.message || errorMsg;
                    } catch (jsonError) {
                        const errText = await res.text();
                        errorMsg = errText || errorMsg;
                    }
                    throw new Error(errorMsg);
                }

                const resData = await res.json();
                window.showNotification(resData.message, 'success');

                const masterIndex = allMasters.findIndex(m => m.productCode === data.productCode);
                if (masterIndex > -1) {
                    allMasters[masterIndex] = { ...allMasters[masterIndex], ...data };
                } else {
                    allMasters.push(data);
                }

                if (isNew) {
                    const productCodeInput = tbody.querySelector('input[name="productCode"]');
                    productCodeInput.readOnly = true;
                    tbody.dataset.recordId = data.productCode;
                    const originInput = tbody.querySelector('input[name="origin"]');
                    originInput.value = data.origin;
                }
                
                tbody.querySelectorAll('tr').forEach(row => {
                    row.classList.remove('save-success');
                    void row.offsetWidth;
                    row.classList.add('save-success');
                });

            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        }
      
        if (target.classList.contains('quote-jcshms-btn')) {
            showModal(tbody, populateFormWithJcshms);
        }
    });

    loadAndRenderMasters();
}

export function resetMasterEditView() {
    if (tableContainer) {
        if(kanaNameInput) kanaNameInput.value = '';
        if(dosageFormInput) dosageFormInput.value = '';
        if(shelfNumberInput) shelfNumberInput.value = '';
        loadAndRenderMasters();
    }
}