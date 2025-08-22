// C:\Dev\WASABI\static\js\deadstock.js

import { showModal } from './inout_modal.js';

// ▼▼▼ [修正点] 新しい入力欄の変数を追加 ▼▼▼
let view, outputContainer, startDateInput, endDateInput, excludeZeroStockCheckbox, createCsvBtn, kanaNameInput, dosageFormInput;
// ▲▲▲ 修正ここまで ▲▲▲

/**
 * 期限・ロット入力行のHTMLを生成する
 * @param {object} record - 保存済みのレコードデータ
 * @param {string} unitName - 単位名
 * @param {boolean} isFirstRow - その製品の最初の行かどうか
 * @returns {string} HTML文字列
 */
function createRowHTML(record = {}, unitName = '単位', isFirstRow = false) {
    const isNewRecord = !record.id;
    const stockQty = isNewRecord ? 0 : (record.stockQuantityJan || 0);
    const expiry = record.expiryDate || '';
    const lot = record.lotNumber || '';
    const recordId = record.id || `new-${Date.now()}`;
    
    const buttonHTML = isFirstRow
        ? `<button class="btn add-ds-row-btn">＋</button>`
        : `<button class="btn delete-ds-row-btn">－</button>`;

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
            <td>${buttonHTML}</td>
        </tr>
    `;
}

/**
 * 個別の製品（JAN）のブロックHTMLを生成する
 * @param {object} product - 製品データ
 * @returns {string} HTML文字列
 */
function createProductHTML(product) {
    const janUnit = product.yjUnitName || '単位';
    let rowsHTML = '';
    if (product.savedRecords && product.savedRecords.length > 0) {
        rowsHTML = product.savedRecords.map((rec, index) => createRowHTML(rec, janUnit, index === 0)).join('');
    } else {
        rowsHTML = createRowHTML({ stockQuantityJan: product.currentStock }, janUnit, true);
    }

    // ▼▼▼ [修正点] CSVソート用に製品名をdata属性に保持 ▼▼▼
    return `
        <div class="dead-stock-product-container" 
             data-product-code="${product.productCode}" 
             data-yj-code="${product.yjCode}"
             data-product-name="${product.productName}">
            <div class="product-header" style="padding: 4px 8px; background-color: #f0f0f0; border-top: 1px solid #ccc; display: flex; justify-content: space-between;">
                <span>
                    <strong>JAN: ${product.productCode}</strong>
                    <span style="margin-left: 10px;">${product.productName}</span>
                </span>
                <span>在庫: ${product.currentStock.toFixed(2)} ${janUnit}</span>
            </div>
            <div class="dead-stock-entry-container">
                <table class="dead-stock-entry-table">
                    <thead>
                        <tr>
                            <th style="width: 20%;">在庫数量</th>
                            <th style="width: 30%;">使用期限</th>
                            <th style="width: 40%;">ロット番号</th>
                            <th style="width: 10%;">操作</th>
                        </tr>
                    </thead>
                    <tbody>${rowsHTML}</tbody>
                </table>
                <div class="entry-controls" style="text-align: right; padding: 4px;">
                    <button class="btn save-ds-btn" style="background-color: #0d6efd; color: white;">このJANの期限・ロットを保存</button>
                </div>
            </div>
        </div>
    `;
}


/**
 * 包装グループのHTMLを生成する
 * @param {object} pkgGroup - 包装グループデータ
 * @returns {string} HTML文字列
 */
function createPackageGroupHTML(pkgGroup) {
    const productsHTML = pkgGroup.products.map(createProductHTML).join('');
    const unitName = pkgGroup.products.length > 0 ? (pkgGroup.products[0].yjUnitName || '単位') : '単位';
    return `
        <div class="dead-stock-package-container">
            <div class="agg-pkg-header">
                <span>包装: ${pkgGroup.packageKey}</span>
                <span class="header-info">合計在庫: ${pkgGroup.totalStock.toFixed(2)} ${unitName}</span>
            </div>
            <div class="products-container">${productsHTML}</div>
        </div>
    `;
}


/**
 * APIからのレスポンスを元にデッドストックリスト全体を描画する
 * @param {Array} data - APIから取得したデータ
 */
function renderDeadStockList(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>対象データが見つかりませんでした。</p>";
        return;
    }

    let html = '';
    data.forEach(group => {
        const unitName = group.packageGroups.length > 0 && group.packageGroups[0].products.length > 0 ? (group.packageGroups[0].products[0].yjUnitName || '単位') : '単位';
        // ▼▼▼ [修正点] CSVソート用にカナ名をdata属性に保持 ▼▼▼
        html += `<div class="yj-group-wrapper" 
                      data-yj-code-wrapper="${group.yjCode}"
                      data-kana-name="${group.productName}">
            <div class="agg-yj-header">
                <div style="flex-grow: 1;">
                    <span>YJ: ${group.yjCode}</span>
                    <span class="product-name">${group.productName}</span>
                    <span class="header-info">総在庫: ${group.totalStock.toFixed(2)} ${unitName}</span>
                </div>
                <button class="add-jan-btn btn" data-yj-code="${group.yjCode}">JAN品目追加</button>
            </div>
            <div class="packages-container">
                ${group.packageGroups.map(createPackageGroupHTML).join('')}
            </div>
        </div>`;
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
    createCsvBtn = document.getElementById('create-deadstock-csv-btn');
    // ▼▼▼ [修正点] 新しい入力欄の要素を取得 ▼▼▼
    kanaNameInput = document.getElementById('ds-kanaName');
    dosageFormInput = document.getElementById('ds-dosageForm');
    // ▲▲▲ 修正ここまで ▲▲▲

    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, today.getDate());
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);
    
    // ▼▼▼ [修正点] CSV作成ボタンのイベントリスナーを最終仕様に修正 ▼▼▼
    createCsvBtn.addEventListener('click', () => {
        const dataForCsv = [];

        // 1. 画面からデータを収集し、並べ替え可能な配列を作成
        document.querySelectorAll('.yj-group-wrapper').forEach(yjDiv => {
            const yjCode = yjDiv.querySelector('.agg-yj-header span:first-child').textContent.replace('YJ: ', '');
            const kanaName = yjDiv.dataset.kanaName; // ソート用のカナ名

            yjDiv.querySelectorAll('.dead-stock-product-container').forEach(productDiv => {
                const janCode = productDiv.dataset.productCode;
                const productName = productDiv.dataset.productName;

                productDiv.querySelectorAll('tbody tr').forEach(row => {
                    const quantityInput = row.querySelector('.ds-stock-quantity');
                    const quantity = parseFloat(quantityInput.value);
                    
                    // 数量が0より大きい行のみを対象とする
                    if (quantity > 0) {
                        dataForCsv.push({
                            yjCode: yjCode,
                            janCode: janCode,
                            productName: productName,
                            kanaName: kanaName, // ソート用
                            quantity: quantity,
                            unit: row.querySelector('.ds-unit-name').textContent,
                            expiry: row.querySelector('.ds-expiry-date').value,
                            lot: row.querySelector('.ds-lot-number').value.trim(),
                        });
                    }
                });
            });
        });

        if (dataForCsv.length === 0) {
            window.showNotification('数量が1以上のエクスポート対象データがありません。', 'error');
            return;
        }

        // 2. 収集したデータをHTMLの表示順（カナ名）と同じように並べ替え
        dataForCsv.sort((a, b) => {
            return a.kanaName.localeCompare(b.kanaName, 'ja');
        });

        // 3. CSV文字列を生成
        const header = ["YJコード", "JANコード", "製品名", "数量", "単位", "使用期限", "ロット番号"];
        const csvRows = dataForCsv.map(d => 
            [
                `"${d.yjCode}"`,
                `"${d.janCode}"`,
                `"${d.productName}"`,
                d.quantity,
                `"${d.unit}"`,
                `"${d.expiry}"`,
                `"${d.lot}"`
            ].join(',')
        );

        const csvContent = [header.join(','), ...csvRows].join('\r\n');

        // 4. ファイルとしてダウンロード
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement("a");
        
        const dateStr = endDateInput.value.replace(/-/g, '');
        const fileName = `デッドストック_${dateStr}.csv`;

        link.setAttribute("href", URL.createObjectURL(blob));
        link.setAttribute("download", fileName);
        link.style.visibility = 'hidden';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);

        window.showNotification('CSVファイルを作成しました。', 'success');
    });

    view.addEventListener('click', async (e) => {
        // ... (run-dead-stock-btn, save-ds-btn, などのリスナーは変更なし) ...
        const target = e.target;

        if (target.id === 'run-dead-stock-btn') {
            window.showLoading();
            // ▼▼▼ [修正点] 新しい絞り込み条件をAPIパラメータに追加 ▼▼▼
            const params = new URLSearchParams({
                startDate: startDateInput.value.replace(/-/g, ''),
                endDate: endDateInput.value.replace(/-/g, ''),
                excludeZeroStock: excludeZeroStockCheckbox.checked,
                kanaName: kanaNameInput.value,
                dosageForm: dosageFormInput.value,
            });
            // ▲▲▲ 修正ここまで ▲▲▲

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

        if (target.classList.contains('add-ds-row-btn')) {
            const productContainer = target.closest('.dead-stock-product-container');
            const unitNameSpan = productContainer.querySelector('.ds-unit-name');
            const unitName = unitNameSpan ? unitNameSpan.textContent : '単位';
            const tbody = productContainer.querySelector('tbody');
            tbody.insertAdjacentHTML('beforeend', createRowHTML({}, unitName, false));
        }

        if (target.classList.contains('delete-ds-row-btn')) {
            target.closest('tr').remove();
        }

        if (target.classList.contains('save-ds-btn')) {
            const productContainer = target.closest('.dead-stock-product-container');
            const productCode = productContainer.dataset.productCode;
            const yjCode = productContainer.dataset.yjCode;
            const rows = productContainer.querySelectorAll('tbody tr');
            const payload = [];

            const pkgContainer = productContainer.closest('.dead-stock-package-container');
            const pkgHeaderText = pkgContainer.querySelector('.agg-pkg-header span:first-child').textContent;
            const pkgInfo = pkgHeaderText.replace('包装: ', '').split('|');

            rows.forEach(row => {
                payload.push({
                    productCode: productCode,
                    yjCode: yjCode,
                    packageForm: pkgInfo[0] || '',
                    janPackInnerQty: parseFloat(pkgInfo[1]) || 0,
                    yjUnitName: pkgInfo[2] || '',
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
        
        if (target.classList.contains('add-jan-btn')) {
            const yjCode = target.dataset.yjCode;
            window.showLoading();
            try {
                const resMasters = await fetch(`/api/masters/by_yj_code?yj_code=${yjCode}`);
                if (!resMasters.ok) throw new Error('製品リストの取得に失敗');
                const masters = await resMasters.json();
                if (!masters || masters.length === 0) {
                    throw new Error('このYJコードに紐づく製品マスターが見つかりません。');
                }
                
                const targetWrapper = target.closest(`[data-yj-code-wrapper]`);
                showModal(targetWrapper, async (selectedProduct, wrapper) => {
                    window.showLoading();
                    try {
                        const resStock = await fetch(`/api/stock/current?jan_code=${selectedProduct.productCode}`);
                        if (!resStock.ok) throw new Error('在庫数の取得に失敗');
                        const stockData = await resStock.json();
                        
                        const newPackageKey = `${selectedProduct.packageForm}|${selectedProduct.janPackInnerQty}|${selectedProduct.yjUnitName}`;
                        
                        const newProductData = {
                            ...selectedProduct,
                            currentStock: stockData.stock,
                            savedRecords: [],
                        };
                        const newProductHTML = createProductHTML(newProductData);

                        const packagesContainer = wrapper.querySelector('.packages-container');
                        let targetPackageContainer = null;
                        
                        packagesContainer.querySelectorAll('.dead-stock-package-container').forEach(pkgDiv => {
                            const headerText = pkgDiv.querySelector('.agg-pkg-header span:first-child').textContent;
                            if (headerText.includes(newPackageKey)) {
                                targetPackageContainer = pkgDiv.querySelector('.products-container');
                            }
                        });

                        if (!targetPackageContainer) {
                            const newPackageGroupData = {
                                packageKey: newPackageKey,
                                totalStock: newProductData.currentStock,
                                products: [newProductData]
                            };
                            const newPackageGroupHTML = createPackageGroupHTML(newPackageGroupData);
                            packagesContainer.insertAdjacentHTML('beforeend', newPackageGroupHTML);
                        } else {
                            targetPackageContainer.insertAdjacentHTML('beforeend', newProductHTML);
                        }

                    } catch (err) {
                        window.showNotification(err.message, 'error');
                    } finally {
                        window.hideLoading();
                    }
                }, { initialResults: masters, searchApi: `/api/masters/by_yj_code?yj_code=${yjCode}` });
            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        }
    });
}