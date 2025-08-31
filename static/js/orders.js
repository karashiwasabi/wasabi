// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\orders.js

import { hiraganaToKatakana } from './utils.js';

function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    return balance;
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
                        // ▼▼▼ [ここから修正] ▼▼▼
                        const isProvisional = master.productCode.startsWith('99999') && master.productCode.length > 13;
                        const rowClass = isProvisional ? 'provisional-order-item' : '';
                        const disabledAttr = isProvisional ? 'disabled' : '';

                        const recommendedOrder = master.yjPackUnitQty > 0 ? Math.ceil(pkgShortfall / master.yjPackUnitQty) : 0;
                        
                        let rowWholesalerOptions = '<option value="">--- 選択 ---</option>';
                        wholesalers.forEach(w => {
                            const isSelected = (w.code === master.supplierWholesale);
                            rowWholesalerOptions += `<option value="${w.code}" ${isSelected ? 'selected' : ''}>${w.name}</option>`;
                        });

                        let actionCellHTML = '';
                        if (isProvisional) {
                            actionCellHTML = '<td><span style="color: red; font-weight: bold;">発注不可</span></td>';
                        } else {
                            actionCellHTML = '<td><button class="remove-order-item-btn btn">除外</button></td>';
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
                        // ▲▲▲ [修正ここまで] ▲▲▲
                    }
                });
            }
        });
        html += `</tbody></table>`;
    });
    container.innerHTML = html;
}

export function initOrders() {
    const view = document.getElementById('order-view');
    if (!view) return;

    const runBtn = document.getElementById('generate-order-candidates-btn');
    const outputContainer = document.getElementById('order-candidates-output');
    const startDateInput = document.getElementById('order-startDate');
    const endDateInput = document.getElementById('order-endDate');
    const kanaNameInput = document.getElementById('order-kanaName');
    const dosageFormInput = document.getElementById('order-dosageForm');
    const coefficientInput = document.getElementById('order-reorder-coefficient');
    const createCsvBtn = document.getElementById('createOrderCsvBtn');
    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, 1);
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);

    runBtn.addEventListener('click', async () => {
        window.showLoading();
        const params = new URLSearchParams({
            startDate: startDateInput.value.replace(/-/g, ''),
            endDate: endDateInput.value.replace(/-/g, ''),
            kanaName: hiraganaToKatakana(kanaNameInput.value),
            dosageForm: dosageFormInput.value,
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
            // ▼▼▼ [ここから修正] 発注不可の行をスキップする処理を追加 ▼▼▼
            if (row.classList.contains('provisional-order-item')) {
                return; // この行はスキップ
            }
            // ▲▲▲ [修正ここまで] ▲▲▲

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
        if (e.target.classList.contains('remove-order-item-btn')) {
            const row = e.target.closest('tr');
            const tbody = row.closest('tbody');
            row.remove();
            if (tbody.children.length === 0) {
                const header = tbody.closest('table').previousElementSibling;
                header.remove();
                tbody.closest('table').remove();
            }
        }
    });
}