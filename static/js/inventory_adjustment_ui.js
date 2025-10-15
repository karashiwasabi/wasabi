// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/inventory_adjustment_ui.js

import { transactionTypeMap } from './common_table.js';
import { wholesalerMap, clientMap } from './master_data.js';
import { getLocalDateString } from './utils.js';

// このファイル内で共有する変数
let unitMap = {};
let lastLoadedDataCache = null; // UI描画関数内でキャッシュを参照できるようにする

/**
 * 単位マップを外部から設定する
 * @param {object} map 
 */
export function setUnitMap(map) {
    unitMap = map;
}

/**
 * 標準的な取引履歴テーブルのHTMLを生成する
 * @param {string} id - テーブルのID
 * @param {Array} records - 表示するレコードの配列
 * @param {boolean} addCheckbox - 予製用のチェックボックスを追加するか
 * @param {string|null} customBody - tbodyの内容をカスタムHTMLで置き換える場合
 * @returns {string} - テーブル全体のHTML文字列
 */
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

/**
 * 「1. 全体サマリー」セクションのHTMLを生成する
 * @param {object} yjGroup - 製品グループデータ
 * @param {number} yesterdaysTotal - 前日在庫合計
 * @returns {string} HTML文字列
 */
function generateSummaryLedgerHtml(yjGroup, yesterdaysTotal) {
    const endDate = getLocalDateString();
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - 30);
    const startDateStr = startDate.toISOString().slice(0, 10);

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

/**
 * 「予製払出明細」セクションのHTMLを生成する
 * @param {Array} precompDetails - 予製詳細データ
 * @returns {string} HTML文字列
 */
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

/**
 * 「2. 棚卸入力」セクションの入力行のHTMLを生成する
 * @param {object} master - 製品マスターデータ
 * @param {object|null} deadStockRecord - 既存のロット・期限情報
 * @param {boolean} isPrimary - 最初の行（目安転記欄）かどうか
 * @returns {string} HTML文字列
 */
export function createFinalInputRow(master, deadStockRecord = null, isPrimary = false) {
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

/**
 * 「2. 棚卸入力」セクション全体のHTMLを生成する
 * @param {Array} packageLedgers - 包装台帳データの配列
 * @param {string} yjUnitName - YJ単位名
 * @param {object} cache - lastLoadedDataCache
 * @param {number} yesterdaysTotal - 前日の理論在庫合計
 * @returns {string} HTML文字列
 */
function generateInputSectionsHtml(packageLedgers, yjUnitName = '単位', cache, yesterdaysTotal) {
    const packageGroupsHtml = (packageLedgers || []).map(pkgLedger => {
        // ▼▼▼【修正】前日の包装別在庫を取得 ▼▼▼
        let yesterdaysPkgStock = 0;
        if(cache.yesterdaysStock && cache.yesterdaysStock.packageLedgers){
            const prevPkg = cache.yesterdaysStock.packageLedgers.find(p => p.packageKey === pkgLedger.packageKey);
            if(prevPkg) {
                yesterdaysPkgStock = prevPkg.endingBalance || 0;
            }
        }
        // ▲▲▲【修正ここまで】▲▲▲

        let html = `
        <div class="package-input-group" style="margin-bottom: 20px;">
            <div class="agg-pkg-header">
                <span>包装: ${pkgLedger.packageKey}</span>
            </div>`;
        html += (pkgLedger.masters || []).map(master => {
            if (!master) return '';
            
            const janUnitName = (master.janUnitCode === 0 || !unitMap[master.janUnitCode]) ? master.yjUnitName : (unitMap[master.janUnitCode] || master.yjUnitName);
            
            // ▼▼▼【ここから修正】ご指示の表示を追加 ▼▼▼
            const userInputArea = `
            <div class="user-input-area" style="font-size: 14px; padding: 10px; background-color: #fffbdd;">
                    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px; justify-content: space-between;">
                        <div style="display: flex; align-items: center; gap: 8px;">
                            <label style="font-weight: bold; min-width: 250px;">① 本日の実在庫数量（予製除く）:</label>
                            <input type="number" class="physical-stock-input" data-product-code="${master.productCode}" step="any">
                            <span>(${janUnitName})</span>
                        </div>
                        <span style="font-size: 12px; color: #555;">本日理論在庫(包装計): ${(pkgLedger.endingBalance || 0).toFixed(2)} ${yjUnitName}</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 8px; font-weight: bold; color: #dc3545; justify-content: space-between;">
                        <div style="display: flex; align-items: center; gap: 8px;">
                            <label style="min-width: 250px;">② 前日在庫(逆算値):</label>
                            <span class="calculated-previous-day-stock" data-product-code="${master.productCode}">0.00</span>
                            <span>(${janUnitName})</span>
                            <span style="font-size: 11px; color: #555; margin-left: 10px;">(この数値が棚卸データとして登録されます)</span>
                        </div>
                        <span style="font-size: 12px; color: #555;">前日理論在庫(包装計): ${yesterdaysPkgStock.toFixed(2)} ${yjUnitName}</span>
                    </div>
                </div>`;
            // ▲▲▲【修正ここまで】▲▲▲
            
            const relevantDeadStock = (cache.deadStockDetails || []).filter(ds => ds.productCode === master.productCode);
            let finalInputTbodyHtml;
            if (relevantDeadStock.length > 0) {
                finalInputTbodyHtml = relevantDeadStock.map((rec, index) => createFinalInputRow(master, rec, index === 0)).join('');
            } else {
                finalInputTbodyHtml = createFinalInputRow(master, null, true);
            }
            const finalInputTable = renderStandardTable(`final-table-${master.productCode}`, [], false, 
                `<tbody class="final-input-tbody" data-product-code="${master.productCode}">${finalInputTbodyHtml}</tbody>`);
            
            return `<div class="product-input-group" style="padding-left: 20px; margin-top: 10px;">
                        ${userInputArea}
                        <div style="margin-top: 10px;">
                            <p style="font-size: 12px; font-weight: bold; margin-bottom: 4px;">ロット・期限を個別入力</p>
                            ${finalInputTable}
                        </div>
                    </div>`;
        }).join('');

        html += `</div>`;
        return html;
    }).join('');
    
    return `<div class="input-section" style="margin-top: 30px;">
        <h3 class="view-subtitle">2. 棚卸入力</h3>
        <div class="inventory-input-area" style="padding: 10px; border: 1px solid #ccc; background-color: #f8f9fa; margin-bottom: 15px;">
            <div style="display: flex; gap: 20px; align-items: flex-end;">
                <div class="field-group">
                    <label for="inventory-date" style="font-weight: bold;">棚卸日:</label>
                    <input type="date" id="inventory-date">
                </div>
                <form id="adjustment-barcode-form" style="flex-grow: 1;">
                    <div class="field-group">
                        <label for="adjustment-barcode-input" style="font-weight: bold;">バーコードでロット・期限入力</label>
                        <input type="text" id="adjustment-barcode-input" inputmode="latin" placeholder="GS1-128バーコードをスキャンしてEnter" style="ime-mode: disabled; font-size: 1.1em;">
                    </div>
                </form>
            </div>
        </div>
        ${packageGroupsHtml}
    </div>`;
}

/**
 * 「3. 参考」セクションのHTMLを生成する
 * @param {Array} deadStockRecords - 既存のロット・期限情報
 * @param {object} cache - lastLoadedDataCache
 * @returns {string} HTML文字列
 */
function generateDeadStockReferenceHtml(deadStockRecords, cache) {
    if (!deadStockRecords || deadStockRecords.length === 0) {
        return '';
    }

    const getProductName = (productCode) => {
        if (!cache || !cache.transactionLedger) return productCode;
        for (const yjGroup of cache.transactionLedger) {
            for (const pkg of yjGroup.packageLedgers) {
                const master = (pkg.masters || []).find(m => m.productCode === productCode);
                if (master) return master.productName;
            }
        }
        return productCode;
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

/**
 * 棚卸調整画面の全HTMLを生成する
 * @param {object} data - サーバーから取得した全データ
 * @param {object} cache - 読み込み済みデータキャッシュ
 * @returns {string} HTML文字列
 */
export function generateFullHtml(data, cache) {
    lastLoadedDataCache = cache;
    if (!data.transactionLedger || data.transactionLedger.length === 0) {
        return '<p>対象の製品データが見つかりませんでした。</p>';
    }
    const yjGroup = data.transactionLedger[0];
    const productName = yjGroup.productName;
    const yesterdaysTotal = data.yesterdaysStock ? (data.yesterdaysStock.endingBalance || 0) : 0;
    const summaryLedgerHtml = generateSummaryLedgerHtml(yjGroup, yesterdaysTotal);
    const summaryPrecompHtml = generateSummaryPrecompHtml(data.precompDetails);
    const inputSectionsHtml = generateInputSectionsHtml(yjGroup.packageLedgers, yjGroup.yjUnitName, cache, yesterdaysTotal);
    const deadStockReferenceHtml = generateDeadStockReferenceHtml(data.deadStockDetails, cache);
    return `<h2 style="text-align: center; margin-bottom: 20px;">【棚卸調整】 ${productName} (YJ: ${yjGroup.yjCode})</h2>
        ${summaryLedgerHtml}
        ${summaryPrecompHtml}
        ${inputSectionsHtml}
        ${deadStockReferenceHtml}`;
}