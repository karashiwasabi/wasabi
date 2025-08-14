// C:\Dev\WASABI\static\js\inout_details_table.js

import { showModal } from './inout_modal.js';
import { transactionTypeMap, createUploadTableHTML } from './common_table.js';

let tableBody, addRowBtn, tableContainer;

// ▼▼▼ [修正点] 「個数」をINPUTから表示用のセルに変更 ▼▼▼
function createInoutRowsHTML(record = {}) {
    const rowId = record.lineNumber || `new-${Date.now()}`;
    const janQuantity = record.janQuantity ?? 1;
    const datQuantity = record.datQuantity ?? 1; // Default value, will be recalculated
    const nhiPrice = record.nhiPrice || 0;
    const janPackInnerQty = record.janPackInnerQty || 0;
    const yjQuantity = janQuantity * janPackInnerQty;
    const subtotal = yjQuantity * nhiPrice;
    const transactionType = record.flag ? (transactionTypeMap[record.flag] || '') : '';

    const upperRow = `
        <tr data-row-id="${rowId}">
            <td rowspan="2" class="center"><button class="delete-row-btn btn">削除</button></td>
            <td>${record.transactionDate || ''}</td>
            <td class="yj-jan-code display-yj-code">${record.yjCode || ''}</td>
            <td colspan="2" class="product-name-cell left" style="cursor: pointer; text-decoration: underline; color: blue;">${record.productName || 'ここをクリックして製品を検索'}</td>
            <td class="right display-dat-quantity">${datQuantity.toFixed(2)}</td>
            <td class="right display-yj-quantity">${yjQuantity.toFixed(2)}</td>
            <td class="right display-yj-pack-unit-qty">${record.yjPackUnitQty || ''}</td>
            <td class="display-yj-unit-name">${record.yjUnitName || ''}</td>
            <td class="right display-unit-price">${nhiPrice.toFixed(4)}</td>
            <td class="right">${record.taxAmount || ''}</td>
            <td><input type="text" name="expiryDate" value="${record.expiryDate || ''}" placeholder="YYYYMM"></td>
            <td class="left">${record.clientCode || ''}</td>
            <td class="right">${record.lineNumber || ''}</td>
        </tr>`;

    const lowerRow = `
        <tr data-row-id-lower="${rowId}">
            <td>${transactionType}</td>
            <td class="yj-jan-code display-jan-code">${record.productCode || record.janCode || ''}</td>
            <td class="left display-package-spec">${record.formattedPackageSpec || record.packageSpec || ''}</td>
            <td class="left display-maker-name">${record.makerName || ''}</td>
            <td class="left display-usage-classification">${record.usageClassification || ''}</td>
            <td class="right"><input type="number" name="janQuantity" value="${janQuantity}" step="any"></td>
            <td class="right display-jan-pack-unit-qty">${record.janPackUnitQty || ''}</td>
            <td class="display-jan-unit-name">${record.janUnitName || ''}</td>
            <td class="right display-subtotal">${subtotal.toFixed(2)}</td>
            <td class="right">${record.taxRate != null ? (record.taxRate * 100).toFixed(0) + "%" : ""}</td>
            <td class="left"><input type="text" name="lotNumber" value="${record.lotNumber || ''}"></td>
            <td class="left">${record.receiptNumber || ''}</td>
            <td class="left">${record.processFlagMA || ''}</td>
        </tr>`;

    return upperRow + lowerRow;
}

export function populateDetailsTable(records) {
    if (!records || records.length === 0) {
        clearDetailsTable();
        return;
    }
    tableBody.innerHTML = records.map(createInoutRowsHTML).join('');
    
    tableBody.querySelectorAll('tr[data-row-id]').forEach((row, index) => {
        if (records[index]) {
            const masterData = { ...records[index] };
            delete masterData.id;
            delete masterData.runningBalance;
            row.dataset.product = JSON.stringify(masterData);
            recalculateRow(row); // Recalculate to set the initial datQuantity correctly
        }
    });
}

export function clearDetailsTable() {
    if (tableBody) {
        tableBody.innerHTML = `<tr><td colspan="14">ヘッダーで情報を選択後、「明細を追加」ボタンを押してください。</td></tr>`;
    }
}

export function getDetailsData() {
    const records = [];
    const rows = tableBody.querySelectorAll('tr[data-row-id]');
    rows.forEach(row => {
        const productDataString = row.dataset.product;
        if (!productDataString || productDataString === '{}') return;
        const productData = JSON.parse(productDataString);
        const lowerRow = row.nextElementSibling;
        
        const janQuantity = parseFloat(lowerRow.querySelector('input[name="janQuantity"]').value) || 0;
        let datQuantity = 0;
        if(productData.janPackUnitQty > 0) {
            datQuantity = janQuantity / productData.janPackUnitQty;
        }

        const record = {
            productCode: productData.productCode,
            productName: productData.productName,
            datQuantity: datQuantity,
            expiryDate: row.querySelector('input[name="expiryDate"]').value,
            janQuantity: janQuantity,
            lotNumber: lowerRow.querySelector('input[name="lotNumber"]').value,
        };
        records.push(record);
    });
    return records;
}

function recalculateRow(upperRow) {
    const productDataString = upperRow.dataset.product;
    if (!productDataString) return;
    const product = JSON.parse(productDataString);
    const lowerRow = upperRow.nextElementSibling;
    if (!lowerRow) return;
    
    const janQuantity = parseFloat(lowerRow.querySelector('[name="janQuantity"]').value) || 0;
    const nhiPrice = parseFloat(product.nhiPrice) || 0;
    const janPackInnerQty = parseFloat(product.janPackInnerQty) || 0;
    const janPackUnitQty = parseFloat(product.janPackUnitQty) || 0;

    let datQuantity = 0;
    if(janPackUnitQty > 0) {
        datQuantity = janQuantity / janPackUnitQty;
    }
    const yjQuantity = janQuantity * janPackInnerQty;
    const subtotal = yjQuantity * nhiPrice;

    upperRow.querySelector('.display-dat-quantity').textContent = datQuantity.toFixed(2);
    upperRow.querySelector('.display-yj-quantity').textContent = yjQuantity.toFixed(2);
    lowerRow.querySelector('.display-subtotal').textContent = subtotal.toFixed(2);
}
// ▲▲▲ 修正ここまで ▲▲▲

export function initDetailsTable() {
    tableContainer = document.getElementById('inout-details-container');
    addRowBtn = document.getElementById('addRowBtn');
    if (!tableContainer || !addRowBtn) return;
    
    tableContainer.innerHTML = createUploadTableHTML('inout-details-table');
    tableBody = document.querySelector('#inout-details-table tbody');
    
    addRowBtn.addEventListener('click', () => {
        if (tableBody.querySelector('td[colspan="14"]')) {
            tableBody.innerHTML = '';
        }
        tableBody.insertAdjacentHTML('beforeend', createInoutRowsHTML());
    });

    tableBody.addEventListener('click', (e) => {
        if (e.target.classList.contains('delete-row-btn')) {
            const upperRow = e.target.closest('tr');
            const lowerRow = upperRow.nextElementSibling;
            if(lowerRow) lowerRow.remove();
            upperRow.remove();
            if (tableBody.children.length === 0) {
                 clearDetailsTable();
            }
        }
        // ▼▼▼ [修正点] モーダル呼び出し時に、実行したい処理（コールバック）を直接渡す ▼▼▼
        if (e.target.classList.contains('product-name-cell')) {
            const activeRow = e.target.closest('tr');
            showModal(activeRow, (selectedProduct, targetRow) => {
                targetRow.dataset.product = JSON.stringify(selectedProduct);
                const lowerRow = targetRow.nextElementSibling;
                
                targetRow.querySelector('.display-yj-code').textContent = selectedProduct.yjCode;
                targetRow.querySelector('.product-name-cell').textContent = selectedProduct.productName;
                targetRow.querySelector('.display-yj-pack-unit-qty').textContent = selectedProduct.yjPackUnitQty || '';
                targetRow.querySelector('.display-yj-unit-name').textContent = selectedProduct.yjUnitName || '';
                targetRow.querySelector('.display-unit-price').textContent = (selectedProduct.nhiPrice || 0).toFixed(4);
                
                lowerRow.querySelector('.display-jan-code').textContent = selectedProduct.productCode;
                lowerRow.querySelector('.display-package-spec').textContent = selectedProduct.formattedPackageSpec || '';
                lowerRow.querySelector('.display-maker-name').textContent = selectedProduct.makerName;
                lowerRow.querySelector('.display-usage-classification').textContent = selectedProduct.usageClassification || '';
                lowerRow.querySelector('.display-jan-pack-unit-qty').textContent = selectedProduct.janPackUnitQty || '';
                lowerRow.querySelector('.display-jan-unit-name').textContent = selectedProduct.janUnitName || '';
                
                const quantityInput = lowerRow.querySelector('input[name="janQuantity"]');
                quantityInput.focus();
                quantityInput.select();
                recalculateRow(targetRow);
            });
        }
        // ▲▲▲ 修正ここまで ▲▲▲
    });

    tableBody.addEventListener('input', (e) => {
        const upperRow = e.target.closest('tr[data-row-id]') || e.target.closest('tr[data-row-id-lower]')?.previousElementSibling;
        if(upperRow) {
            recalculateRow(upperRow);
        }
    });
}