// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/orders.js
import { hiraganaToKatakana, getLocalDateString, toHalfWidth } from './utils.js';
import { wholesalerMap } from './master_data.js';
import { showModal } from './inout_modal.js';

let continuousOrderModal, continuousOrderBtn, closeContinuousModalBtn;
let continuousBarcodeForm, continuousBarcodeInput, scannedItemsList, scannedItemsCount, processingIndicator;
let scanQueue = [];
let isProcessingQueue = false;

function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    return balance;
}

function addOrUpdateOrderItem(productMaster) {
    const outputContainer = document.getElementById('order-candidates-output');
    const productCode = productMaster.productCode;
    const yjCode = productMaster.yjCode;

    const existingRow = outputContainer.querySelector(`tr[data-jan-code="${productCode}"]`);
    if (existingRow) {
        const quantityInput = existingRow.querySelector('.order-quantity-input');
        if (quantityInput) {
            quantityInput.value = parseInt(quantityInput.value, 10) + 1;
            window.showNotification(`「${productMaster.productName}」の数量を1増やしました。`, 'success');
        }
        return;
    }

    let wholesalerOptions = '<option value="">--- 選択 ---</option>';
    wholesalerMap.forEach((name, code) => {
        const isSelected = (code === productMaster.supplierWholesale);
        wholesalerOptions += `<option value="${code}" ${isSelected ? 'selected' : ''}>${name}</option>`;
    });

    const actionCellHTML = `
        <td class="order-actions-cell">
            <div class="order-action-buttons">
                <button class="remove-order-item-btn btn">除外</button>
                <button class="go-to-master-btn btn" data-product-code="${productMaster.productCode}">ﾏｽﾀ</button>
                <button class="set-unorderable-btn btn" data-product-code="${productMaster.productCode}">発注不可</button>
                <button class="go-to-inv-adj-btn btn" data-yj-code="${productMaster.yjCode}">棚卸調整</button>
            </div>
        </td>
    `;

    const newRowHTML = `
        <tr data-jan-code="${productMaster.productCode}" 
            data-yj-code="${productMaster.yjCode}"
            data-product-name="${productMaster.productName}"
            data-package-form="${productMaster.packageForm}"
            data-jan-pack-inner-qty="${productMaster.janPackInnerQty}"
            data-yj-unit-name="${productMaster.yjUnitName}"
            data-yj-pack-unit-qty="${productMaster.yjPackUnitQty}"
            data-order-multiplier="${productMaster.yjPackUnitQty}"> 
            <td class="left">${productMaster.productName}</td>
            <td class="left">${productMaster.makerName || ''}</td>
            <td class="left">${productMaster.formattedPackageSpec}</td>
            <td><select class="wholesaler-select" style="width: 100%;">${wholesalerOptions}</select></td>
            <td>1包装 (${productMaster.yjPackUnitQty} ${productMaster.yjUnitName})</td>
            <td><input type="number" value="1" class="order-quantity-input" style="width: 80px;"></td>
            ${actionCellHTML}
        </tr>
    `;

    let yjGroupWrapper = outputContainer.querySelector(`.order-yj-group-wrapper[data-yj-code="${yjCode}"]`);

    if (yjGroupWrapper) {
        const tbody = yjGroupWrapper.querySelector('tbody');
        tbody.insertAdjacentHTML('beforeend', newRowHTML);
    } else {
        const yjHeaderHTML = `
            <div class="agg-yj-header">
                <span>YJ: ${yjCode}</span>
                <span class="product-name">${productMaster.productName}</span>
            </div>`;
        
        const tableHTML = `
            <table class="data-table" style="margin-bottom: 20px;">
                <thead>
                    <tr>
                        <th style="width: 25%;">製品名（包装）</th>
                        <th style="width: 15%;">メーカー</th>
                        <th style="width: 15%;">包装仕様</th>
                        <th style="width: 20%;">卸業者</th>
                        <th style="width: 10%;">発注単位</th>
                        <th style="width: 5%;">発注数</th>
                        <th style="width: 10%;">操作</th>
                    </tr>
                </thead>
                <tbody>
                    ${newRowHTML}
                </tbody>
            </table>`;
        
        const newGroupHTML = `
            <div class="order-yj-group-wrapper" data-yj-code="${yjCode}">
                ${yjHeaderHTML}
                ${tableHTML}
            </div>`;
        
        outputContainer.insertAdjacentHTML('beforeend', newGroupHTML);
    }
    window.showNotification(`「${productMaster.productName}」を発注リストに追加しました。`, 'success');
}

function updateScannedItemsDisplay() {
    const counts = scanQueue.reduce((acc, code) => {
        acc[code] = (acc[code] || 0) + 1;
        return acc;
    }, {});

    scannedItemsList.innerHTML = Object.entries(counts).map(([code, count]) => {
        return `<div class="scanned-item">
                    <span class="scanned-item-name">${code}</span>
                    <span class="scanned-item-count">x ${count}</span>
                </div>`;
    }).join('');
    scannedItemsCount.textContent = scanQueue.length;
}

async function processScanQueue() {
    if (isProcessingQueue) return;

    isProcessingQueue = true;
    processingIndicator.classList.remove('hidden');

    while (scanQueue.length > 0) {
        const barcode = scanQueue.shift();

        try {
            let gs1Code = '';
            if (barcode.startsWith('01') && barcode.length > 16) {
                gs1Code = barcode.substring(2, 16);
            } else {
                gs1Code = barcode;
            }

            const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1Code}`);
            if (!res.ok) {
                if (res.status === 404) {
                    const newMaster = await createAndFetchMaster(gs1Code);
                    addOrUpdateOrderItem(newMaster);
                } else {
                    throw new Error(`サーバーエラー (HTTP ${res.status})`);
                }
            } else {
                const productMaster = await res.json();
                addOrUpdateOrderItem(productMaster);
            }
        } catch (err) {
            console.error(`バーコード[${barcode}]の処理に失敗:`, err);
            window.showNotification(`バーコード[${barcode}]の処理に失敗しました: ${err.message}`, 'error');
        } finally {
            updateScannedItemsDisplay();
        }
    }

    isProcessingQueue = false;
    processingIndicator.classList.add('hidden');
}

function renderOrderCandidates(data, container, wholesalers) {
    if (!data || data.length === 0) {
        container.innerHTML = "<p>発注が必要な品目はありませんでした。</p>";
        return;
    }

    let html = '';
    data.forEach(yjGroup => {
        const yjShortfall = yjGroup.totalReorderPoint - (yjGroup.endingBalance || 0);

        html += `
            <div class="order-yj-group-wrapper" data-yj-code="${yjGroup.yjCode}">
                <div class="agg-yj-header" style="background-color: #ff0015ff;">
                    <span>YJ: ${yjGroup.yjCode}</span>
                    <span class="product-name">${yjGroup.productName}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(yjGroup.endingBalance)} | 
                        発注点: ${formatBalance(yjGroup.totalReorderPoint)} | 
                        不足数: ${formatBalance(yjShortfall)}
                    </span>
                </div>
        `;

        const existingBackordersForYj = yjGroup.packageLedgers.flatMap(p => p.existingBackorders || []);
        if (existingBackordersForYj.length > 0) {
            html += `<div class="existing-backorders-info">
                        <strong>＜既存の発注残＞</strong>
                        <ul>`;
            existingBackordersForYj.forEach(bo => {
                const wName = wholesalerMap.get(bo.wholesalerCode.String) || bo.wholesalerCode.String || '不明';
                html += `<li>${bo.orderDate}: ${bo.productName} - 数量: ${bo.remainingQuantity.toFixed(2)} (${wName})</li>`;
            });
            html += `</ul></div>`;
        }

        html += `
                <table class="data-table" style="margin-bottom: 20px;">
                    <thead>
                        <tr>
                            <th style="width: 25%;">製品名（包装）</th>
                            <th style="width: 15%;">メーカー</th>
                            <th style="width: 15%;">包装仕様</th>
                            <th style="width: 20%;">卸業者</th>
                            <th style="width: 10%;">発注単位</th>
                            <th style="width: 5%;">発注数</th>
                            <th style="width: 10%;">操作</th>
                        </tr>
                    </thead>
                    <tbody>
        `;
        yjGroup.packageLedgers.forEach(pkg => {
            if (pkg.masters && pkg.masters.length > 0) {
                pkg.masters.forEach(master => {
                    const pkgShortfall = pkg.reorderPoint - (pkg.endingBalance || 0);
                    if (pkgShortfall > 0) {
                        const isProvisional = master.productCode.startsWith('99999') && master.productCode.length > 13;
                        const isOrderStopped = master.isOrderStopped === 1;
                        const isOrderable = !isProvisional && !isOrderStopped;

                        const rowClass = !isOrderable ? 'provisional-order-item' : '';
                        const disabledAttr = !isOrderable ? 'disabled' : '';

                        const recommendedOrder = master.yjPackUnitQty > 0 ? Math.ceil(pkgShortfall / master.yjPackUnitQty) : 0;
                        
                        let rowWholesalerOptions = '<option value="">--- 選択 ---</option>';
                        wholesalers.forEach(w => {
                            const isSelected = (w.code === master.supplierWholesale);
                            rowWholesalerOptions += `<option value="${w.code}" ${isSelected ? 'selected' : ''}>${w.name}</option>`;
                        });

                        let actionCellHTML = `
                            <td class="order-actions-cell">
                                <div class="order-action-buttons">
                        `;
                        if (isOrderable) {
                            actionCellHTML += '<button class="remove-order-item-btn btn">除外</button>';
                        } else {
                            actionCellHTML += '<button class="change-to-orderable-btn btn">発注に変更</button>';
                        }
                        actionCellHTML += `
                                    <button class="go-to-master-btn btn" data-product-code="${master.productCode}">ﾏｽﾀ</button>
                                    <button class="set-unorderable-btn btn" data-product-code="${master.productCode}">発注不可</button>
                                    <button class="go-to-inv-adj-btn btn" data-yj-code="${yjGroup.yjCode}">棚卸調整</button>
                                </div>
                            </td>
                        `;

                        html += `
                            <tr class="${rowClass}" 
                                data-jan-code="${master.productCode}" 
                                data-yj-code="${yjGroup.yjCode}"
                                data-product-name="${master.productName}"
                                data-package-form="${master.packageForm}"
                                data-jan-pack-inner-qty="${master.janPackInnerQty}"
                                data-yj-unit-name="${master.yjUnitName}"
                                data-yj-pack-unit-qty="${master.yjPackUnitQty}"
                                data-order-multiplier="${master.yjPackUnitQty}"> 
                                <td class="left">${master.productName}</td>
                                <td class="left">${master.makerName || ''}</td>
                                <td class="left">${master.formattedPackageSpec}</td>
                                <td><select class="wholesaler-select" style="width: 100%;" ${disabledAttr}>${rowWholesalerOptions}</select></td>
                                <td>1包装 (${master.yjPackUnitQty} ${master.yjUnitName})</td>
                                <td><input type="number" value="${recommendedOrder}" class="order-quantity-input" style="width: 80px;" ${disabledAttr}></td>
                                ${actionCellHTML}
                            </tr>
                        `;
                    }
                });
            }
        });
        html += `</tbody></table></div>`;
    });
    container.innerHTML = html;
}

async function handleOrderBarcodeScan(e) {
    e.preventDefault();
    const barcodeInput = document.getElementById('order-barcode-input');
    const inputValue = barcodeInput.value.trim();
    if (!inputValue) return;

    let gs1Code = '';
    if (inputValue.startsWith('01') && inputValue.length > 16) {
        gs1Code = inputValue.substring(2, 16);
    } else {
        gs1Code = inputValue;
    }

    if (!gs1Code) {
        window.showNotification('有効なGS1コードではありません。', 'error');
        return;
    }

    window.showLoading('製品情報を検索中...');
    try {
        const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1Code}`);
        if (!res.ok) {
            if (res.status === 404) {
                if (confirm(`このGS1コードはマスターに登録されていません。\n新規マスターを作成して発注リストに追加しますか？`)) {
                    const newMaster = await createAndFetchMaster(gs1Code);
                    addOrUpdateOrderItem(newMaster);
                } else {
                    throw new Error('このGS1コードはマスターに登録されていません。');
                }
            } else {
                throw new Error('製品情報の検索に失敗しました。');
            }
        } else {
            const productMaster = await res.json();
            addOrUpdateOrderItem(productMaster);
        }
        barcodeInput.value = '';
        barcodeInput.focus();
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

async function createAndFetchMaster(gs1Code) {
    window.showLoading('新規マスターを作成中...');
    try {
        const productCode = gs1Code.length === 14 ? gs1Code.substring(1) : gs1Code;

        const createRes = await fetch('/api/master/create_provisional', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ gs1Code, productCode }),
        });
        const createData = await createRes.json();
        if (!createRes.ok) {
            throw new Error(createData.message || 'マスターの作成に失敗しました。');
        }
        window.showNotification(`新規マスターを作成しました (YJ: ${createData.yjCode})`, 'success');
        window.showLoading('作成したマスター情報を取得中...');
        const fetchRes = await fetch(`/api/product/by_gs1?gs1_code=${gs1Code}`);
        if (!fetchRes.ok) {
            throw new Error('作成されたマスター情報の取得に失敗しました。');
        }
        const newMaster = await fetchRes.json();
        return newMaster;

    } catch (err) {
        throw err;
    }
}

export function initOrders() {
    const view = document.getElementById('order-view');
    if (!view) return;

    const runBtn = document.getElementById('generate-order-candidates-btn');
    const outputContainer = document.getElementById('order-candidates-output');
    const kanaNameInput = document.getElementById('order-kanaName');
    const dosageFormInput = document.getElementById('order-dosageForm');
    const coefficientInput = document.getElementById('order-reorder-coefficient');
    const createCsvBtn = document.getElementById('createOrderCsvBtn');
    const barcodeInput = document.getElementById('order-barcode-input');
    const barcodeForm = document.getElementById('order-barcode-form');
    const shelfNumberInput = document.getElementById('order-shelf-number');
    const addFromMasterBtn = document.getElementById('add-order-item-from-master-btn');

    continuousOrderModal = document.getElementById('continuous-order-modal');
    continuousOrderBtn = document.getElementById('continuous-order-btn');
    closeContinuousModalBtn = document.getElementById('close-continuous-modal-btn');
    continuousBarcodeForm = document.getElementById('continuous-barcode-form');
    continuousBarcodeInput = document.getElementById('continuous-barcode-input');
    scannedItemsList = document.getElementById('scanned-items-list');
    scannedItemsCount = document.getElementById('scanned-items-count');
    processingIndicator = document.getElementById('processing-indicator');

    // ▼▼▼【ここから修正】▼▼▼
    addFromMasterBtn.addEventListener('click', () => {
        showModal(view, async (selectedProduct) => {
            window.showLoading('品目を準備しています...');
            try {
                let masterToAdd;
                if (selectedProduct.isAdopted) {
                    // 採用済みの場合は、JANコードで完全なマスター情報を取得しなおす
                    const res = await fetch(`/api/master/by_code/${selectedProduct.productCode}`);
                    if (!res.ok) {
                        const errJson = await res.json().catch(() => ({ message: '採用済みマスター情報の取得に失敗しました。' }));
                        throw new Error(errJson.message);
                    }
                    masterToAdd = await res.json();
                } else {
                    // 未採用の場合は、JCSHMSからマスターを新規作成するAPIを叩く
                    const createRes = await fetch('/api/master/create_from_jcshms', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ productCode: selectedProduct.productCode })
                    });
                    if (!createRes.ok) {
                        const errData = await createRes.json();
                        throw new Error(errData.message || 'JCSHMSからのマスター作成に失敗しました。');
                    }
                    masterToAdd = await createRes.json();
                }
                addOrUpdateOrderItem(masterToAdd);
            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        });
    });
    // ▲▲▲【修正ここまで】▲▲▲

    continuousOrderBtn.addEventListener('click', () => {
        scanQueue = [];
        updateScannedItemsDisplay();
        continuousOrderModal.classList.remove('hidden');
        document.body.classList.add('modal-open');
        setTimeout(() => continuousBarcodeInput.focus(), 100);
    });

    closeContinuousModalBtn.addEventListener('click', () => {
        continuousOrderModal.classList.add('hidden');
        document.body.classList.remove('modal-open');
    });

    continuousBarcodeForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const barcode = continuousBarcodeInput.value.trim();
        if (barcode) {
            scanQueue.push(barcode);
            updateScannedItemsDisplay();
            processScanQueue();
        }
        continuousBarcodeInput.value = '';
    });
    
    if (barcodeForm) {
        barcodeForm.addEventListener('submit', handleOrderBarcodeScan);
    }
    
    runBtn.addEventListener('click', async () => {
        window.showLoading();
        const params = new URLSearchParams({
            kanaName: hiraganaToKatakana(kanaNameInput.value),
            dosageForm: dosageFormInput.value,
            shelfNumber: shelfNumberInput.value,
            coefficient: coefficientInput.value,
        });

        try {
            const res = await fetch(`/api/orders/candidates?${params.toString()}`);
            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || 'List generation failed');
            }
            const data = await res.json();
            
            renderOrderCandidates(data.candidates, outputContainer, data.wholesalers || []);

        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    });

    createCsvBtn.addEventListener('click', async () => {
        const rows = outputContainer.querySelectorAll('tbody tr');
        if (rows.length === 0) {
            window.showNotification('発注する品目がありません。', 'error');
            return;
        }

        const backorderPayload = [];
        let csvContent = "";
        let hasItemsToOrder = false;

        rows.forEach(row => {
            if (row.classList.contains('provisional-order-item')) {
                return; 
            }
            
            const quantityInput = row.querySelector('.order-quantity-input');
            const quantity = parseInt(quantityInput.value, 10);
            
            if (quantity > 0) {
                hasItemsToOrder = true;
                
                const janCode = row.dataset.janCode;
                const productName = row.cells[0].textContent;
                const wholesalerCode = row.querySelector('.wholesaler-select').value;
    
                const csvRow = [janCode, `"${productName}"`, quantity, wholesalerCode].join(',');
                csvContent += csvRow + "\r\n";

                const orderMultiplier = parseFloat(row.dataset.orderMultiplier) || 0;
                
                backorderPayload.push({
                    yjCode: row.dataset.yjCode,
                    packageForm: row.dataset.packageForm,
                    janPackInnerQty: parseFloat(row.dataset.janPackInnerQty),
                    yjUnitName: row.dataset.yjUnitName,
                    yjQuantity: quantity * orderMultiplier,
                    productName: row.dataset.productName,
                    yjPackUnitQty: parseFloat(row.dataset.yjPackUnitQty) || 0,
                    wholesalerCode: { String: wholesalerCode, Valid: wholesalerCode !== "" },
                });
            }
        });

        if (!hasItemsToOrder) {
            window.showNotification('発注数が1以上の品目がありません。', 'error');
            return;
        }

        window.showLoading();
        try {
            const res = await fetch('/api/orders/place', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(backorderPayload),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '発注残の登録に失敗しました。');
            
            window.showNotification(resData.message, 'success');
            const sjisArray = Encoding.convert(csvContent, {
                to: 'SJIS',
                from: 'UNICODE',
                type: 'array'
            });
            const sjisUint8Array = new Uint8Array(sjisArray);

            const blob = new Blob([sjisUint8Array], { type: 'text/csv' });
            const link = document.createElement("a");
            const url = URL.createObjectURL(blob);
            const now = new Date();
            const timestamp = `${now.getFullYear()}${(now.getMonth()+1).toString().padStart(2, '0')}${now.getDate().toString().padStart(2, '0')}_${now.getHours().toString().padStart(2, '0')}${now.getMinutes().toString().padStart(2, '0')}${now.getSeconds().toString().padStart(2, '0')}`;
            const fileName = `発注書_${timestamp}.csv`;
            
            link.setAttribute("href", url);
            link.setAttribute("download", fileName);
            link.style.visibility = 'hidden';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        } catch(err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });

    outputContainer.addEventListener('click', async (e) => {
        const target = e.target;
        const row = target.closest('tr');

        if (target.classList.contains('go-to-master-btn')) {
            const productCode = target.dataset.productCode;
            document.dispatchEvent(new CustomEvent('navigateToMasterEdit', {
                detail: { productCode },
                bubbles: true
            }));
        } else if (target.classList.contains('set-unorderable-btn')) {
            const productCode = target.dataset.productCode;
            const productName = row ? row.cells[0].textContent : productCode;
            if (!confirm(`「${productName}」を発注不可に設定しますか？\nこの品目は今後、不足品リストに表示されなくなります。`)) {
                return;
            }
            window.showLoading('マスターを更新中...');
            try {
                const res = await fetch('/api/master/set_order_stopped', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ productCode: productCode, status: 1 }),
                });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '更新に失敗しました。');

                if (row) {
                    row.classList.add('provisional-order-item');
                    row.querySelector('.wholesaler-select').disabled = true;
                    row.querySelector('.order-quantity-input').disabled = true;
                    target.disabled = true; // ボタン自体も無効化
                }
                window.showNotification(`「${productName}」を発注不可に設定しました。`, 'success');

            } catch(err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        } else if (target.classList.contains('go-to-inv-adj-btn')) {
            const yjCode = target.dataset.yjCode;
            document.dispatchEvent(new CustomEvent('navigateToInventoryAdjustment', {
                detail: { yjCode },
                bubbles: true
            }));
        } else if (target.classList.contains('change-to-orderable-btn')) {
            if (row) {
                row.classList.remove('provisional-order-item');
                row.querySelector('.wholesaler-select').disabled = false;
                row.querySelector('.order-quantity-input').disabled = false;

                target.textContent = '除外';
                target.classList.remove('change-to-orderable-btn');
                target.classList.add('remove-order-item-btn');
            }
        } else if (target.classList.contains('remove-order-item-btn')) {
            const tbody = row.closest('tbody');
            const table = tbody.closest('table');
            const wrapper = table.closest('.order-yj-group-wrapper');
            row.remove();
            
            if (tbody.children.length === 0 && wrapper) {
                wrapper.remove();
            }
        }
    });
}