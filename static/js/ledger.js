// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\ledger.js

import { showModal } from './inout_modal.js';
import { hiraganaToKatakana } from './utils.js';
import { wholesalerMap } from './master_data.js';

let view, selectProductBtn, outputContainer, selectedProductDisplay, printBtn;

/**
 * サーバーから受け取ったデータで台帳テーブルを描画する
 * @param {Array} records - 表示する台帳データの配列
 */
function renderLedgerTable(records) {
    if (!records || records.length === 0) {
        outputContainer.innerHTML = "<p>対象期間の取引データがありませんでした。</p>";
        return;
    }

    // Dateプロパティが存在しないレコードがあってもエラーにならないように、安全なソート処理を行う
    records.sort((a, b) => (a.Date || '').localeCompare(b.Date || ''));

    const tableHeader = `
        <thead>
            <tr>
                <th>日付</th>
                <th>入庫 (YJ単位)</th>
                <th>出庫 (YJ単位)</th>
                <th>在庫 (YJ単位)</th>
                <th>卸</th>
                <th>ロット番号</th>
                <th>使用期限</th>
            </tr>
        </thead>
    `;

    const tableBody = records.map(rec => {
        // 卸コードを卸名に変換する
        const wholesalerName = wholesalerMap.get(rec.Wholesaler) || rec.Wholesaler || '';

        return `
        <tr>
            <td>${rec.Date || ''}</td>
            <td class="right">${rec.Receipt > 0 ? rec.Receipt.toFixed(2) : ''}</td>
            <td class="right">${rec.Dispense > 0 ? rec.Dispense.toFixed(2) : ''}</td>
            <td class="right">${(rec.Stock ?? 0).toFixed(2)}</td>
            <td class="left">${wholesalerName}</td>
            <td class="left">${rec.LotNumber || ''}</td>
            <td>${rec.ExpiryDate || ''}</td>
        </tr>
    `}).join('');

    outputContainer.innerHTML = `<table class="data-table">${tableHeader}<tbody>${tableBody}</tbody></table>`;
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
        const data = await res.json();
        renderLedgerTable(data);
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
    selectedProductDisplay.textContent = selectedProduct.productName;
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
}