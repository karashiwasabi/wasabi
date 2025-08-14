// C:\Dev\WASABI\static\js\deadstock.js

let view, outputContainer, startDateInput, endDateInput, excludeZeroStockCheckbox;

// ▼▼▼ [修正点] HTML生成時に、後で使うYJコードをdata属性に埋め込む ▼▼▼
function createPackageHTML(pkg, yjCode) {
    let rowsHTML = '';
    const janUnit = pkg.yjUnitName || '単位';

    if (pkg.savedRecords && pkg.savedRecords.length > 0) {
        rowsHTML = pkg.savedRecords.map(rec => createRowHTML(rec, janUnit)).join('');
    } else {
        rowsHTML = createRowHTML({ stockQuantityJan: pkg.currentStock }, janUnit);
    }

    return `
        <div class="agg-pkg-header">
            <span>包装: ${pkg.packageForm}|${pkg.janPackInnerQty}|${pkg.yjUnitName}</span>
            <span class="header-info">在庫: ${pkg.currentStock.toFixed(2)} ${janUnit}</span>
        </div>
        <div class="dead-stock-entry-container" data-product-code="${pkg.productCode}" data-yj-code="${yjCode}">
            <table class="dead-stock-entry-table">
                <thead>
                    <tr>
                        <th style="width: 20%;">在庫数量</th>
                        <th style="width: 30%;">使用期限</th>
                        <th style="width: 40%;">ロット番号</th>
                        <th style="width: 10%;">削除</th>
                    </tr>
                </thead>
                <tbody>
                    ${rowsHTML}
                </tbody>
            </table>
            <div class="entry-controls">
                <button class="btn add-ds-row-btn">行追加</button>
                <button class="btn save-ds-btn" style="background-color: #0d6efd; color: white;">この包装の期限・ロットを保存</button>
            </div>
        </div>
    `;
}
// ▲▲▲ 修正ここまで ▲▲▲

function createRowHTML(record = {}, unitName = '単位') {
    const stockQty = record.stockQuantityJan || 0;
    const expiry = record.expiryDate || '';
    const lot = record.lotNumber || '';
    const recordId = record.id || `new-${Date.now()}`;
    return `
        <tr data-record-id="${recordId}">
            <td>
                <div style="display: flex; align-items: center;">
                    <input type="number" class="ds-stock-quantity" value="${stockQty}" step="any" style="flex-grow: 1;">
                    <span class="ds-unit-name" style="margin-left: 5px;">${unitName}</span>
                </div>
            </td>
            <td><input type="text" class="ds-expiry-date" placeholder="YYYYMM" value="${expiry}"></td>
            <td><input type="text" class="ds-lot-number" value="${lot}"></td>
            <td><button class="btn delete-ds-row-btn">－</button></td>
        </tr>
    `;
}

function renderDeadStockList(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>対象データが見つかりませんでした。</p>";
        return;
    }

    let html = '';
    data.forEach(group => {
        const janUnit = group.packages.length > 0 ? (group.packages[0].yjUnitName || '単位') : '単位';
        html += `
            <div class="agg-yj-header">
                <span>YJ: ${group.yjCode}</span>
                <span class="product-name">${group.productName}</span>
                <span class="header-info">総在庫: ${group.totalStock.toFixed(2)} ${janUnit}</span>
            </div>
        `;
        group.packages.forEach(pkg => {
            // ▼▼▼ [修正点] createPackageHTML に group.yjCode を渡す ▼▼▼
            html += createPackageHTML(pkg, group.yjCode);
            // ▲▲▲ 修正ここまで ▲▲▲
        });
    });
    outputContainer.innerHTML = html;
}


export function initDeadStock() {
    view = document.getElementById('deadstock-view');
    if (!view) return;

    outputContainer = document.getElementById('deadstock-output-container');
    startDateInput = document.getElementById('ds-startDate');
    endDateInput = document.getElementById('ds-endDate');
    excludeZeroStockCheckbox = document.getElementById('ds-exclude-zero-stock');

    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, today.getDate());
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);
    
    view.addEventListener('click', async (e) => {
        if (e.target.id === 'run-dead-stock-btn') {
            window.showLoading();
            const params = new URLSearchParams({
                startDate: startDateInput.value.replace(/-/g, ''),
                endDate: endDateInput.value.replace(/-/g, ''),
                excludeZeroStock: excludeZeroStockCheckbox.checked,
            });

            try {
                const res = await fetch(`/api/deadstock/list?${params.toString()}`);
                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || 'Failed to generate dead stock list');
                }
                const data = await res.json();
                renderDeadStockList(data);
            } catch (err) {
                outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
            } finally {
                window.hideLoading();
            }
        }

        if (e.target.classList.contains('add-ds-row-btn')) {
            const container = e.target.closest('.dead-stock-entry-container');
            const unitNameSpan = container.querySelector('.ds-unit-name');
            const unitName = unitNameSpan ? unitNameSpan.textContent : '単位';
            const tbody = container.querySelector('tbody');
            tbody.insertAdjacentHTML('beforeend', createRowHTML({}, unitName));
        }

        if (e.target.classList.contains('delete-ds-row-btn')) {
            e.target.closest('tr').remove();
        }

        if (e.target.classList.contains('save-ds-btn')) {
            const container = e.target.closest('.dead-stock-entry-container');
            const productCode = container.dataset.productCode;
            // ▼▼▼ [修正点] YJコードをdata属性から安全に取得する ▼▼▼
            const yjCode = container.dataset.yjCode;
            // ▲▲▲ 修正ここまで ▲▲▲
            const rows = container.querySelectorAll('tbody tr');
            const payload = [];

            const header = container.previousElementSibling;
            const pkgInfo = header.querySelector('span:first-child').textContent.replace('包装: ', '').split('|');
            
            rows.forEach(row => {
                payload.push({
                    productCode: productCode,
                    yjCode: yjCode, // 正しいYJコードをセット
                    packageForm: pkgInfo[0],
                    janPackInnerQty: parseFloat(pkgInfo[1]) || 0,
                    yjUnitName: pkgInfo[2],
                    stockQuantityJan: parseFloat(row.querySelector('.ds-stock-quantity').value) || 0,
                    expiryDate: row.querySelector('.ds-expiry-date').value,
                    lotNumber: row.querySelector('.ds-lot-number').value,
                });
            });

            window.showLoading();
            try {
                const res = await fetch('/api/deadstock/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
                window.showNotification(resData.message, 'success');
            } catch (err) {
                window.showNotification(`エラー: ${err.message}`, 'error');
            } finally {
                window.hideLoading();
            }
        }
    });
}