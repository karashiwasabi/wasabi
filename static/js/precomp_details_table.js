// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\precomp_details_table.js
import { showModal } from './inout_modal.js';
import { createUploadTableHTML } from './common_table.js';
import { clientMap } from './master_data.js';

let tableBody, addRowBtn, outputContainer;

function createRowHTML(rec = {}) {
    const isNew = !rec.janCode && !rec.productCode;
    const rowId = isNew ? `new-${Date.now()}` : rec.id;
    const productData = isNew ? {} : rec;
    const janQuantity = rec.janQuantity || 1;
    const yjQuantity = janQuantity * (rec.janPackInnerQty || 0);

    const topRow = `
        <tr class="data-row" data-row-id="${rowId}" data-product='${JSON.stringify(productData)}'>
            <td rowspan="2" class="center"><button class="delete-row-btn btn">削除</button><button class="insert-row-btn btn" style="margin-top: 4px;">挿入</button></td>
            <td>${rec.transactionDate || ''}</td>
            <td class="yj-jan-code">${rec.yjCode || ''}</td>
            <td colspan="2" class="product-name-cell left" style="cursor: pointer; text-decoration: underline; color: blue;">${rec.productName || 'ここをクリックして製品を検索'}</td>
            <td></td>
            <td class="right yj-quantity-cell">${yjQuantity.toFixed(2)}</td>
            <td class="right yj-pack-unit-qty-cell">${rec.yjPackUnitQty || ''}</td>
            <td class="yj-unit-name-cell">${rec.yjUnitName || ''}</td>
            <td></td><td></td><td></td>
            <td class="left client-code-cell">${clientMap.get(rec.clientCode) || rec.clientCode || ''}</td>
            <td class="right line-number-cell">${rec.lineNumber || ''}</td>
        </tr>`;

    const bottomRow = `
        <tr class="data-row-bottom">
            <td>予製</td>
            <td class="yj-jan-code jan-code-cell">${rec.janCode || rec.productCode || ''}</td>
            <td class="left package-spec-cell">${rec.packageSpec || rec.formattedPackageSpec || ''}</td>
            <td class="left maker-name-cell">${rec.makerName || ''}</td>
            <td class="left usage-classification-cell">${rec.usageClassification || ''}</td>
            <td><input type="number" name="janQuantity" value="${janQuantity}" step="any"></td>
            <td class="right jan-pack-unit-qty-cell">${rec.janPackUnitQty || ''}</td>
            <td class="jan-unit-name-cell">${rec.janUnitName || ''}</td>
            <td></td><td></td><td></td>
            <td class="left receipt-number-cell">${rec.receiptNumber || ''}</td>
            <td></td>
        </tr>`;
    return topRow + bottomRow;
}

function recalculateRow(quantityInputElement) {
    const lowerRow = quantityInputElement.closest('tr.data-row-bottom');
    if (!lowerRow) return;
    const topRow = lowerRow.previousElementSibling;
    if (!topRow || !topRow.classList.contains('data-row')) return;

    const productDataString = topRow.dataset.product;
    if (!productDataString || productDataString === '{}') return;
    const productData = JSON.parse(productDataString);

    const janQuantity = parseFloat(quantityInputElement.value) || 0;
    const yjQuantity = janQuantity * (productData.janPackInnerQty || 0);
    const yjQuantityCell = topRow.querySelector('.yj-quantity-cell');
    if (yjQuantityCell) {
        yjQuantityCell.textContent = yjQuantity.toFixed(2);
    }
}

export function populateDetailsTable(records) {
    if (!tableBody) return;
    if (!records || records.length === 0) {
        clearDetailsTable();
        return;
    }
    tableBody.innerHTML = records.map(createRowHTML).join('');
}

export function clearDetailsTable() {
    if (tableBody) {
        tableBody.innerHTML = '<tr><td colspan="14">患者番号を入力して「呼び出し」を押してください。</td></tr>';
    }
}

export function getDetailsData() {
    const records = [];
    outputContainer.querySelectorAll('tr.data-row[data-row-id]').forEach(row => {
        const productDataString = row.dataset.product;
        if (!productDataString || productDataString === '{}') return;

        const janQuantity = parseFloat(row.nextElementSibling.querySelector('input[name="janQuantity"]').value) || 0;
        if (janQuantity > 0) {
            const productData = JSON.parse(productDataString);
            const code = productData.productCode || productData.janCode; 

            if (code) { 
                records.push({
                    productCode: code,
                    janQuantity: janQuantity,
                });
            }
        }
    });
    return records;
}

export function initDetailsTable() {
    outputContainer = document.getElementById('precomp-details-container');
    addRowBtn = document.getElementById('precomp-add-row-btn');
    if (!outputContainer || !addRowBtn) return;
    outputContainer.innerHTML = createUploadTableHTML('precomp-details-table');
    tableBody = outputContainer.querySelector('tbody');
    clearDetailsTable();

    addRowBtn.addEventListener('click', () => {
        if (tableBody.querySelector('td[colspan="14"]')) {
            tableBody.innerHTML = '';
        }
        tableBody.insertAdjacentHTML('beforeend', createRowHTML());
    });
    tableBody.addEventListener('input', (e) => {
        if (e.target.matches('input[name="janQuantity"]')) {
            recalculateRow(e.target);
        }
    });
    tableBody.addEventListener('click', (e) => {
        const target = e.target;
        if (target.classList.contains('delete-row-btn')) {
            const topRow = target.closest('tr');
            const bottomRow = topRow.nextElementSibling;
            topRow.remove();
            if (bottomRow) bottomRow.remove();
            if (tableBody.children.length === 0) {
                clearDetailsTable();
            }
        }

        if (target.classList.contains('insert-row-btn')) {
			const topRow = target.closest('tr');
			const bottomRow = topRow.nextElementSibling;
			bottomRow.insertAdjacentHTML('afterend', createRowHTML());
		}

        if (target.classList.contains('product-name-cell')) {
            const activeRow = target.closest('tr');
            // この画面では /api/masters/search_all (製品マスター検索) を使用します
            showModal(activeRow, (selectedProduct, targetRow) => {
                targetRow.dataset.product = JSON.stringify(selectedProduct);
                const lowerRow = targetRow.nextElementSibling;

                targetRow.querySelector('.yj-jan-code').textContent = selectedProduct.yjCode || '';
                targetRow.querySelector('.product-name-cell').textContent = selectedProduct.productName || '';
                targetRow.querySelector('.yj-pack-unit-qty-cell').textContent = selectedProduct.yjPackUnitQty || '';
                targetRow.querySelector('.yj-unit-name-cell').textContent = selectedProduct.yjUnitName || '';
                
                lowerRow.querySelector('.jan-code-cell').textContent = selectedProduct.janCode || selectedProduct.productCode || '';
                lowerRow.querySelector('.package-spec-cell').textContent = selectedProduct.packageSpec || selectedProduct.formattedPackageSpec || '';
                lowerRow.querySelector('.maker-name-cell').textContent = selectedProduct.makerName || '';
                lowerRow.querySelector('.usage-classification-cell').textContent = selectedProduct.usageClassification || '';
                lowerRow.querySelector('.jan-pack-unit-qty-cell').textContent = selectedProduct.janPackUnitQty || '';
                lowerRow.querySelector('.jan-unit-name-cell').textContent = selectedProduct.janUnitName || '';
                const quantityInput = lowerRow.querySelector('input[name="janQuantity"]');
                recalculateRow(quantityInput);
                quantityInput.focus();
                quantityInput.select();
            }, { searchApi: '/api/masters/search_all' });
        }
    });
}