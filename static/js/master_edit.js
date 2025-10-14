// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\master_edit.js

import { showModal } from './inout_modal.js';
import { hiraganaToKatakana } from './utils.js';
import { wholesalerMap } from './master_data.js';

let view, tableContainer, refreshBtn, addRowBtn, kanaNameInput, dosageFormInput, shelfNumberInput;
let allMasters = []; // サーバーから取得した全マスターデータを保持する配列
let unitMap = {};

// ▼▼▼【ここから追加】棚番連続登録モーダル用の変数を追加▼▼▼
let shelfModal, closeShelfModalBtn, bulkShelfNumberBtn;
let shelfBarcodeForm, shelfBarcodeInput, shelfScannedList, shelfScannedCount;
let shelfNumberInputForBulk, shelfRegisterBtn, shelfClearBtn;
let shelfScanPool = []; // GS1コードを溜めるプール
// ▲▲▲【追加ここまで】▲▲▲

// 内部で使用するCSSを定義
const style = document.createElement('style');
style.innerHTML = `
    .master-edit-table .field-group { display: flex; flex-direction: column; gap: 2px; }
    .master-edit-table .field-group label { font-size: 10px; font-weight: bold; color: #555; }
    .master-edit-table .field-group input, .master-edit-table .field-group select, .master-edit-table .field-group textarea { width: 100%; font-size: 12px; padding: 4px; border: 1px solid #ccc; }
    .master-edit-table textarea { resize: vertical; min-height: 40px; }
    .master-edit-table .flags-container { display: flex; gap: 5px; }
    .master-edit-table .flags-container .field-group { flex: 1; }
    .master-edit-table input:read-only, .master-edit-table select:disabled, .master-edit-table input:disabled { background-color: #e9ecef; color: #6c757d; cursor: not-allowed; }
    
    .save-success {
        animation: flash-green 1.5s ease-out;
    }
    @keyframes flash-green {
        0% { background-color: #d1e7dd; }
        100% { background-color: transparent; }
    }
`;
document.head.appendChild(style);


async function fetchUnitMap() {
    if (Object.keys(unitMap).length > 0) return;
    try {
        const res = await fetch('/api/units/map');
        if (!res.ok) throw new Error('単位マスタの取得に失敗');
        unitMap = await res.json();
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    }
}

function formatPackageSpecForRow(tbody) {
    if (!tbody) return;
    const getVal = (name) => tbody.querySelector(`[name="${name}"]`)?.value || '';
    const packageForm = getVal('packageForm');
    const yjPackUnitQty = getVal('yjPackUnitQty');
    const yjUnitName = getVal('yjUnitName');
    const janPackInnerQty = getVal('janPackInnerQty');
    const janPackUnitQty = getVal('janPackUnitQty');
    const janUnitCode = getVal('janUnitCode');
    
    let formattedSpec = `${packageForm} ${yjPackUnitQty}${yjUnitName}`;
    if (parseFloat(janPackInnerQty) > 0 && parseFloat(janPackUnitQty) > 0) {
        let janUnitName = (janUnitCode === '0' || janUnitCode === '') ?
            '' : (unitMap[janUnitCode] || '');
        formattedSpec += ` (${janPackInnerQty}${yjUnitName}×${janPackUnitQty}${janUnitName})`;
    }
    const targetCell = tbody.querySelector('.formatted-spec-cell');
    if (targetCell) targetCell.textContent = formattedSpec;
}

function createMasterRowHTML(master = {}) {
    const isNew = !master.productCode;
    const rowId = master.productCode || `new-${Date.now()}`;
    const isProtected = master.origin === 'JCSHMS';
    const disabledAttr = isProtected ? 'disabled' : '';

    const row1 = `
        <tr class="data-row-top">
            <td colspan="2"><div class="field-group"><label>1. JAN</label><input type="text" name="productCode" value="${master.productCode || ''}" placeholder="製品コード(JAN)" ${!isNew ? 'readonly' : ''}></div></td>
            <td colspan="2"><div class="field-group"><label>2. YJ</label><input type="text" name="yjCode" value="${master.yjCode || ''}" placeholder="YJコード" ${isNew || isProtected ? 'readonly' : ''}></div></td>
            <td colspan="2"><div class="field-group"><label>3. GS1</label><input type="text" name="gs1Code" value="${master.gs1Code || ''}" ${disabledAttr}></div></td>
            <td colspan="3"><div class="field-group"><label>4. 商品名</label><input type="text" name="productName" value="${master.productName || ''}" ${disabledAttr}></div></td>
            <td colspan="3"><div class="field-group"><label>5. カナ</label><input type="text" name="kanaName" value="${master.kanaName || ''}" ${disabledAttr}></div></td>
        </tr>`;

    const row2 = `
        <tr class="data-row-middle">
            <td colspan="3"><div class="field-group"><label>6. メーカー</label><input type="text" name="makerName" value="${master.makerName || ''}" ${disabledAttr}></div></td>
            <td colspan="3"><div class="field-group"><label>7. 規格容量</label><input type="text" name="specification" value="${master.specification || ''}" ${disabledAttr}></div></td>
            <td colspan="2"><div class="field-group"><label>8. 剤型</label><input type="text" name="usageClassification" value="${master.usageClassification || ''}" ${disabledAttr}></div></td>
            <td colspan="4"><div class="field-group"><label>9. 包装</label><input type="text" name="packageForm" value="${master.packageForm || ''}" ${disabledAttr}></div></td>
        </tr>`;
    
    let janUnitOptions = '<option value="0">YJ単位と同じ</option>';
    for (const [code, name] of Object.entries(unitMap)) {
        if (code !== '0') janUnitOptions += `<option value="${code}" ${code == (master.janUnitCode || 0) ? 'selected' : ''}>${name}</option>`;
    }
    const flags = [
        { key: 'flagPoison', lbl: '18.毒' }, { key: 'flagDeleterious', lbl: '19.劇' }, { key: 'flagNarcotic', lbl: '20.麻' },
        { key: 'flagPsychotropic', lbl: '21.向' }, { key: 'flagStimulant', lbl: '22.覚' }, { key: 'flagStimulantRaw', lbl: '23.覚原' }
    ];
    const flagSelectorsHTML = flags.map(f => {
        const opts = (f.key === 'flagPsychotropic') ? [0, 1, 2, 3] : [0, 1];
        const options = opts.map(o => `<option value="${o}" ${o == (master[f.key] || 0) ? 'selected' : ''}>${o}</option>`).join('');
        return `<div class="field-group"><label style="text-align:center;">${f.lbl}</label><select name="${f.key}" ${disabledAttr}>${options}</select></div>`;
    }).join('');

    const row3 = `
        <tr class="data-row-middle">
            <td><div class="field-group"><label>10. YJ単位</label><input type="text" name="yjUnitName" value="${master.yjUnitName || ''}" ${disabledAttr}></div></td>
            <td><div class="field-group"><label>11. YJ数量</label><input type="number" name="yjPackUnitQty" value="${master.yjPackUnitQty || 0}" ${disabledAttr}></div></td>
            <td><div class="field-group"><label>12. 包装数</label><input type="number" name="janPackInnerQty" value="${master.janPackInnerQty || 0}" ${disabledAttr}></div></td>
            <td><div class="field-group"><label>13. JAN単位</label><select name="janUnitCode" ${disabledAttr}>${janUnitOptions}</select></div></td>
            <td><div class="field-group"><label>14. JAN数量</label><input type="number" name="janPackUnitQty" value="${master.janPackUnitQty || 0}" ${disabledAttr}></div></td>
            <td><div class="field-group"><label>15. マスタ</label><input type="text" name="origin" value="${master.origin || ''}" readonly></div></td>
            <td><div class="field-group"><label>16. 薬価</label><input type="number" name="nhiPrice" value="${master.nhiPrice || 0}" step="0.0001" ${disabledAttr}></div></td>
            <td colspan="5"><div class="flags-container">${flagSelectorsHTML}</div></td>
        </tr>`;

    let wholesalerOptions = '<option value="">--- 選択 ---</option>';
    wholesalerMap.forEach((name, code) => {
        wholesalerOptions += `<option value="${code}" ${code === master.supplierWholesale ? 'selected' : ''}>${name}</option>`;
    });
    const isOrderStoppedOptions = `<option value="0" ${ (master.isOrderStopped || 0) == 0 ? 'selected' : '' }>可</option><option value="1" ${ (master.isOrderStopped || 0) == 1 ? 'selected' : '' }>不可</option>`;
    
    const row4 = `
        <tr class="data-row-user">
            <td colspan="2"><div class="field-group"><label>17. 納入価(包装)</label><input type="number" name="purchasePrice" value="${master.purchasePrice || ''}" step="0.01"></div></td>
            <td colspan="2"><div class="field-group"><label>24. 卸</label><select name="supplierWholesale">${wholesalerOptions}</select></div></td>
            <td><div class="field-group"><label>25. 発注</label><select name="isOrderStopped">${isOrderStoppedOptions}</select></div></td>
            <td><div class="field-group"><label>26. グループ</label><input type="text" name="groupCode" value="${master.groupCode || ''}"></div></td>
            <td><div class="field-group"><label>27. 棚番</label><input type="text" name="shelfNumber" value="${master.shelfNumber || ''}"></div></td>
            <td><div class="field-group"><label>28. 分類</label><input type="text" name="category" value="${master.category || ''}"></div></td>
            <td colspan="2"><div class="field-group"><label>29. メモ</label><textarea name="userNotes">${master.userNotes || ''}</textarea></div></td>
            <td><div class="field-group"><label>&nbsp;</label><button class="save-master-btn btn">30.保存</button></div></td>
            <td><div class="field-group"><label>&nbsp;</label><button class="quote-jcshms-btn btn" ${disabledAttr}>31.引用</button></div></td>
        </tr>`;

    return `<tbody data-record-id="${rowId}">${row1}${row2}${row3}${row4}</tbody>`;
}

function renderMasters(mastersToRender) {
    if (!mastersToRender || mastersToRender.length === 0) {
        tableContainer.innerHTML = '<p>対象のマスターが見つかりません。</p>';
        return;
    }
    const allTablesHTML = mastersToRender.map(master => {
        const tableContent = createMasterRowHTML(master);
        return `<table class="data-table master-edit-table">${tableContent}</table>`;
    }).join('');
    tableContainer.innerHTML = allTablesHTML;
}

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
    
    renderMasters(filteredMasters);
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

// ▼▼▼【ここから修正】棚番連続登録用の関数群を修正・追加▼▼▼

/**
 * 棚番登録モーダル内のスキャン済みリスト表示を更新する
 */
function updateShelfScannedList() {
    shelfScannedCount.textContent = shelfScanPool.length;
    shelfScannedList.innerHTML = shelfScanPool.map(item => `
        <div class="scanned-item" data-gs1="${item.gs1}">
            <span class="scanned-item-name ${item.name ? 'resolved' : ''}">${item.name || item.gs1}</span>
        </div>
    `).join('');
}

/**
 * スキャンされたGS1コードに対応する品目名を取得して表示を更新する
 * 見つからない場合は仮マスター作成を試みる
 * @param {string} gs1 - GS1コード
 */
async function resolveProductNameForShelfScan(gs1) {
    try {
        const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1}`);
        if (res.ok) {
            const product = await res.json();
            const poolItem = shelfScanPool.find(item => item.gs1 === gs1);
            if (poolItem) {
                poolItem.name = product.productName;
                const listItem = shelfScannedList.querySelector(`.scanned-item[data-gs1="${gs1}"] .scanned-item-name`);
                if (listItem) {
                    listItem.textContent = product.productName;
                    listItem.classList.add('resolved');
                }
            }
        } else if (res.status === 404) {
            // マスターが見つからない場合、ユーザーに確認して仮マスターを作成
            if (confirm(`このGS1コード[${gs1}]はマスターに登録されていません。\n新規に仮マスターを作成しますか？`)) {
                const newMaster = await createProvisionalMasterForShelf(gs1);
                const poolItem = shelfScanPool.find(item => item.gs1 === gs1);
                 if (poolItem) {
                    poolItem.name = newMaster.productName;
                    const listItem = shelfScannedList.querySelector(`.scanned-item[data-gs1="${gs1}"] .scanned-item-name`);
                    if (listItem) {
                        listItem.textContent = newMaster.productName;
                        listItem.classList.add('resolved');
                    }
                }
            }
        }
    } catch (err) {
        console.warn(`品目名の解決に失敗: ${gs1}`, err);
        window.showNotification(`品目名の解決に失敗しました: ${gs1}`, 'error');
    }
}


/**
 * 棚番登録フローの中で、GS1コードから仮マスターを作成する
 * @param {string} gs1 - GS1コード
 */
async function createProvisionalMasterForShelf(gs1) {
    window.showLoading('新規マスターを作成中...');
    try {
        // GS1コードからJANコードを生成（単純なルール）
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

        // 作成したマスター情報を再取得して返す
        const fetchRes = await fetch(`/api/product/by_gs1?gs1_code=${gs1}`);
        if (!fetchRes.ok) {
            throw new Error('作成したマスター情報の取得に失敗しました。');
        }
        return await fetchRes.json();

    } catch (err) {
        window.showNotification(err.message, 'error');
        throw err; // エラーを呼び出し元に伝播させる
    } finally {
        window.hideLoading();
    }
}


/**
 * 「この棚番で登録」ボタンが押されたときの処理
 */
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
        updateShelfScannedList();
        shelfNumberInputForBulk.value = '';
        shelfBarcodeInput.focus();

        loadAndRenderMasters();

    } catch (err) {
        window.showNotification(`エラー: ${err.message}`, 'error');
    } finally {
        window.hideLoading();
    }
}

// ▲▲▲【修正ここまで】▲▲▲

export async function initMasterEdit() {
    view = document.getElementById('master-edit-view');
    if (!view) return;
    tableContainer = document.getElementById('master-edit-container');
    refreshBtn = document.getElementById('refreshMastersBtn');
    addRowBtn = document.getElementById('addMasterRowBtn');
    kanaNameInput = document.getElementById('master-edit-kanaName');
    dosageFormInput = document.getElementById('master-edit-dosageForm');
    shelfNumberInput = document.getElementById('master-edit-shelfNumber');

    // ▼▼▼【ここから追加】棚番モーダル用の要素を取得・イベント設定▼▼▼
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
        updateShelfScannedList();
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
            // 重複チェック
            if (!shelfScanPool.some(item => item.gs1 === barcode)) {
                shelfScanPool.push({ gs1: barcode, name: null });
                updateShelfScannedList();
                resolveProductNameForShelfScan(barcode); // 裏で品目名を取得
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
            updateShelfScannedList();
            shelfBarcodeInput.focus();
        }
    });
    // ▲▲▲【追加ここまで】▲▲▲

    await fetchUnitMap();

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

    kanaNameInput.addEventListener('input', applyFiltersAndRender);
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
}

export function resetMasterEditView() {
    if (tableContainer) {
        if(kanaNameInput) kanaNameInput.value = '';
        if(dosageFormInput) dosageFormInput.value = '';
        if(shelfNumberInput) shelfNumberInput.value = '';
        loadAndRenderMasters();
    }
}