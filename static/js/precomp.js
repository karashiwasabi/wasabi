import { showModal } from './inout_modal.js';

let view, patientNumberInput, saveBtn, addRowBtn, tableBody, loadBtn, clearBtn;

function createRowHTML(rec = {}) {
    const isNew = !rec.id;
    const rowId = isNew ? `new-${Date.now()}` : rec.id;
    const janQuantity = isNew ? 1 : ((rec.janPackInnerQty > 0) ? (rec.quantity / rec.janPackInnerQty) : 0);
    
    const productData = isNew ? {} : {
        productCode: rec.productCode,
        productName: rec.productName,
        yjUnitName: rec.yjUnitName,
    };

    return `
        <tr data-row-id="${rowId}" data-product='${JSON.stringify(productData)}'>
            <td class="product-name-cell" style="cursor: pointer; text-decoration: underline; color: blue;">${rec.productName || '製品を選択'}</td>
            <td class="display-jan-code">${rec.productCode || ''}</td>
            <td class="display-yj-unit-name">${rec.yjUnitName || ''}</td>
            <td><input type="number" name="janQuantity" value="${janQuantity}" step="any" style="width: 100px;"></td>
            <td><button class="delete-row-btn btn">×</button></td>
        </tr>
    `;
}

function renderPrecompRows(records = []) {
    if (records.length === 0) {
        tableBody.innerHTML = '<tr class="no-data-row"><td colspan="5">データがありません。「行追加」してください。</td></tr>';
        return;
    }
    tableBody.innerHTML = records.map(rec => createRowHTML(rec)).join('');
}

function getDetailsData() {
    const records = [];
    const rows = tableBody.querySelectorAll('tr[data-row-id]');
    rows.forEach(row => {
        const productDataString = row.dataset.product;
        if (!productDataString || productDataString === '{}') return;
        
        const janQuantity = parseFloat(row.querySelector('input[name="janQuantity"]').value) || 0;
        if (janQuantity > 0) {
            records.push({
                productCode: JSON.parse(productDataString).productCode,
                janQuantity: janQuantity,
            });
        }
    });
    return records;
}

export function initPrecomp() {
    view = document.getElementById('precomp-view');
    if (!view) return;

    patientNumberInput = document.getElementById('precomp-patient-number');
    saveBtn = document.getElementById('precomp-save-btn');
    addRowBtn = document.getElementById('precomp-add-row-btn');
    tableBody = document.querySelector('#precomp-details-table tbody');
    loadBtn = document.getElementById('precomp-load-btn');
    clearBtn = document.getElementById('precomp-clear-btn');

    addRowBtn.addEventListener('click', () => {
        if (tableBody.querySelector('.no-data-row')) {
            tableBody.innerHTML = '';
        }
        tableBody.insertAdjacentHTML('beforeend', createRowHTML());
    });

    loadBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('患者番号を入力してください。', 'error');
            return;
        }
        window.showLoading();
        try {
            const res = await fetch(`/api/precomp/load?patientNumber=${encodeURIComponent(patientNumber)}`);
            if (!res.ok) throw new Error('データの呼び出しに失敗しました。');
            const records = await res.json();
            renderPrecompRows(records);
        } catch (err) {
            window.showNotification(err.message, 'error');
            tableBody.innerHTML = '<tr class="no-data-row"><td colspan="5">呼び出しに失敗しました。</td></tr>';
        } finally {
            window.hideLoading();
        }
    });

    clearBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('中断する患者番号を入力してください。', 'error');
            return;
        }
        if (!confirm(`患者番号: ${patientNumber} の予製データをすべて削除（中断）します。本当によろしいですか？`)) {
            return;
        }
        window.showLoading();
        try {
            const res = await fetch(`/api/precomp/clear?patientNumber=${encodeURIComponent(patientNumber)}`, {
                method: 'DELETE',
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '中断に失敗しました。');
            window.showNotification(resData.message, 'success');
            patientNumberInput.value = '';
            renderPrecompRows([]);
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });

    saveBtn.addEventListener('click', async () => {
        const patientNumber = patientNumberInput.value.trim();
        if (!patientNumber) {
            window.showNotification('患者番号を入力してください。', 'error');
            return;
        }

        const records = getDetailsData();
        if (records.length === 0) {
            window.showNotification('保存する明細がありません。', 'error');
            return;
        }

        if (!confirm(`患者番号: ${patientNumber} の予製データを保存します。よろしいですか？`)) {
            return;
        }

        const payload = { patientNumber, records };
        window.showLoading();
        try {
            const res = await fetch('/api/precomp/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
            
            window.showNotification(resData.message, 'success');
            patientNumberInput.value = '';
            renderPrecompRows([]);
        } catch (err) {
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
        }
    });

    tableBody.addEventListener('click', (e) => {
        if (e.target.classList.contains('delete-row-btn')) {
            e.target.closest('tr').remove();
            if (tableBody.children.length === 0) {
                 renderPrecompRows([]);
            }
        }

        if (e.target.classList.contains('product-name-cell')) {
            const activeRow = e.target.closest('tr');
        // ▼▼▼ 修正点: showModalの第3引数を正しいオブジェクト形式に変更 ▼▼▼
        showModal(activeRow, (selectedProduct, targetRow) => {
            const productData = {
                productCode: selectedProduct.productCode,
                productName: selectedProduct.productName,
                yjUnitName: selectedProduct.yjUnitName,
            };
            targetRow.dataset.product = JSON.stringify(productData);
            targetRow.querySelector('.product-name-cell').textContent = selectedProduct.productName;
            targetRow.querySelector('.display-jan-code').textContent = selectedProduct.productCode;
            targetRow.querySelector('.display-yj-unit-name').textContent = selectedProduct.yjUnitName;
            targetRow.querySelector('input[name="janQuantity"]').focus();
        }, { searchApi: '/api/masters/search_all' }); // 修正箇所
        // ▲▲▲ 修正ここまで ▲▲▲
        }
    });
}