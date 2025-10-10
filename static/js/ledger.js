// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\ledger.js

import { showModal } from './inout_modal.js';
import { hiraganaToKatakana } from './utils.js';
import { wholesalerMap, clientMap } from './master_data.js';
import { transactionTypeMap } from './common_table.js';

let view, selectProductBtn, outputContainer, selectedProductDisplay, printBtn;
let lastLoadedData = null; // APIから取得したデータをキャッシュする

/**
 * トランザクションレコードから符号付きのYJ数量を計算するヘルパー関数
 * @param {object} record - TransactionRecordオブジェクト
 * @returns {number} - 符号付きのYJ数量
 */
function getSignedYjQty(record) {
    const flag = record.flag;
    const qty = record.yjQuantity || 0;
    switch (flag) {
        case 1: case 4: case 11: // 入庫系
            return qty;
        case 2: case 3: case 5: case 12: // 出庫系
            return -qty;
        default: // 棚卸など
            return 0;
    }
}


// ▼▼▼【ここから修正】▼▼▼
/**
 * 最新の棚卸を基点に、在庫を再計算し、日付昇順のリストを返す
 * @param {Array} transactions - サーバーから受け取った昇順の取引リスト
 * @returns {Array} - 在庫を再計算した、昇順の取引リスト
 */
function recalculateBalancesFromLatestInventory(transactions) {
    if (!transactions || transactions.length === 0) {
        return [];
    }

    // サーバーから来た昇順のデータを深いコピー
    let recalculatedTxs = JSON.parse(JSON.stringify(transactions));

    // 1. 最新の棚卸レコードのインデックスを探す
    let latestInventoryIndex = -1;
    for (let i = recalculatedTxs.length - 1; i >= 0; i--) {
        if (recalculatedTxs[i].flag === 0) {
            latestInventoryIndex = i;
            break;
        }
    }

    if (latestInventoryIndex !== -1) {
        // ---【A】期間内に最新の棚卸がある場合 ---

        // a. 棚卸レコード自体の在庫を、その棚卸数量で確定させる
        recalculatedTxs[latestInventoryIndex].runningBalance = recalculatedTxs[latestInventoryIndex].yjQuantity;

        // b. 棚卸日より未来の在庫を、棚卸在庫を基点に再計算（昇順）
        for (let i = latestInventoryIndex + 1; i < recalculatedTxs.length; i++) {
            recalculatedTxs[i].runningBalance = recalculatedTxs[i - 1].runningBalance + getSignedYjQty(recalculatedTxs[i]);
        }

        // c. 棚卸日より過去の在庫を、棚卸在庫を基点に再計算（逆算）
        for (let i = latestInventoryIndex - 1; i >= 0; i--) {
            recalculatedTxs[i].runningBalance = recalculatedTxs[i + 1].runningBalance - getSignedYjQty(recalculatedTxs[i + 1]);
        }

    } else {
        // ---【B】期間内に棚卸がない場合 ---
        
        // サーバーが計算した最終在庫（昇順での最後のレコード）を基点に、全て過去に遡って逆算する
        if (recalculatedTxs.length > 0) {
            for (let i = recalculatedTxs.length - 2; i >= 0; i--) {
                recalculatedTxs[i].runningBalance = recalculatedTxs[i + 1].runningBalance - getSignedYjQty(recalculatedTxs[i + 1]);
            }
        }
    }

    // 昇順のまま配列を返す
    return recalculatedTxs;
}

/**
 * 予製情報テーブルを含む、台帳ビュー全体のHTMLを生成して描画する
 */
function renderLedgerView() {
    if (!lastLoadedData) {
        outputContainer.innerHTML = "<p>データがありません。</p>";
        return;
    }

    // 新しいロジックで在庫を再計算し、日付昇順のリストを取得
    const ascendingTransactions = recalculateBalancesFromLatestInventory(lastLoadedData.ledgerTransactions);

    const ledgerHtml = renderLedgerTable(ascendingTransactions); // 昇順リストでテーブル描画
    const precompHtml = renderPrecompDetails(lastLoadedData.precompDetails);
    
    // 最終在庫サマリー（最も新しい取引の在庫）
    const finalTheoreticalStock = ascendingTransactions.length > 0
        ? ascendingTransactions[ascendingTransactions.length - 1].runningBalance // 昇順なので最後の要素
        : 0;

    const summaryHtml = `
        <div style="text-align: right; margin-top: 20px; padding-top: 10px; border-top: 2px solid #333; font-weight: bold;">
            <span>最終理論在庫: <span id="final-theoretical-stock">${finalTheoreticalStock.toFixed(2)}</span></span> | 
            <span>チェック済み予製合計: <span id="total-precomp-stock">0.00</span></span> | 
            <span style="color: blue;">最終実在庫: <span id="final-real-stock">${finalTheoreticalStock.toFixed(2)}</span></span>
        </div>
    `;

    outputContainer.innerHTML = ledgerHtml + precompHtml + summaryHtml;
    updateRealStock(); // 初回描画時に実在庫を計算
}
// ▲▲▲【修正ここまで】▲▲▲

/**
 * 台帳テーブルのHTMLを生成する
 * @param {Array} records - 表示する台帳データの配列 (LedgerTransaction)
 */
function renderLedgerTable(records) {
    if (!records || records.length === 0) {
        return "<h4>取引履歴 (過去30日)</h4><p>対象期間の取引データがありませんでした。</p>";
    }

    const tableHeader = `
        <thead>
            <tr>
                <th>日付</th>
                <th>種別</th>
                <th>入庫 (YJ)</th>
                <th>出庫 (YJ)</th>
                <th>在庫(理論)</th>
                <th style="color: blue;">在庫(実)</th>
                <th>卸/患者</th>
                <th>ロット</th>
                <th>期限</th>
            </tr>
        </thead>
    `;

    const tableBody = records.map(rec => {
        const signedQty = getSignedYjQty(rec);
        const receipt = signedQty > 0 ? signedQty.toFixed(2) : '';
        const dispense = signedQty < 0 ? (-signedQty).toFixed(2) : '';
        
        let partyName = '';
        if (rec.flag === 1 || rec.flag === 2) { // 納品・返品
             partyName = wholesalerMap.get(rec.clientCode) || rec.clientCode || '';
        } else if (rec.flag === 3 || rec.flag === 5) { // 処方・予製
             partyName = clientMap.get(rec.clientCode) || rec.clientCode || '';
        }

        return `
        <tr class="ledger-row" data-theoretical-stock="${rec.runningBalance}">
            <td>${rec.transactionDate || ''}</td>
            <td>${transactionTypeMap[rec.flag] || ''}</td>
            <td class="right">${receipt}</td>
            <td class="right">${dispense}</td>
            <td class="right">${(rec.runningBalance ?? 0).toFixed(2)}</td>
            <td class="right real-stock-cell" style="font-weight: bold; color: blue;">${(rec.runningBalance ?? 0).toFixed(2)}</td>
            <td class="left">${partyName}</td>
            <td class="left">${rec.lotNumber || ''}</td>
            <td>${rec.expiryDate || ''}</td>
        </tr>
    `}).join('');

    return `<h3 class="view-subtitle">取引履歴 (過去30日)</h3><table class="data-table">${tableHeader}<tbody>${tableBody}</tbody></table>`;
}

/**
 * 予製情報テーブルのHTMLを生成する
 * @param {Array} records - 表示する予製データの配列 (TransactionRecord)
 */
function renderPrecompDetails(records) {
    if (!records || records.length === 0) {
        return '<div style="margin-top: 20px;"><h3 class="view-subtitle">関連する予製情報</h3><p>この製品に紐づく予製情報はありません。</p></div>';
    }

    const tableHeader = `
        <thead>
            <tr>
                <th style="width: 5%;"><input type="checkbox" class="precomp-check-all" checked></th>
                <th style="width: 25%;">患者番号</th>
                <th style="width: 40%;">製品名</th>
                <th style="width: 15%;">予製数量 (YJ)</th>
                <th style="width: 15%;">包装</th>
            </tr>
        </thead>
    `;
    const tableBody = records.map(rec => `
        <tr>
            <td class="center"><input type="checkbox" class="precomp-check" data-quantity="${rec.yjQuantity}" checked></td>
            <td class="left">${clientMap.get(rec.clientCode) || rec.clientCode}</td>
            <td class="left">${rec.productName}</td>
            <td class="right">${rec.yjQuantity.toFixed(2)}</td>
            <td class="left">${rec.packageSpec}</td>
        </tr>
    `).join('');

    return `<div style="margin-top: 20px;">
                <h3 class="view-subtitle">関連する予製情報</h3>
                <p style="font-size: 11px; margin-bottom: 5px;">チェックを入れた予製は実在庫から引かれます。</p>
                <table class="data-table">${tableHeader}<tbody>${tableBody}</tbody></table>
            </div>`;
}

/**
 * 予製チェックボックスの変更に応じて実在庫を再計算・描画する
 */
function updateRealStock() {
    let precompTotal = 0;
    outputContainer.querySelectorAll('.precomp-check:checked').forEach(checkbox => {
        precompTotal += parseFloat(checkbox.dataset.quantity || 0);
    });

    // 予製合計サマリーを更新
    const totalPrecompEl = document.getElementById('total-precomp-stock');
    if (totalPrecompEl) totalPrecompEl.textContent = precompTotal.toFixed(2);

    // 各行の実在庫を更新
    const ledgerRows = outputContainer.querySelectorAll('tr.ledger-row');
    ledgerRows.forEach(row => {
        const theoreticalStock = parseFloat(row.dataset.theoreticalStock);
        const realStock = theoreticalStock - precompTotal;
        row.querySelector('.real-stock-cell').textContent = realStock.toFixed(2);
    });

    // 最終実在庫サマリーを更新
    const finalRealStockEl = document.getElementById('final-real-stock');
    if (finalRealStockEl) {
        const finalTheoreticalStockEl = document.getElementById('final-theoretical-stock');
        const finalTheoreticalStock = finalTheoreticalStockEl ? parseFloat(finalTheoreticalStockEl.textContent) : 0;
        finalRealStockEl.textContent = (finalTheoreticalStock - precompTotal).toFixed(2);
    }
}


/**
 * 指定された製品コードの台帳データをサーバーから取得して描画する
 * @param {string} productCode - 対象の製品JANコード
 */
async function loadLedgerForProduct(productCode) {
    outputContainer.innerHTML = '<p>台帳データを読み込み中...</p>';
    window.showLoading();
    try {
        const res = await fetch(`/api/ledger/product/${productCode}`);
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || '台帳データの取得に失敗しました。');
        }
        lastLoadedData = await res.json();
        
        // 通常のビューを描画する
        renderLedgerView();

    } catch (err) {
        outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

/**
 * 「品目を選択...」ボタンがクリックされたときの処理
 */
async function onSelectProductClick() {
    const drugTypeCheckboxes = document.querySelectorAll('input[name="ledgerDrugType"]:checked');
    const selectedDrugTypes = Array.from(drugTypeCheckboxes).map(cb => cb.value).join(',');

    const params = new URLSearchParams({
        deadStockOnly: false,
        drugTypes: selectedDrugTypes
    });
    const apiUrl = `/api/products/search_filtered?${params.toString()}`;

    window.showLoading();
    try {
        const res = await fetch(apiUrl);
        if (!res.ok) throw new Error('品目リストの取得に失敗しました。');
        const products = await res.json();
        window.hideLoading();
        
        showModal(view, (selectedProduct) => {
            selectedProductDisplay.textContent = `${selectedProduct.productName} (${selectedProduct.yjCode})`;
            loadLedgerForProduct(selectedProduct.productCode);
        }, { 
            initialResults: products, 
            searchApi: apiUrl 
        });
    } catch (err) {
        window.hideLoading();
        window.showNotification(err.message, 'error');
    }
}


/**
 * 管理台帳ビューの初期化
 */
export function initLedgerView() {
    view = document.getElementById('ledger-view');
    if (!view) return;

    selectProductBtn = document.getElementById('ledger-select-product-btn');
    outputContainer = document.getElementById('ledger-output-container');
    selectedProductDisplay = document.getElementById('ledger-selected-product');
    printBtn = document.getElementById('print-ledger-btn');

    selectProductBtn.addEventListener('click', onSelectProductClick);
    
    printBtn.addEventListener('click', () => {
        if (outputContainer.querySelector('table')) {
            view.classList.add('print-this-view');
            window.print();
        } else {
            window.showNotification('印刷するデータがありません。', 'error');
        }
    });
    
    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });

    // イベント委譲で予製チェックボックスの変更をハンドル
    outputContainer.addEventListener('change', (e) => {
        if (e.target.classList.contains('precomp-check')) {
            updateRealStock();
        } else if (e.target.classList.contains('precomp-check-all')) {
            const isChecked = e.target.checked;
            outputContainer.querySelectorAll('.precomp-check').forEach(chk => chk.checked = isChecked);
            updateRealStock();
        }
    });
}