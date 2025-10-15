// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/master_edit_ui.js

import { wholesalerMap } from './master_data.js';

let unitMap = {};

export function setUnitMap(map) {
    unitMap = map;
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

export function renderMasters(mastersToRender, tableContainer) {
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

export function updateShelfScannedList(shelfScannedList, shelfScannedCount, shelfScanPool) {
    shelfScannedCount.textContent = shelfScanPool.length;
    shelfScannedList.innerHTML = shelfScanPool.map(item => `
        <div class="scanned-item" data-gs1="${item.gs1}">
            <span class="scanned-item-name ${item.name ? 'resolved' : ''}">${item.name || item.gs1}</span>
        </div>
    `).join('');
}

export { createMasterRowHTML, formatPackageSpecForRow };