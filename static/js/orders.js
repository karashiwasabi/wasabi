// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\orders.js

import { hiraganaToKatakana, getLocalDateString } from './utils.js';

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
                        // 「発注不可」の条件を仮マスターとマスター設定の両方で判定
                        const isProvisional = master.productCode.startsWith('99999') && master.productCode.length > 13;
                        const isOrderStopped = master.isOrderStopped === 1;
                        const isOrderable = !isProvisional && !isOrderStopped;

                        const rowClass = !isOrderable ? 'provisional-order-item' : ''; // 発注不可品にはスタイルを適用
                        const disabledAttr = !isOrderable ? 'disabled' : '';

                        const recommendedOrder = master.yjPackUnitQty > 0 ? Math.ceil(pkgShortfall / master.yjPackUnitQty) : 0;
                        
                        let rowWholesalerOptions = '<option value="">--- 選択 ---</option>';
                        wholesalers.forEach(w => {
                            const isSelected = (w.code === master.supplierWholesale);
                            rowWholesalerOptions += `<option value="${w.code}" ${isSelected ? 'selected' : ''}>${w.name}</option>`;
                        });

                        // 「操作」列のボタンを条件によって変更
                        let actionCellHTML = '';
                        if (isOrderable) {
                            actionCellHTML = '<td><button class="remove-order-item-btn btn">除外</button></td>';
                        } else {
                            // 発注不可品には「発注に変更」ボタンを表示
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
        html += `</tbody></table>`;
    });
    container.innerHTML = html;
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

    runBtn.addEventListener('click', async () => {
        window.showLoading();
        const params = new URLSearchParams({
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

    // ▼▼▼【ここから修正】▼▼▼
    outputContainer.addEventListener('click', (e) => {
        const target = e.target;
        
        if (target.classList.contains('change-to-orderable-btn')) {
            const row = target.closest('tr');
            if (row) {
                // スタイルとdisabled属性を解除
                row.classList.remove('provisional-order-item');
                row.querySelector('.wholesaler-select').disabled = false;
                row.querySelector('.order-quantity-input').disabled = false;

                // ボタンを「除外」ボタンに変更
                target.textContent = '除外';
                target.classList.remove('change-to-orderable-btn');
                target.classList.add('remove-order-item-btn');
            }
        } else if (target.classList.contains('remove-order-item-btn')) {
            const row = target.closest('tr');
            const tbody = row.closest('tbody');
            row.remove();
            if (tbody.children.length === 0) {
                const header = tbody.closest('table').previousElementSibling;
                header.remove();
                tbody.closest('table').remove();
            }
        }
    });
    // ▲▲▲【修正ここまで】▲▲▲
}