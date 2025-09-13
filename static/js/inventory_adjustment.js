// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\inventory_adjustment.js

import { showModal } from './inout_modal.js';
import { transactionTypeMap } from './common_table.js';
import { wholesalerMap } from './master_data.js';
import { getLocalDateString } from './utils.js';


// ▼▼▼【修正】startDateFilter, endDateFilter を削除 ▼▼▼
let view, outputContainer;
let dosageFormFilter, kanaInitialFilter, selectProductBtn, deadStockOnlyFilter;
let currentYjCode = null;
let lastLoadedDataCache = null;
let unitMap = {};

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

export async function initInventoryAdjustment() {
    await fetchUnitMap();
    view = document.getElementById('inventory-adjustment-view');
    if (!view) return;

    dosageFormFilter = document.getElementById('ia-dosageForm');
    kanaInitialFilter = document.getElementById('ia-kanaInitial');
    selectProductBtn = document.getElementById('ia-select-product-btn');
    // ▼▼▼【修正】startDateFilter, endDateFilter の取得を削除 ▼▼▼
    deadStockOnlyFilter = document.getElementById('ia-dead-stock-only');
    outputContainer = document.getElementById('inventory-adjustment-output');
    
    // ▼▼▼【修正】日付のデフォルト値設定ロジックを削除 ▼▼▼

    selectProductBtn.addEventListener('click', onSelectProductClick);
    outputContainer.addEventListener('input', handleInputChanges);
    outputContainer.addEventListener('click', handleClicks);
}

async function onSelectProductClick() {
    const dosageForm = dosageFormFilter.value;
    const kanaInitial = kanaInitialFilter.value;
    const isDeadStockOnly = deadStockOnlyFilter.checked;
    
    // ▼▼▼【修正】URLSearchParamsから期間関連のパラメータを削除 ▼▼▼
    const params = new URLSearchParams({
        dosageForm: dosageForm,
        kanaInitial: kanaInitial,
        deadStockOnly: isDeadStockOnly,
    });
    
    const apiUrl = `/api/products/search_filtered?${params.toString()}`;
    
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
    // ▼▼▼【修正】startDate, endDate の取得とチェックを削除 ▼▼▼
    if (!yjCode) {
        window.showNotification('YJコードを指定してください。', 'error');
        return;
    }

    window.showLoading();
    outputContainer.innerHTML = '<p>データを読み込んでいます...</p>';
    try {
        // ▼▼▼【修正】APIのURLから期間パラメータを削除 ▼▼▼
        const apiUrl = `/api/inventory/adjust/data?yjCode=${yjCode}`;
        const res = await fetch(apiUrl);
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'データ取得に失敗しました。');
        }
        lastLoadedDataCache = await res.json();
        const html = generateFullHtml(lastLoadedDataCache);
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

function generateFullHtml(data) {
    // ▼▼▼ [修正点] 表示する元帳データを transactionLedger に変更 ▼▼▼
    if (!data.transactionLedger || data.transactionLedger.length === 0) {
        return '<p>対象の製品データが見つかりませんでした。</p>';
    }
    const yjGroup = data.transactionLedger[0];
    // ▲▲▲ 修正ここまで ▲▲▲

    const productName = yjGroup.productName;

    // ▼▼▼ [修正点] 理論在庫の参照先を yesterdaysStock に変更 ▼▼▼
    const yesterdaysTotal = data.yesterdaysStock ? (data.yesterdaysStock.endingBalance || 0) : 0;
    // ▲▲▲ 修正ここまで ▲▲▲

    const summaryLedgerHtml = generateSummaryLedgerHtml(yjGroup, yesterdaysTotal);
    const summaryPrecompHtml = generateSummaryPrecompHtml(data.precompDetails);
    
    const inputSectionsHtml = generateInputSectionsHtml(yjGroup.packageLedgers, yjGroup.yjUnitName);
    const deadStockReferenceHtml = generateDeadStockReferenceHtml(data.deadStockDetails);
    return `<h2 style="text-align: center; margin-bottom: 20px;">【棚卸調整】 ${productName} (YJ: ${yjGroup.yjCode})</h2>
        ${summaryLedgerHtml}
        ${summaryPrecompHtml}
        ${inputSectionsHtml}
        ${deadStockReferenceHtml}`;
}

function generateSummaryLedgerHtml(yjGroup, yesterdaysTotal) {
    // ▼▼▼【ここから修正】▼▼▼
    // 削除された日付入力欄を参照するのではなく、JSで日付を直接計算する
    const endDate = getLocalDateString();
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - 30); // 30日前に固定
    const startDateStr = startDate.toISOString().slice(0, 10);
    // ▲▲▲【修正ここまで】▲▲▲

    let packageLedgerHtml = (yjGroup.packageLedgers || []).map(pkg => {
        const sortedTxs = (pkg.transactions || []).sort((a, b) => 
            (a.transactionDate + a.id).toString().localeCompare(b.transactionDate + b.id)
        );
    
        const pkgHeader = `
            <div class="agg-pkg-header" style="margin-top: 10px;">
                <span>包装: ${pkg.packageKey}</span>
                <span class="balance-info">
                    本日理論在庫(包装計): ${(pkg.endingBalance || 0).toFixed(2)} ${yjGroup.yjUnitName}
                </span>
            </div>
        `;
        const txTable = renderStandardTable(`ledger-table-${pkg.packageKey.replace(/[^a-zA-Z0-9]/g, '')}`, sortedTxs);
        return pkgHeader + txTable;
    }).join('');

    return `<div class="summary-section">
        <h3 class="view-subtitle">1. 全体サマリー</h3>
        <div class="report-section-header">
            <h4>在庫元帳 (期間: ${startDateStr} ～ ${endDate})</h4>
            <span class="header-total">【参考】前日理論在庫合計: ${yesterdaysTotal.toFixed(2)} ${yjGroup.yjUnitName}</span>
        </div>
        ${packageLedgerHtml}
    </div>`;
}

function generateSummaryPrecompHtml(precompDetails) {
    const precompTransactions = (precompDetails || []).map(p => ({
        transactionDate: (p.transactionDate || '').slice(0, 8),
        flag: '予製',
        clientCode: p.clientCode ? `患者: ${p.clientCode}` : '',
        receiptNumber: p.receiptNumber,
        yjQuantity: p.yjQuantity,
        yjUnitName: p.yjUnitName,
        janCode: p.janCode,
        productName: p.productName,
        yjCode: p.yjCode,
        packageSpec: p.packageSpec,
        makerName: p.makerName,
        usageClassification: p.usageClassification,
        janQuantity: p.janQuantity,
        janPackUnitQty: p.janPackUnitQty,
        janUnitName: p.janUnitName
    }));
    return `<div class="summary-section" style="margin-top: 15px;">
        <div class="report-section-header"><h4>予製払出明細 (全体)</h4>
        <span class="header-total" id="precomp-active-total">有効合計: 0.00</span></div>
        ${renderStandardTable('precomp-table', precompTransactions, true)}</div>`;
}

function generateDeadStockReferenceHtml(deadStockRecords) {
    if (!deadStockRecords || deadStockRecords.length === 0) {
        return '';
    }

    const getProductName = (productCode) => {
        const master = findMaster(productCode);
        return master ? master.productName : productCode;
    };

    const rowsHtml = deadStockRecords.map(rec => `
        <tr>
            <td class="left">${getProductName(rec.productCode)}</td>
            <td class="right">${rec.stockQuantityJan.toFixed(2)}</td>
            <td>${rec.expiryDate || ''}</td>
            <td class="left">${rec.lotNumber || ''}</td>
        </tr>
    `).join('');
    return `
        <div class="summary-section" style="margin-top: 30px;">
            <h3 class="view-subtitle">3. 参考：現在登録済みのロット・期限情報</h3>
            <p style="font-size: 11px; margin-bottom: 5px;">※このリストは参照用です。棚卸情報を保存するには、上の「2. 棚卸入力」の欄に改めて入力してください。</p>
            <table class="data-table">
                <thead>
                    <tr>
                        <th style="width: 40%;">製品名</th>
                        <th style="width: 15%;">在庫数量(JAN)</th>
                        <th style="width: 20%;">使用期限</th>
                        <th style="width: 25%;">ロット番号</th>
                    </tr>
                </thead>
                <tbody>
                    ${rowsHtml}
                </tbody>
            </table>
        </div>
    `;
}

function generateInputSectionsHtml(packageLedgers, yjUnitName = '単位') {
    const packageGroupsHtml = (packageLedgers || []).map(pkgLedger => {
        let html = `
            <div class="package-input-group" style="margin-bottom: 20px;">
                <div class="agg-pkg-header">
                    <span>包装: ${pkgLedger.packageKey}</span>
                </div>`;

        html += (pkgLedger.masters || []).map(master => {
            if (!master) return '';
            
            const janUnitName = (master.janUnitCode === 0 || !unitMap[master.janUnitCode]) ? master.yjUnitName : (unitMap[master.janUnitCode] || master.yjUnitName);
            
            const userInputArea = `
                <div class="user-input-area" style="font-size: 14px; padding: 10px; background-color: #fffbdd;">
                    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
                        <label style="font-weight: bold; min-width: 250px;">① 本日の実在庫数量（予製除く）:</label>
                        <input type="number" class="physical-stock-input" data-product-code="${master.productCode}" step="any">
                        <span>(${janUnitName})</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 8px; font-weight: bold; color: #dc3545;">
                        <label style="min-width: 250px;">② 前日在庫(逆算値):</label>
                        <span class="calculated-previous-day-stock" data-product-code="${master.productCode}">0.00</span>
                        <span>(${janUnitName})</span>
                        <span style="font-size: 11px; color: #555; margin-left: 10px;">(この数値が棚卸データとして登録されます)</span>
                    </div>
                </div>`;
            
            const relevantDeadStock = (lastLoadedDataCache.deadStockDetails || []).filter(ds => ds.productCode === master.productCode);
            let finalInputTbodyHtml;
            if (relevantDeadStock.length > 0) {
                finalInputTbodyHtml = relevantDeadStock.map((rec, index) => createFinalInputRow(master, rec, index === 0)).join('');
            } else {
                finalInputTbodyHtml = createFinalInputRow(master, null, true);
            }

            const finalInputTable = renderStandardTable(`final-table-${master.productCode}`, [], false, 
                `<tbody class="final-input-tbody" data-product-code="${master.productCode}">${finalInputTbodyHtml}</tbody>`);
            
            // ▼▼▼【ここから修正】▼▼▼
            // <details>と<summary>タグを削除し、常に表示されるようにする
            return `<div class="product-input-group" style="padding-left: 20px; margin-top: 10px;">
                        ${userInputArea}
                        <div style="margin-top: 10px;">
                            <p style="font-size: 12px; font-weight: bold; margin-bottom: 4px;">ロット・期限を個別入力</p>
                            ${finalInputTable}
                        </div>
                    </div>`;
            // ▲▲▲【修正ここまで】▲▲▲
        }).join('');

        html += `</div>`;
        return html;
    }).join('');

    return `<div class="input-section" style="margin-top: 30px;"><h3 class="view-subtitle">2. 棚卸入力</h3>
        <div class="inventory-input-area" style="padding: 10px; border: 1px solid #ccc; background-color: #f8f9fa; margin-bottom: 15px;">
            <label for="inventory-date" style="font-weight: bold;">棚卸日:</label>
            <input type="date" id="inventory-date"></div>
        ${packageGroupsHtml}</div>`;
}


function createFinalInputRow(master, deadStockRecord = null, isPrimary = false) {
    const actionButtons = isPrimary ?
`
        <button class="btn add-deadstock-row-btn" data-product-code="${master.productCode}">＋</button>
        <button class="btn register-inventory-btn">登録</button>
    ` : `<button class="btn delete-deadstock-row-btn">－</button>`;

    const quantityInputClass = isPrimary ? 'final-inventory-input' : 'lot-quantity-input';
    const quantityPlaceholder = isPrimary ? '目安をここに転記' : 'ロット数量';
    const quantity = deadStockRecord ? deadStockRecord.stockQuantityJan : '';
    const expiry = deadStockRecord ? deadStockRecord.expiryDate : '';
    const lot = deadStockRecord ? deadStockRecord.lotNumber : '';
    const topRow = `<tr class="inventory-row"><td rowspan="2"><div style="display: flex; flex-direction: column; gap: 4px;">${actionButtons}</div></td>
        <td>(棚卸日)</td><td class="yj-jan-code">${master.yjCode}</td><td class="left" colspan="2">${master.productName}</td>
        <td></td><td></td><td class="right">${master.yjPackUnitQty || ''}</td><td>${master.yjUnitName || ''}</td>
        <td></td><td></td><td><input type="text" class="expiry-input" placeholder="YYYYMM" value="${expiry}"></td><td></td><td></td></tr>`;
    const bottomRow = `<tr class="inventory-row"><td>棚卸</td><td class="yj-jan-code">${master.productCode}</td>
        <td>${master.formattedPackageSpec || ''}</td><td>${master.makerName || ''}</td><td>${master.usageClassification || ''}</td>
        <td><input type="number" class="${quantityInputClass}" data-product-code="${master.productCode}" placeholder="${quantityPlaceholder}" value="${quantity}"></td>
        <td class="right">${master.janPackUnitQty || ''}</td><td>${master.janUnitName || ''}</td>
        <td></td><td></td><td><input type="text" class="lot-input" placeholder="ロット番号" value="${lot}"></td><td></td><td></td></tr>`;
    return topRow + bottomRow;
}

function renderStandardTable(id, records, addCheckbox = false, customBody = null) {
    const header = `<thead>
        <tr><th rowspan="2">－</th><th>日付</th><th>YJ</th><th colspan="2">製品名</th><th>個数</th><th>YJ数量</th><th>YJ包装数</th><th>YJ単位</th><th>単価</th><th>税額</th><th>期限</th><th>得意先</th><th>行</th></tr>
           <tr><th>種別</th><th>JAN</th><th>包装</th><th>メーカー</th><th>剤型</th><th>JAN数量</th><th>JAN包装数</th><th>JAN単位</th><th>金額</th><th>税率</th><th>ロット</th><th>伝票番号</th><th>MA</th></tr></thead>`;
    let bodyHtml = customBody ? customBody : `<tbody>${(!records || records.length === 0) ?
 '<tr><td colspan="14">対象データがありません。</td></tr>' : records.map(rec => {
        let clientDisplayHtml = '';
        if (rec.flag === 1 || rec.flag === 2) {
            clientDisplayHtml = wholesalerMap.get(rec.clientCode) || rec.clientCode || '';
        } else {
            clientDisplayHtml = rec.clientCode || '';
        }

        const top = `<tr><td rowspan="2">${addCheckbox ? `<input type="checkbox" class="precomp-active-check" data-quantity="${rec.yjQuantity}" data-product-code="${rec.janCode}">` : ''}</td>
            <td>${rec.transactionDate || ''}</td><td class="yj-jan-code">${rec.yjCode || ''}</td><td class="left" colspan="2">${rec.productName || ''}</td>
            <td class="right">${rec.datQuantity?.toFixed(2) || ''}</td><td class="right">${rec.yjQuantity?.toFixed(2) || ''}</td><td class="right">${rec.yjPackUnitQty || ''}</td><td>${rec.yjUnitName || ''}</td>
            <td class="right">${rec.unitPrice?.toFixed(4) || ''}</td><td class="right">${rec.taxAmount?.toFixed(2) || ''}</td><td>${rec.expiryDate || ''}</td><td class="left">${clientDisplayHtml}</td><td class="right">${rec.lineNumber || ''}</td></tr>`;
    
        const bottom = `<tr><td>${transactionTypeMap[rec.flag] || rec.flag}</td><td class="yj-jan-code">${rec.janCode || ''}</td><td>${rec.packageSpec || ''}</td><td>${rec.makerName || ''}</td>
            <td>${rec.usageClassification || ''}</td><td class="right">${rec.janQuantity?.toFixed(2) || ''}</td><td class="right">${rec.janPackUnitQty || ''}</td><td>${rec.janUnitName || ''}</td>
            <td class="right">${rec.subtotal?.toFixed(2) || ''}</td><td class="right">${rec.taxRate != null ? (rec.taxRate * 100).toFixed(0) + "%" : ""}</td><td>${rec.lotNumber || ''}</td><td class="left">${rec.receiptNumber || ''}</td><td class="left">${rec.processFlagMA || ''}</td></tr>`;
    return top + bottom;
    }).join('')}</tbody>`;
    return `<table class="data-table" id="${id}">${header}${bodyHtml}</table>`;
}

function handleInputChanges(e) {
    const targetClassList = e.target.classList;

    // ▼▼▼ [修正点] 新しい入力欄のイベントハンドラを追加 ▼▼▼
    if (targetClassList.contains('physical-stock-input') || targetClassList.contains('precomp-active-check')) {
        reverseCalculateStock();
    }
    // ▲▲▲ 修正ここまで ▲▲▲

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

// ▼▼▼ [修正点] recalculateTotals を reverseCalculateStock に改名・ロジック変更 ▼▼▼
function reverseCalculateStock() {
    const todayStr = getLocalDateString().replace(/-/g, '');

    // 1. 有効な予製数をJAN単位で集計
    const precompTotalsByProduct = {};
    document.querySelectorAll('.precomp-active-check:checked').forEach(cb => {
        const productCode = cb.dataset.productCode;
        const master = findMaster(productCode);
        if (!master) return;
        
        const yjQuantity = parseFloat(cb.dataset.quantity) || 0;
        let janQuantity = 0;
        if (master.janPackInnerQty > 0) {
            janQuantity = yjQuantity / master.janPackInnerQty;
        }
        precompTotalsByProduct[productCode] = (precompTotalsByProduct[productCode] || 0) + janQuantity;
    });

    // 2. 本日分の取引変動数をJAN単位で集計
    const todayNetChangeByProduct = {};
    if (lastLoadedDataCache && lastLoadedDataCache.transactionLedger) {
        lastLoadedDataCache.transactionLedger.forEach(yjGroup => {
            yjGroup.packageLedgers.forEach(pkg => {
                pkg.transactions.forEach(tx => {
                    if (tx.transactionDate === todayStr && tx.flag !== 0) { // 棚卸(flag=0)は除外
                        let janQty = tx.janQuantity || 0;
                        // 処方(flag:3)のようにYJ数量しか記録されていない場合、JAN数量を逆算する
                        if (janQty === 0 && tx.yjQuantity && tx.janPackInnerQty > 0) {
                            janQty = tx.yjQuantity / tx.janPackInnerQty;
                        }
                        const signedJanQty = janQty * (tx.flag === 1 || tx.flag === 11 || tx.flag === 4 ? 1 : -1);
                        todayNetChangeByProduct[tx.janCode] = (todayNetChangeByProduct[tx.janCode] || 0) + signedJanQty;
                    }
                });
            });
        });
    }

    // 3. 各製品の入力欄に対して逆算を実行
    document.querySelectorAll('.physical-stock-input').forEach(input => {
        const productCode = input.dataset.productCode;
        const physicalStockToday = parseFloat(input.value) || 0;
        const precompStock = precompTotalsByProduct[productCode] || 0;
        const netChangeToday = todayNetChangeByProduct[productCode] || 0;

        const totalStockToday = physicalStockToday + precompStock;
        const calculatedPreviousDayStock = totalStockToday - netChangeToday;
        
        const displaySpan = document.querySelector(`.calculated-previous-day-stock[data-product-code="${productCode}"]`);
        if(displaySpan) displaySpan.textContent = calculatedPreviousDayStock.toFixed(2);
        
        const finalInput = document.querySelector(`.final-inventory-input[data-product-code="${productCode}"]`);
        if(finalInput) {
            finalInput.value = calculatedPreviousDayStock.toFixed(2);
            updateFinalInventoryTotal(productCode);
        }
    });
}
// ▲▲▲ 修正ここまで ▲▲▲

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
        // ▼▼▼ [修正点] サーバーに送る値をJAN単位の合計数量にする ▼▼▼
        inventoryData[productCode] = totalInputQuantity;
        // ▲▲▲ 修正ここまで ▲▲▲
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