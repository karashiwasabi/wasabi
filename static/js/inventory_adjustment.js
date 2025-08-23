// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\inventory_adjustment.js

import { showModal } from './inout_modal.js';
import { transactionTypeMap } from './common_table.js';

let view, outputContainer;
let dosageFormFilter, kanaInitialFilter, selectProductBtn, startDateFilter, endDateFilter;
let currentYjCode = null;
let lastLoadedDataCache = null;

export function initInventoryAdjustment() {
    view = document.getElementById('inventory-adjustment-view');
    if (!view) return;

    dosageFormFilter = document.getElementById('ia-dosageForm');
    kanaInitialFilter = document.getElementById('ia-kanaInitial');
    selectProductBtn = document.getElementById('ia-select-product-btn');
    startDateFilter = document.getElementById('ia-startDate');
    endDateFilter = document.getElementById('ia-endDate');
    outputContainer = document.getElementById('inventory-adjustment-output');

    const today = new Date();
    const oneMonthAgo = new Date(today.getFullYear(), today.getMonth() - 1, today.getDate());
    endDateFilter.value = today.toISOString().slice(0, 10);
    startDateFilter.value = oneMonthAgo.toISOString().slice(0, 10);

    selectProductBtn.addEventListener('click', onSelectProductClick);
    outputContainer.addEventListener('input', handleInputChanges);
    outputContainer.addEventListener('click', handleClicks);
}

async function onSelectProductClick() {
    const dosageForm = dosageFormFilter.value;
    const kanaInitial = kanaInitialFilter.value;
    const apiUrl = `/api/products/search_filtered?dosageForm=${encodeURIComponent(dosageForm)}&kanaInitial=${encodeURIComponent(kanaInitial)}`;
    
    window.showLoading();
    try {
        const res = await fetch(apiUrl);
        if (!res.ok) throw new Error('品目リストの取得に失敗しました。');
        const products = await res.json();
        window.hideLoading();
        showModal(view, (selectedProduct) => {
            loadAndRenderDetails(selectedProduct.yjCode);
        }, { initialResults: products, searchApi: apiUrl });
    } catch (err) {
        window.hideLoading();
        window.showNotification(err.message, 'error');
    }
}

async function loadAndRenderDetails(yjCode) {
    currentYjCode = yjCode;
    const startDate = startDateFilter.value.replace(/-/g, '');
    const endDate = endDateFilter.value.replace(/-/g, '');
    if (!yjCode || !startDate || !endDate) {
        window.showNotification('YJコードと期間を指定してください。', 'error');
        return;
    }

    window.showLoading();
    outputContainer.innerHTML = '<p>データを読み込んでいます...</p>';
    try {
        const apiUrl = `/api/inventory/adjust/data?yjCode=${yjCode}&startDate=${startDate}&endDate=${endDate}`;
        const res = await fetch(apiUrl);
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'データ取得に失敗しました。');
        }
        lastLoadedDataCache = await res.json();
        const html = generateFullHtml(lastLoadedDataCache);
        outputContainer.innerHTML = html;
        
        const dateInput = document.getElementById('inventory-date');
        if(dateInput) dateInput.value = endDateFilter.value;
    } catch (err) {
        outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

function generateFullHtml(data) {
    if (!data.stockLedger || data.stockLedger.length === 0) {
        return '<p>対象の製品データが見つかりませんでした。</p>';
    }
    const yjGroup = data.stockLedger[0];
    const productName = yjGroup.productName;
    const theoreticalTotal = yjGroup.endingBalance || 0;
    const allMasters = (yjGroup.packageLedgers || []).flatMap(pkg => pkg.masters || []);

    const summaryLedgerHtml = generateSummaryLedgerHtml(yjGroup, theoreticalTotal);
    const summaryPrecompHtml = generateSummaryPrecompHtml(data.precompDetails);
    // ▼▼▼ [修正点] generateInputSectionsHtmlにallMastersを渡すように変更 ▼▼▼
    const inputSectionsHtml = generateInputSectionsHtml(allMasters, yjGroup.yjUnitName);
    // ▲▲▲ 修正ここまで ▲▲▲

    return `<h2 style="text-align: center; margin-bottom: 20px;">【棚卸調整】 ${productName} (YJ: ${yjGroup.yjCode})</h2>
        ${summaryLedgerHtml}${summaryPrecompHtml}${inputSectionsHtml}`;
}

function generateSummaryLedgerHtml(yjGroup, total) {
    const startDate = startDateFilter.value;
    const endDate = endDateFilter.value;
    let transactions = [];
    (yjGroup.packageLedgers || []).forEach(pkg => {
        (pkg.transactions || []).forEach(tx => {
            const master = (pkg.masters || []).find(m => m.productCode === tx.janCode) || {};
            tx.packageSpec = master.formattedPackageSpec || master.packageForm || '';
            tx.makerName = master.makerName || '';
            transactions.push(tx);
        });
    });
    transactions.sort((a, b) => (a.transactionDate + a.id).toString().localeCompare(b.transactionDate + b.id));

    return `<div class="summary-section"><h3 class="view-subtitle">1. 全体サマリー</h3>
        <div class="report-section-header"><h4>在庫元帳 (期間: ${startDate} ～ ${endDate})</h4>
        <span class="header-total">理論在庫合計: ${total.toFixed(2)} ${yjGroup.yjUnitName}</span></div>
        ${renderStandardTable('ledger-table', transactions)}</div>`;
}

function generateSummaryPrecompHtml(precompDetails) {
    const precompTransactions = (precompDetails || []).map(p => ({
        transactionDate: p.createdAt.slice(0, 10), flag: '予製', clientCode: `患者: ${p.patientNumber}`,
        receiptNumber: `PRECOMP-${p.id}`, yjQuantity: p.quantity, yjUnitName: p.yjUnitName,
        janCode: p.productCode, productName: p.productName,
    }));
    return `<div class="summary-section" style="margin-top: 15px;">
        <div class="report-section-header"><h4>予製払出明細 (全体)</h4>
        <span class="header-total" id="precomp-active-total">有効合計: 0.00</span></div>
        ${renderStandardTable('precomp-table', precompTransactions, true)}</div>`;
}

// ▼▼▼ [修正点] 関数全体を、包装単位でグループ化するロジックに全面的に書き換え ▼▼▼
function generateInputSectionsHtml(allMasters, yjUnitName = '単位') {
    // 1. マスターを包装(packageKey)ごとにグループ化する
    const packagesMap = new Map();
    allMasters.forEach(master => {
        if (!master) return;
        const key = `${master.packageForm}|${master.janPackInnerQty}|${master.yjUnitName}`; // 包装を特定するキー
        if (!packagesMap.has(key)) {
            packagesMap.set(key, { masters: [], theoreticalStock: 0 });
        }
        const pkg = packagesMap.get(key);
        pkg.masters.push(master);
        pkg.theoreticalStock += (lastLoadedDataCache.stockLedger[0]?.packageLedgers
            .flatMap(p => p.masters)
            .find(m => m.productCode === master.productCode)?.currentStock || 0);
    });

    // 2. グループ化したデータからHTMLを生成する
    let packageGroupsHtml = '';
    for (const [key, pkgGroup] of packagesMap.entries()) {
        const theoreticalStockText = `理論在庫(包装計): ${pkgGroup.theoreticalStock.toFixed(2)} ${yjUnitName}`;
        
        // 包装グループのヘッダー
        packageGroupsHtml += `
            <div class="package-input-group" style="margin-bottom: 20px;">
                <div class="agg-pkg-header">
                    <span>包装: ${key}</span>
                    <span class="balance-info">${theoreticalStockText}</span>
                </div>`;

        // 包装に属する各製品(JAN)の入力欄
        pkgGroup.masters.forEach(master => {
            const unitName = master.janPackInnerQty > 0 ? '(JAN単位)' : `(${master.yjUnitName})`;
            const shelfStockInput = `<div class="shelf-stock-input-area">
                <label>棚在庫(目で見た数量) - ${master.productName}:</label>
                <input type="number" class="shelf-stock-input" data-product-code="${master.productCode}">
                <span>${unitName}</span>
                <span class="calculated-total-display" data-product-code="${master.productCode}">=> 最終在庫(目安): 0.00</span>
            </div>`;

            const finalInputTable = renderStandardTable(`final-table-${master.productCode}`, [], false, 
                `<tbody class="final-input-tbody" data-product-code="${master.productCode}">
                    ${createFinalInputRow(master, true)}
                </tbody>`);
            
            packageGroupsHtml += `<div class="product-input-group" style="padding-left: 20px;">${shelfStockInput}${finalInputTable}</div>`;
        });

        packageGroupsHtml += `</div>`; // .package-input-group
    }

    return `<div class="input-section" style="margin-top: 30px;"><h3 class="view-subtitle">2. 棚卸入力</h3>
        <div class="inventory-input-area" style="padding: 10px; border: 1px solid #ccc; background-color: #f8f9fa; margin-bottom: 15px;">
             <label for="inventory-date" style="font-weight: bold;">棚卸日:</label>
             <input type="date" id="inventory-date"></div>
        ${packageGroupsHtml}</div>`;
}
// ▲▲▲ 修正ここまで ▲▲▲

function createFinalInputRow(master, isPrimary = false) {
    const actionButtons = isPrimary ? `
        <button class="btn add-deadstock-row-btn" data-product-code="${master.productCode}">＋</button>
        <button class="btn register-inventory-btn">登録</button>
    ` : `<button class="btn delete-deadstock-row-btn">－</button>`;
    const quantityInputClass = isPrimary ? 'final-inventory-input' : 'lot-quantity-input';
    const quantityPlaceholder = isPrimary ? '目安をここに転記' : 'ロット数量';
    const topRow = `<tr class="inventory-row"><td rowspan="2"><div style="display: flex; flex-direction: column; gap: 4px;">${actionButtons}</div></td>
        <td>(棚卸日)</td><td class="yj-jan-code">${master.yjCode}</td><td class="left" colspan="2">${master.productName}</td>
        <td></td><td></td><td class="right">${master.yjPackUnitQty || ''}</td><td>${master.yjUnitName || ''}</td>
        <td></td><td></td><td><input type="text" class="expiry-input" placeholder="YYYYMM"></td><td></td><td></td></tr>`;
    const bottomRow = `<tr class="inventory-row"><td>棚卸</td><td class="yj-jan-code">${master.productCode}</td>
        <td>${master.formattedPackageSpec || ''}</td><td>${master.makerName || ''}</td><td>${master.usageClassification || ''}</td>
        <td><input type="number" class="${quantityInputClass}" data-product-code="${master.productCode}" placeholder="${quantityPlaceholder}"></td>
        <td class="right">${master.janPackUnitQty || ''}</td><td>${master.janUnitName || ''}</td>
        <td></td><td></td><td><input type="text" class="lot-input" placeholder="ロット番号"></td><td></td><td></td></tr>`;
    return topRow + bottomRow;
}

function renderStandardTable(id, records, addCheckbox = false, customBody = null) {
    const header = `<thead>
           <tr><th rowspan="2">－</th><th>日付</th><th>YJ</th><th colspan="2">製品名</th><th>個数</th><th>YJ数量</th><th>YJ包装数</th><th>YJ単位</th><th>単価</th><th>税額</th><th>期限</th><th>得意先</th><th>行</th></tr>
           <tr><th>種別</th><th>JAN</th><th>包装</th><th>メーカー</th><th>剤型</th><th>JAN数量</th><th>JAN包装数</th><th>JAN単位</th><th>金額</th><th>税率</th><th>ロット</th><th>伝票番号</th><th>MA</th></tr></thead>`;
    let bodyHtml = customBody ? customBody : `<tbody>${(!records || records.length === 0) ? '<tr><td colspan="14">対象データがありません。</td></tr>' : records.map(rec => {
        const top = `<tr><td rowspan="2">${addCheckbox ? `<input type="checkbox" class="precomp-active-check" data-quantity="${rec.yjQuantity}" data-product-code="${rec.janCode}">` : ''}</td>
            <td>${rec.transactionDate || ''}</td><td class="yj-jan-code">${rec.yjCode || ''}</td><td class="left" colspan="2">${rec.productName || ''}</td>
            <td class="right">${rec.datQuantity?.toFixed(2) || ''}</td><td class="right">${rec.yjQuantity?.toFixed(2) || ''}</td><td class="right">${rec.yjPackUnitQty || ''}</td><td>${rec.yjUnitName || ''}</td>
            <td class="right">${rec.unitPrice?.toFixed(4) || ''}</td><td class="right">${rec.taxAmount?.toFixed(2) || ''}</td><td>${rec.expiryDate || ''}</td><td class="left">${rec.clientCode || ''}</td><td class="right">${rec.lineNumber || ''}</td></tr>`;
        const bottom = `<tr><td>${transactionTypeMap[rec.flag] || rec.flag}</td><td class="yj-jan-code">${rec.janCode || ''}</td><td>${rec.packageSpec || ''}</td><td>${rec.makerName || ''}</td>
            <td>${rec.usageClassification || ''}</td><td class="right">${rec.janQuantity?.toFixed(2) || ''}</td><td class="right">${rec.janPackUnitQty || ''}</td><td>${rec.janUnitName || ''}</td>
            <td class="right">${rec.subtotal?.toFixed(2) || ''}</td><td class="right">${rec.taxRate != null ? (rec.taxRate * 100).toFixed(0) + "%" : ""}</td><td>${rec.lotNumber || ''}</td><td class="left">${rec.receiptNumber || ''}</td><td class="left">${rec.processFlagMA || ''}</td></tr>`;
        return top + bottom;
    }).join('')}</tbody>`;
    return `<table class="data-table" id="${id}">${header}${bodyHtml}</table>`;
}

function handleInputChanges(e) {
    const targetClassList = e.target.classList;
    if (targetClassList.contains('precomp-active-check') || targetClassList.contains('shelf-stock-input')) {
        recalculateTotals();
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
            const newRowHTML = createFinalInputRow(master, false);
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

function recalculateTotals() {
    const precompTotalsByPackage = {};
    document.querySelectorAll('.precomp-active-check:checked').forEach(cb => {
        const productCode = cb.dataset.productCode;
        const yjQuantity = parseFloat(cb.dataset.quantity) || 0;
        precompTotalsByPackage[productCode] = (precompTotalsByPackage[productCode] || 0) + yjQuantity;
    });
    const grandTotal = Object.values(precompTotalsByPackage).reduce((sum, val) => sum + val, 0);
    const yjUnitName = lastLoadedDataCache?.stockLedger[0]?.yjUnitName || '';
    document.getElementById('precomp-active-total').textContent = `有効合計: ${grandTotal.toFixed(2)} ${yjUnitName}`;

    document.querySelectorAll('.shelf-stock-input').forEach(input => {
        const productCode = input.dataset.productCode;
        const shelfStockInputVal = parseFloat(input.value) || 0;
        const master = findMaster(productCode);
        if (!master) return;
        const isJanInput = master.janPackInnerQty > 0;
        const shelfStockYj = isJanInput ? shelfStockInputVal * master.janPackInnerQty : shelfStockInputVal;
        const precompStockYj = precompTotalsByPackage[productCode] || 0;
     
        const finalStockYj = shelfStockYj + precompStockYj;
        const finalStockForDisplay = isJanInput ? (finalStockYj / master.janPackInnerQty) : finalStockYj;
        const displaySpan = document.querySelector(`.calculated-total-display[data-product-code="${productCode}"]`);
        if(displaySpan) displaySpan.textContent = `=> 最終在庫(目安): ${finalStockForDisplay.toFixed(2)}`;
        const finalInput = document.querySelector(`.final-inventory-input[data-product-code="${productCode}"]`);
        if(finalInput) {
            finalInput.value = finalStockForDisplay.toFixed(2);
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
    const finalInput = tbody.querySelector('.final-inventory-input');
    if (finalInput && document.activeElement !== finalInput) {
        // This logic could be enhanced, but for now we just sum.
    }
}

function findMaster(productCode) {
    if (!lastLoadedDataCache) return null;
    const allMasters = (lastLoadedDataCache.stockLedger[0].packageLedgers || []).flatMap(pkg => pkg.masters || []);
    return allMasters.find(m => m.productCode === productCode);
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
    const allMasters = (lastLoadedDataCache.stockLedger[0].packageLedgers || []).flatMap(pkg => pkg.masters || []);
    allMasters.forEach(master => {
        const productCode = master.productCode;
        const tbody = document.querySelector(`.final-input-tbody[data-product-code="${productCode}"]`);
        if (!tbody) {
            inventoryData[productCode] = 0;
            return;
        };

        let totalInputQuantity = 0; // JAN単位またはYJ単位での合計
        const isJanInput = master.janPackInnerQty > 0;
      
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

        const totalYjQuantity = isJanInput ? totalInputQuantity * master.janPackInnerQty : totalInputQuantity;
        inventoryData[productCode] = totalYjQuantity;
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