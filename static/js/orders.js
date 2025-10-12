// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/orders.js
import { hiraganaToKatakana, getLocalDateString, toHalfWidth } from './utils.js';
import { wholesalerMap } from './master_data.js';

// ▼▼▼【ここから追加】▼▼▼
// 連続スキャン用のグローバル変数
let continuousOrderModal, continuousOrderBtn, closeContinuousModalBtn;
let continuousBarcodeForm, continuousBarcodeInput, scannedItemsList, scannedItemsCount, processingIndicator;
let scanQueue = [];
let isProcessingQueue = false;
// ▲▲▲【追加ここまで】▲▲▲

function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    return balance;
}

// ▼▼▼【ここから修正】▼▼▼
/**
 * 発注リストに品目を追加、または既に存在する場合は数量を更新する関数
 * @param {object} productMaster - 追加する製品のマスターデータ
 */
function addOrUpdateOrderItem(productMaster) {
    const outputContainer = document.getElementById('order-candidates-output');
    const productCode = productMaster.productCode;
    const yjCode = productMaster.yjCode;

    // 1. 既に同じJANコードの行が存在するかチェック
    const existingRow = outputContainer.querySelector(`tr[data-jan-code="${productCode}"]`);
    if (existingRow) {
        // 存在する場合、数量を1増やす
        const quantityInput = existingRow.querySelector('.order-quantity-input');
        if (quantityInput) {
            quantityInput.value = parseInt(quantityInput.value, 10) + 1;
            window.showNotification(`「${productMaster.productName}」の数量を1増やしました。`, 'success');
        }
        return;
    }

    // 2. 新しい行のHTMLを生成
    let wholesalerOptions = '<option value="">--- 選択 ---</option>';
    wholesalerMap.forEach((name, code) => {
        const isSelected = (code === productMaster.supplierWholesale);
        wholesalerOptions += `<option value="${code}" ${isSelected ? 'selected' : ''}>${name}</option>`;
    });

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
            <td><button class="remove-order-item-btn btn">除外</button></td>
        </tr>
    `;

    // 3. 対応するYJコードのグループを探す
    let yjGroupWrapper = outputContainer.querySelector(`.order-yj-group-wrapper[data-yj-code="${yjCode}"]`);

    if (yjGroupWrapper) {
        // 4a. 既存のYJグループに新しい行を追加
        const tbody = yjGroupWrapper.querySelector('tbody');
        tbody.insertAdjacentHTML('beforeend', newRowHTML);
    } else {
        // 4b. YJグループが存在しない場合、新しいグループ全体を作成
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

/**
 * 連続スキャンモーダル内のスキャン済みリストの表示を更新する関数
 */
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

/**
 * バックグラウンドでスキャンキューを処理する非同期関数
 */
async function processScanQueue() {
    if (isProcessingQueue) return; // 既に処理中なら何もしない

    isProcessingQueue = true;
    processingIndicator.classList.remove('hidden');

    while (scanQueue.length > 0) {
        const barcode = scanQueue.shift(); // キューから1件取り出す

        try {
            // GS1コードから製品情報を取得（既存のロジックを再利用）
            let gs1Code = '';
            if (barcode.startsWith('01') && barcode.length > 16) {
                gs1Code = barcode.substring(2, 16);
            } else {
                gs1Code = barcode;
            }

            const res = await fetch(`/api/product/by_gs1?gs1_code=${gs1Code}`);
            if (!res.ok) {
                if (res.status === 404) {
                    // マスターが存在しない場合は自動で作成
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
            // モーダル内の表示を更新して、処理済みの項目をリストから消す
            updateScannedItemsDisplay();
        }
    }

    isProcessingQueue = false;
    processingIndicator.classList.add('hidden');
}
// ▲▲▲【修正ここまで】▲▲▲

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

                        let actionCellHTML = '';
                        if (isOrderable) {
                            actionCellHTML = '<td><button class="remove-order-item-btn btn">除外</button></td>';
                        } else {
                            actionCellHTML = '<td><button class="change-to-orderable-btn btn">発注に変更</button></td>';
                        }

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

    // ▼▼▼【ここから追加】▼▼▼
    // 連続スキャンモーダル用の要素を取得
    continuousOrderModal = document.getElementById('continuous-order-modal');
    continuousOrderBtn = document.getElementById('continuous-order-btn');
    closeContinuousModalBtn = document.getElementById('close-continuous-modal-btn');
    continuousBarcodeForm = document.getElementById('continuous-barcode-form');
    continuousBarcodeInput = document.getElementById('continuous-barcode-input');
    scannedItemsList = document.getElementById('scanned-items-list');
    scannedItemsCount = document.getElementById('scanned-items-count');
    processingIndicator = document.getElementById('processing-indicator');

    // モーダル表示ボタンのイベント
    continuousOrderBtn.addEventListener('click', () => {
        scanQueue = [];
        updateScannedItemsDisplay();
        continuousOrderModal.classList.remove('hidden');
        document.body.classList.add('modal-open');
        setTimeout(() => continuousBarcodeInput.focus(), 100);
    });

    // モーダル閉じるボタンのイベント
    closeContinuousModalBtn.addEventListener('click', () => {
        continuousOrderModal.classList.add('hidden');
        document.body.classList.remove('modal-open');
    });

    // モーダル内のフォーム送信（バーコードスキャン完了）イベント
    continuousBarcodeForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const barcode = continuousBarcodeInput.value.trim();
        if (barcode) {
            scanQueue.push(barcode);
            updateScannedItemsDisplay();
            processScanQueue(); // バックグラウンド処理を開始/継続
        }
        continuousBarcodeInput.value = '';
    });
    // ▲▲▲【追加ここまで】▲▲▲

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
    outputContainer.addEventListener('click', (e) => {
        const target = e.target;
        
        if (target.classList.contains('change-to-orderable-btn')) {
            const row = target.closest('tr');
            if (row) {
                row.classList.remove('provisional-order-item');
                row.querySelector('.wholesaler-select').disabled = false;
                row.querySelector('.order-quantity-input').disabled = false;

                target.textContent = '除外';
                target.classList.remove('change-to-orderable-btn');
                target.classList.add('remove-order-item-btn');
            }
        } else if (target.classList.contains('remove-order-item-btn')) {
            const row = target.closest('tr');
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