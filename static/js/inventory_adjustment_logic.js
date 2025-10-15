// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/inventory_adjustment_logic.js
import { showModal } from './inout_modal.js';
import { hiraganaToKatakana, getLocalDateString } from './utils.js';
import { generateFullHtml, createFinalInputRow, setUnitMap } from './inventory_adjustment_ui.js';

let view, outputContainer;
let dosageFormFilter, kanaInitialFilter, selectProductBtn, deadStockOnlyFilter, barcodeInput, shelfNumberInput;
let currentYjCode = null;
let lastLoadedDataCache = null;

function parseGS1_128(code) {
    let rest = code;
    const data = {};

    if (rest.startsWith('01')) {
        if (rest.length < 16) return null;
        data.gs1Code = rest.substring(2, 16);
        rest = rest.substring(16);
    } else {
        return null;
    }

    if (rest.startsWith('17')) {
        if (rest.length < 8) return data; 
        data.expiryDate = rest.substring(2, 8);
        rest = rest.substring(8);
    }

    if (rest.startsWith('10')) {
        const groupSeparatorIndex = rest.indexOf('\x1D'); // GS (Group Separator)
        if (groupSeparatorIndex !== -1) {
            data.lotNumber = rest.substring(2, groupSeparatorIndex);
        } else {
            data.lotNumber = rest.substring(2);
        }
    }
   
    return data;
}

async function handleAdjustmentBarcodeScan(e) {
    e.preventDefault();
    const barcodeInput = document.getElementById('adjustment-barcode-input');
    const inputValue = barcodeInput.value.trim();
    if (!inputValue) return;

    const parsedData = parseGS1_128(inputValue);
    if (!parsedData || !parsedData.gs1Code) {
        window.showNotification('GS1-128形式のバーコードではありません。', 'error');
        barcodeInput.value = '';
        return;
    }

    window.showLoading('製品情報を検索中...');
    try {
        const res = await fetch(`/api/product/by_gs1?gs1_code=${parsedData.gs1Code}`);
        
        let productMaster;
        if (!res.ok) {
            if (res.status === 404) {
                 if (confirm(`このGS1コードはマスターに登録されていません。\n新規マスターを作成しますか？`)) {
                    productMaster = await createProvisionalMaster(parsedData.gs1Code);
                 } else {
                    throw new Error('このGS1コードはマスターに登録されていません。');
                 }
            } else {
                throw new Error('製品情報の検索に失敗しました。');
            }
        } else {
            productMaster = await res.json();
        }

        const productTbody = outputContainer.querySelector(`.final-input-tbody[data-product-code="${productMaster.productCode}"]`);
        if (!productTbody) {
            throw new Error(`画面内に製品「${productMaster.productName}」の入力欄が見つかりません。`);
        }

        let targetRow = null;
        const rows = productTbody.querySelectorAll('tr.inventory-row');
        for (let i = 0; i < rows.length; i += 2) {
            const expiryInput = rows[i].querySelector('.expiry-input');
            const lotInput = rows[i+1].querySelector('.lot-input');
            if (expiryInput.value.trim() === '' && lotInput.value.trim() === '') {
                targetRow = rows[i];
                break;
            }
        }

        if (!targetRow) {
            const addBtn = productTbody.querySelector('.add-deadstock-row-btn');
            if (addBtn) {
                addBtn.click();
                const newRows = productTbody.querySelectorAll('tr.inventory-row');
                targetRow = newRows[newRows.length - 2];
            }
        }

        if (targetRow) {
            const expiryInput = targetRow.querySelector('.expiry-input');
            const lotInput = targetRow.nextElementSibling.querySelector('.lot-input');
            if (parsedData.expiryDate) {
                expiryInput.value = parsedData.expiryDate;
            }
            if (parsedData.lotNumber) {
                lotInput.value = parsedData.lotNumber;
            }
            window.showNotification('ロット・期限を自動入力しました。', 'success');
        } else {
            throw new Error('ロット・期限の入力欄の追加に失敗しました。');
        }

    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        barcodeInput.value = '';
        barcodeInput.focus();
    }
}

async function handleBarcodeScan(e) {
    e.preventDefault();
    
    const barcodeInput = document.getElementById('ia-barcode-input');
    const inputValue = barcodeInput.value.trim();
    if (!inputValue) return;

    let gs1Code = '';
    if (inputValue.startsWith('01') && inputValue.length > 16) {
        const parsedData = parseGS1_128(inputValue);
        if (parsedData) {
            gs1Code = parsedData.gs1Code;
        }
    }
    
    if (!gs1Code) {
        gs1Code = inputValue;
    }

    if (!gs1Code) {
        window.showNotification('有効なGS1コードではありません。', 'error');
        return;
    }
   
    window.showLoading('製品情報を検索中...');
    try {
        const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1Code}`);
        if (!res.ok) {
            if (res.status === 404) {
                if (confirm(`このGS1コードはマスターに登録されていません。\n新規マスターを作成しますか？`)) {
                    await createProvisionalMaster(gs1Code);
                } else {
                    throw new Error('このGS1コードはマスターに登録されていません。');
                }
            } else {
                throw new Error('製品情報の検索に失敗しました。');
            }
        } else {
            const productMaster = await res.json();
            await loadAndRenderDetails(productMaster.yjCode);
            barcodeInput.value = '';
            barcodeInput.focus();
        }
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

async function createProvisionalMaster(gs1Code) {
    window.showLoading('新規マスターを作成中...');
    try {
        const productCode = gs1Code.length === 14 ? gs1Code.substring(1) : gs1Code;
        const res = await fetch('/api/master/create_provisional', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ gs1Code: gs1Code, productCode: productCode }),
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || 'マスターの作成に失敗しました。');
        }
        window.showNotification(`新規マスターを作成しました (YJ: ${resData.yjCode})`, 'success');
        await loadAndRenderDetails(resData.yjCode);
        const mainBarcode = document.getElementById('ia-barcode-input');
        if(mainBarcode) {
            mainBarcode.value = '';
            mainBarcode.focus();
        }
    } catch (err) {
        throw err;
    }
}

async function onSelectProductClick() {
    const dosageForm = dosageFormFilter.value;
    const kanaInitial = kanaInitialFilter.value;
    const isDeadStockOnly = deadStockOnlyFilter.checked;
    const shelfNumber = shelfNumberInput.value.trim();
    
    const params = new URLSearchParams({
        dosageForm: dosageForm,
        kanaInitial: kanaInitial,
        deadStockOnly: isDeadStockOnly,
        shelfNumber: shelfNumber,
    });
    
    const apiUrl = `/api/products/search_filtered?${params.toString()}`;
    const shouldSkipQueryLengthCheck = !!(dosageForm || kanaInitial || isDeadStockOnly || shelfNumber);
    
    window.showLoading();
    try {
        const res = await fetch(apiUrl);
        if (!res.ok) throw new Error('品目リストの取得に失敗しました。');
        const products = await res.json();
        window.hideLoading();
        showModal(view, (selectedProduct) => {
            loadAndRenderDetails(selectedProduct.yjCode);
        }, { 
            initialResults: products, 
            searchApi: apiUrl,
            skipQueryLengthCheck: shouldSkipQueryLengthCheck
        });
    } catch (err) {
        window.hideLoading();
        window.showNotification(err.message, 'error');
    }
}

async function loadAndRenderDetails(yjCode) {
    currentYjCode = yjCode;
    if (!yjCode) {
        window.showNotification('YJコードを指定してください。', 'error');
        return;
    }
    window.showLoading();
    outputContainer.innerHTML = '<p>データを読み込んでいます...</p>';
    try {
        const apiUrl = `/api/inventory/adjust/data?yjCode=${yjCode}`;
        const res = await fetch(apiUrl);
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'データ取得に失敗しました。');
        }
        
        lastLoadedDataCache = await res.json();
        const html = generateFullHtml(lastLoadedDataCache, lastLoadedDataCache);
        outputContainer.innerHTML = html;
        
        const dateInput = document.getElementById('inventory-date');
        if(dateInput) {
            const yesterday = new Date();
            yesterday.setDate(yesterday.getDate() - 1);
            const yyyy = yesterday.getFullYear();
            const mm = String(yesterday.getMonth() + 1).padStart(2, '0');
            const dd = String(yesterday.getDate()).padStart(2, '0');
            dateInput.value = `${yyyy}-${mm}-${dd}`;
        }
    } catch (err) {
        outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

function handleInputChanges(e) {
    const targetClassList = e.target.classList;
    if (targetClassList.contains('physical-stock-input') || targetClassList.contains('precomp-active-check')) {
        reverseCalculateStock();
    }
    if(targetClassList.contains('lot-quantity-input') || targetClassList.contains('final-inventory-input')){
        const productCode = e.target.dataset.productCode;
        updateFinalInventoryTotal(productCode);
    }
}

function handleClicks(e) {
    const target = e.target;
    if (target.classList.contains('add-deadstock-row-btn')) {
        const productCode = target.dataset.productCode;
        const master = findMaster(productCode);
        const tbody = document.querySelector(`.final-input-tbody[data-product-code="${productCode}"]`);
        if(master && tbody){
            const newRowHTML = createFinalInputRow(master, null, false);
            tbody.insertAdjacentHTML('beforeend', newRowHTML);
        }
    }
  
    if (target.classList.contains('delete-deadstock-row-btn')) {
        const topRow = target.closest('tr');
        const bottomRow = topRow.nextElementSibling;
        const productCode = bottomRow.querySelector('[data-product-code]')?.dataset.productCode;
        topRow.remove();
        bottomRow.remove();
        if(productCode) updateFinalInventoryTotal(productCode);
    }
    if (target.classList.contains('register-inventory-btn')) {
        saveInventoryData();
    }
}

function updatePrecompTotalDisplay() {
    let total = 0;
    document.querySelectorAll('.precomp-active-check:checked').forEach(cb => {
        total += parseFloat(cb.dataset.quantity) || 0;
    });
    const totalEl = document.getElementById('precomp-active-total');
    if (totalEl) {
        totalEl.textContent = `有効合計: ${total.toFixed(2)}`;
    }
}

function reverseCalculateStock() {
    const todayStr = getLocalDateString().replace(/-/g, '');
    const precompTotalsByProduct = {};
    const calculationErrorByProduct = {}; 

    document.querySelectorAll('.precomp-active-check:checked').forEach(cb => {
        const productCode = cb.dataset.productCode;
        const master = findMaster(productCode);
        if (!master) return;
        const yjQuantity = parseFloat(cb.dataset.quantity) || 0;

        if (master.janPackInnerQty > 0) {
            const janQuantity = yjQuantity / master.janPackInnerQty;
            precompTotalsByProduct[productCode] = (precompTotalsByProduct[productCode] || 0) + janQuantity;
        } else if (yjQuantity > 0) {
            calculationErrorByProduct[productCode] = '包装数量(内)未設定';
        }
    });
    updatePrecompTotalDisplay();

    const todayNetChangeByProduct = {};
    if (lastLoadedDataCache && lastLoadedDataCache.transactionLedger) {
        lastLoadedDataCache.transactionLedger.forEach(yjGroup => {
            if (yjGroup.packageLedgers) {
                yjGroup.packageLedgers.forEach(pkg => {
                    if (pkg.transactions) {
                        pkg.transactions.forEach(tx => {
                            if (tx.transactionDate === todayStr && tx.flag !== 0) {
                                let janQty = tx.janQuantity || 0;
                                if (janQty === 0 && tx.yjQuantity) {
                                    if (tx.janPackInnerQty > 0) {
                                        janQty = tx.yjQuantity / tx.janPackInnerQty;
                                    } else if (tx.yjQuantity > 0) {
                                        calculationErrorByProduct[tx.janCode] = '包装数量(内)未設定';
                                    }
                                }
                                const signedJanQty = janQty * (tx.flag === 1 || tx.flag === 11 || tx.flag === 4 ? 1 : -1);
                                todayNetChangeByProduct[tx.janCode] = (todayNetChangeByProduct[tx.janCode] || 0) + signedJanQty;
                            }
                        });
                    }
                });
            }
        });
    }

    document.querySelectorAll('.physical-stock-input').forEach(input => {
        const productCode = input.dataset.productCode;
        const displaySpan = document.querySelector(`.calculated-previous-day-stock[data-product-code="${productCode}"]`);
        const finalInput = document.querySelector(`.final-inventory-input[data-product-code="${productCode}"]`);

        if (calculationErrorByProduct[productCode]) {
            if (displaySpan) displaySpan.innerHTML = `<span style="color: red;">${calculationErrorByProduct[productCode]}</span>`;
            if (finalInput) finalInput.value = '';
            updateFinalInventoryTotal(productCode);
            return;
        }

        const physicalStockToday = parseFloat(input.value) || 0;
        const precompStock = precompTotalsByProduct[productCode] || 0;
        const netChangeToday = todayNetChangeByProduct[productCode] || 0;
        const totalStockToday = physicalStockToday + precompStock;
        const calculatedPreviousDayStock = totalStockToday - netChangeToday;
        
        if (displaySpan) displaySpan.textContent = calculatedPreviousDayStock.toFixed(2);
        if (finalInput) {
            finalInput.value = calculatedPreviousDayStock.toFixed(2);
            updateFinalInventoryTotal(productCode);
        }
    });
}

function updateFinalInventoryTotal(productCode) {
    const tbody = document.querySelector(`.final-input-tbody[data-product-code="${productCode}"]`);
    if (!tbody) return;
    let totalQuantity = 0;
    tbody.querySelectorAll('.final-inventory-input, .lot-quantity-input').forEach(input => {
        totalQuantity += parseFloat(input.value) || 0;
    });
}

function findMaster(productCode) {
    if (!lastLoadedDataCache || !lastLoadedDataCache.transactionLedger || lastLoadedDataCache.transactionLedger.length === 0) {
        return null;
    }
    for (const pkgLedger of lastLoadedDataCache.transactionLedger[0].packageLedgers) {
        const master = (pkgLedger.masters || []).find(m => m.productCode === productCode);
        if (master) {
            return master;
        }
    }
    return null;
}

async function saveInventoryData() {
    const dateInput = document.getElementById('inventory-date');
    if (!dateInput || !dateInput.value) {
        window.showNotification('棚卸日を指定してください。', 'error');
        return;
    }
    if (!confirm(`${dateInput.value}の棚卸データとして保存します。よろしいですか？`)) return;

    const inventoryData = {};
    const deadStockData = [];
    const allMasters = (lastLoadedDataCache.transactionLedger[0].packageLedgers || []).flatMap(pkg => pkg.masters || []);
    
    allMasters.forEach(master => {
        const productCode = master.productCode;
        const tbody = document.querySelector(`.final-input-tbody[data-product-code="${productCode}"]`);
        if (!tbody) {
            inventoryData[productCode] = 0;
            return;
        };
        let totalInputQuantity = 0;
        const inventoryRows = tbody.querySelectorAll('.inventory-row');
        for (let i = 0; i < inventoryRows.length; i += 2) {
            const topRow = inventoryRows[i];
            const bottomRow = inventoryRows[i+1];
            const quantityInput = bottomRow.querySelector('.final-inventory-input, .lot-quantity-input');
            const expiryInput = topRow.querySelector('.expiry-input');
            const lotInput = bottomRow.querySelector('.lot-input');
            if (!quantityInput || !expiryInput || !lotInput) continue;
            const quantity = parseFloat(quantityInput.value) || 0;
            const expiry = expiryInput.value.trim();
            const lot = lotInput.value.trim();
            totalInputQuantity += quantity;
            if (quantity > 0 && (expiry || lot)) {
                deadStockData.push({ 
                    productCode, 
                    yjCode: master.yjCode, packageForm: master.packageForm,
                    janPackInnerQty: master.janPackInnerQty, yjUnitName: master.yjUnitName,
                    stockQuantityJan: quantity, expiryDate: expiry, lotNumber: lot 
                });
            }
        }
        inventoryData[productCode] = totalInputQuantity;
    });
    
    const payload = {
        date: dateInput.value.replace(/-/g, ''),
        yjCode: currentYjCode,
        inventoryData: inventoryData,
        deadStockData: deadStockData,
    };
    
    window.showLoading();
    try {
        const res = await fetch('/api/inventory/adjust/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
        window.showNotification(resData.message, 'success');
        loadAndRenderDetails(currentYjCode);
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

export async function initLogic() {
    let unitMap = {};
    try {
        const res = await fetch('/api/units/map');
        if (!res.ok) throw new Error('単位マスタの取得に失敗');
        unitMap = await res.json();
        setUnitMap(unitMap); // UIモジュールに単位マップを設定
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    }

    view = document.getElementById('inventory-adjustment-view');
    if (!view) return;
    dosageFormFilter = document.getElementById('ia-dosageForm');
    kanaInitialFilter = document.getElementById('ia-kanaInitial');
    selectProductBtn = document.getElementById('ia-select-product-btn');
    deadStockOnlyFilter = document.getElementById('ia-dead-stock-only');
    outputContainer = document.getElementById('inventory-adjustment-output');
    
    barcodeInput = document.getElementById('ia-barcode-input');
    const barcodeForm = document.getElementById('ia-barcode-form');
    shelfNumberInput = document.getElementById('ia-shelf-number');

    if (barcodeForm) {
        barcodeForm.addEventListener('submit', handleBarcodeScan);
    }
    selectProductBtn.addEventListener('click', onSelectProductClick);
    outputContainer.addEventListener('input', handleInputChanges);
    outputContainer.addEventListener('click', handleClicks);
    outputContainer.addEventListener('submit', (e) => {
        if (e.target.id === 'adjustment-barcode-form') {
            handleAdjustmentBarcodeScan(e);
        }
    });
    view.addEventListener('loadInventoryAdjustment', (e) => {
        const { yjCode } = e.detail;
        if (yjCode) {
            dosageFormFilter.value = '';
            kanaInitialFilter.value = '';
            deadStockOnlyFilter.checked = false;
            shelfNumberInput.value = '';
            loadAndRenderDetails(yjCode);
        }
    });
}