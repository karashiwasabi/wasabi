// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\master_edit.js

import { showModal } from './inout_modal.js';
import { hiraganaToKatakana } from './utils.js';
import { wholesalerMap } from './master_data.js';

let view, tableContainer, refreshBtn, addRowBtn, kanaNameInput, dosageFormInput, shelfNumberInput;
let allMasters = []; // サーバーから取得した全マスターデータを保持する配列
let unitMap = {};

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
    
    /* ▼▼▼【ここに追加】▼▼▼ */
    .save-success {
        animation: flash-green 1.5s ease-out;
    }
    @keyframes flash-green {
        0% { background-color: #d1e7dd; }
        100% { background-color: transparent; }
    }
    /* ▲▲▲【追加ここまで】▲▲▲ */
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
    setVal('yjPackUnitQty', selectedProduct.yjPackUnitQty); setVal('janPackInnerQty', selectedProduct.janPackInnerQty);
    setVal('janUnitCode', selectedProduct.janUnitCode); setVal('janPackUnitQty', selectedProduct.janPackUnitQty);
	setVal('specification', selectedProduct.specification);
	setVal('gs1Code', selectedProduct.gs1Code);
    formatPackageSpecForRow(tbody);
}

export async function initMasterEdit() {
    view = document.getElementById('master-edit-view');
    if (!view) return;
    tableContainer = document.getElementById('master-edit-container');
    refreshBtn = document.getElementById('refreshMastersBtn');
    addRowBtn = document.getElementById('addMasterRowBtn');
    kanaNameInput = document.getElementById('master-edit-kanaName');
    dosageFormInput = document.getElementById('master-edit-dosageForm');
    shelfNumberInput = document.getElementById('master-edit-shelfNumber');

    await fetchUnitMap();

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

                // ▼▼▼【ここから修正】▼▼▼
                // loadAndRenderMasters(); を削除し、以下の処理に置き換え

                // 1. メモリ上のデータを更新
                const masterIndex = allMasters.findIndex(m => m.productCode === data.productCode);
                if (masterIndex > -1) {
                    // 既存マスターの更新
                    allMasters[masterIndex] = { ...allMasters[masterIndex], ...data };
                } else {
                    // 新規マスターの追加
                    allMasters.push(data);
                }

                // 2. DOMを直接更新
                if (isNew) {
                    const productCodeInput = tbody.querySelector('input[name="productCode"]');
                    productCodeInput.readOnly = true;
                    tbody.dataset.recordId = data.productCode;
                    const originInput = tbody.querySelector('input[name="origin"]');
                    originInput.value = data.origin;
                }
                
                // 3. 視覚的なフィードバック
                tbody.querySelectorAll('tr').forEach(row => {
                    row.classList.remove('save-success'); // アニメーションを再実行可能にするため一旦削除
                    void row.offsetWidth; // リフローを強制
                    row.classList.add('save-success'); // アニメーションクラスを追加
                });
                // ▲▲▲【修正ここまで】▲▲▲

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