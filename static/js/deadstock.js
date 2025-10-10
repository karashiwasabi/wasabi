// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\deadstock.js

import { hiraganaToKatakana, getLocalDateString } from './utils.js';
import { showModal } from './inout_modal.js';

let view, outputContainer, excludeZeroStockCheckbox, createCsvBtn, kanaNameInput, dosageFormInput, importBtn;
let unitMap = {};

async function fetchUnitMap() {
    if (Object.keys(unitMap).length > 0) return;
    try {
        const res = await fetch('/api/units/map');
        if (!res.ok) throw new Error('単位マスタの取得に失敗');
        unitMap = await res.json();
    } catch (err) {
        console.error(err);
    }
}

// 包装仕様の文字列を生成するヘルパー関数
function formatPackageSpec(master) {
    if (!master) return '';
    const yjUnitName = master.yjUnitName || '';
    let spec = `${master.packageForm || ''} ${master.yjPackUnitQty || 0}${yjUnitName}`;

    if (master.janPackInnerQty > 0 && master.janPackUnitQty > 0) {
        const janUnitName = (master.janUnitCode === 0 || !unitMap[master.janUnitCode]) 
            ? yjUnitName 
            : (unitMap[master.janUnitCode] || yjUnitName);
        spec += ` (${master.janPackInnerQty}${yjUnitName}×${master.janPackUnitQty}${janUnitName})`;
    }
    return spec;
}

function renderDeadStockList(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>対象データが見つかりませんでした。</p>";
        return;
    }

    let html = data.map(group => {
        const yjHeader = `<div class="yj-group-wrapper">
            <div class="agg-yj-header">
                <div style="flex-grow: 1;">
                    <span>YJ: ${group.yjCode}</span>
                    <span class="product-name">${group.productName}</span>
                </div>
                <button class="btn inventory-adjust-link-btn" data-yj-code="${group.yjCode}">棚卸調整</button>
            </div>`;

        let tableRowsHTML = '';
        group.packageGroups.forEach(pkg => {
            pkg.products.forEach(prod => {
                const savedRecords = prod.savedRecords || [];
                const rowSpanCount = savedRecords.length > 0 ? savedRecords.length : 1;
                const rowSpan = `rowspan="${rowSpanCount}"`;

                const janUnitName = (prod.janUnitCode === 0 || !unitMap[prod.janUnitCode]) 
                    ? prod.yjUnitName 
                    : (unitMap[prod.janUnitCode] || prod.yjUnitName);
                
                const spec = formatPackageSpec(prod);
                const lastUsed = prod.lastUsageDate ? `${prod.lastUsageDate.slice(0,4)}-${prod.lastUsageDate.slice(4,6)}-${prod.lastUsageDate.slice(6,8)}` : '使用履歴なし';
                tableRowsHTML += `
                    <tr>
                        <td class="left" ${rowSpan}>${prod.productName}</td>
                        <td ${rowSpan}>${prod.productCode}</td>
                        <td class="left" ${rowSpan}>${prod.makerName}</td>
                        <td class="left" ${rowSpan}>${spec}</td>
                        <td class="right" ${rowSpan}>${prod.currentStock.toFixed(2)} ${janUnitName}</td>
                        <td ${rowSpan}>${lastUsed}</td>`;

                if (savedRecords.length > 0) {
                    const firstRec = savedRecords[0];
                    tableRowsHTML += `
                        <td class="right">${(firstRec.stockQuantityJan || 0).toFixed(2)}</td>
                        <td>${firstRec.expiryDate || ''}</td>
                        <td class="left">${firstRec.lotNumber || ''}</td>
                    </tr>`;
                } else {
                    tableRowsHTML += `
                        <td colspan="3" style="text-align:center; color: #888;">-</td>
                    </tr>`;
                }

                for (let i = 1; i < savedRecords.length; i++) {
                    const rec = savedRecords[i];
                    tableRowsHTML += `
                        <tr>
                            <td class="right">${(rec.stockQuantityJan || 0).toFixed(2)}</td>
                            <td>${rec.expiryDate || ''}</td>
                            <td class="left">${rec.lotNumber || ''}</td>
                        </tr>`;
                }
            });
        });
        
        const tableHTML = `<table class="data-table" style="margin-top: 5px;">
            <thead>
                <tr>
                    <th style="width: 20%;">製品名</th>
                    <th style="width: 12%;">JANコード</th>
                    <th style="width: 12%;">メーカー</th>
                    <th style="width: 18%;">包装仕様</th>
                    <th style="width: 8%;">在庫数</th>
                    <th style="width: 10%;">最終使用日</th>
                    <th style="width: 7%;">在庫</th>
                    <th style="width: 7%;">使用期限</th>
                    <th style="width: 6%;">ロット番号</th>
                </tr>
            </thead>
            <tbody>${tableRowsHTML}</tbody>
        </table></div>`;
        
        return yjHeader + tableHTML;
    }).join('');

    outputContainer.innerHTML = html;
}

export async function initDeadStock() {
    await fetchUnitMap();
    view = document.getElementById('deadstock-view');
    if (!view) return;

    outputContainer = document.getElementById('deadstock-output-container');
    excludeZeroStockCheckbox = document.getElementById('ds-exclude-zero-stock');
    createCsvBtn = document.getElementById('create-deadstock-csv-btn');
    kanaNameInput = document.getElementById('ds-kanaName');
    dosageFormInput = document.getElementById('ds-dosageForm');
    importBtn = document.getElementById('import-deadstock-btn');
    const printBtn = document.getElementById('print-deadstock-btn');
    const printArea = document.getElementById('deadstock-print-area');
    // ▼▼▼【ここから追加】▼▼▼
    const importDeadstockInput = document.getElementById('importDeadstockInput');
    // ▲▲▲【追加ここまで】▲▲▲

    if (printBtn) {
        printBtn.addEventListener('click', () => {
            const originalContent = outputContainer.querySelector('.yj-group-wrapper');
            if (!originalContent) {
                window.showNotification('印刷するデータがありません。先にリストを作成してください。', 'error');
                return;
            }

            const style = document.createElement('style');
            style.innerHTML = '@page { size: A4 portrait; margin: 1cm; }';
            style.id = 'print-style-portrait';
            document.head.appendChild(style);

            const contentToPrint = outputContainer.cloneNode(true);
            contentToPrint.querySelectorAll('button').forEach(btn => btn.remove());
            
            printArea.innerHTML = `
                <h2 style="text-align: center; margin-bottom: 20px;">不動在庫リスト</h2>
                <p style="text-align: right; margin-bottom: 10px;">印刷日: ${new Date().toLocaleDateString()}</p>
                ${contentToPrint.innerHTML}
            `;

            view.classList.add('print-this-view');
            outputContainer.classList.add('hidden');
            printArea.classList.remove('hidden');

            window.print();
        });
    }

    window.addEventListener('afterprint', () => {
        const printStyle = document.getElementById('print-style-portrait');
        if (printStyle) {
            printStyle.remove();
        }

        if (view.classList.contains('print-this-view')) {
            outputContainer.classList.remove('hidden');
            printArea.classList.add('hidden');
            view.classList.remove('print-this-view');
        }
    });
    
    if (outputContainer) {
        outputContainer.addEventListener('click', async (e) => {
            const target = e.target;
            if (target.classList.contains('inventory-adjust-link-btn')) {
                const yjCode = target.dataset.yjCode;
                const event = new CustomEvent('navigateToInventoryAdjustment', {
                    detail: { yjCode: yjCode },
                    bubbles: true
                });
                target.dispatchEvent(event);
            }
        });
    }

    if (view) {
        const runBtn = view.querySelector('#run-dead-stock-btn');
        if (runBtn) {
            runBtn.addEventListener('click', () => {
                window.showLoading();
                const params = new URLSearchParams({
                    excludeZeroStock: excludeZeroStockCheckbox.checked,
                    kanaName: hiraganaToKatakana(kanaNameInput.value),
                    dosageForm: dosageFormInput.value,
                });
                fetch(`/api/deadstock/list?${params.toString()}`)
                    .then(res => {
                        if (!res.ok) { return res.text().then(text => { throw new Error(text || 'Failed to generate dead stock list') }); }
                        return res.json();
                    })
                    .then(data => renderDeadStockList(data))
                    .catch(err => { outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`; })
                    .finally(() => { window.hideLoading(); });
            });
        }
    }

    // ▼▼▼【ここから修正】▼▼▼
    if (createCsvBtn) {
        createCsvBtn.addEventListener('click', () => {
            const params = new URLSearchParams({
                excludeZeroStock: excludeZeroStockCheckbox.checked,
                kanaName: hiraganaToKatakana(kanaNameInput.value),
                dosageForm: dosageFormInput.value,
            });
            window.location.href = `/api/deadstock/export?${params.toString()}`;
        });
    }

    if (importBtn && importDeadstockInput) {
        importBtn.addEventListener('click', () => {
            importDeadstockInput.click();
        });

        importDeadstockInput.addEventListener('change', async (event) => {
            const file = event.target.files[0];
            if (!file) return;

            const formData = new FormData();
            formData.append('file', file);
            
            window.showLoading('CSVファイルをインポート中...');
            try {
                const res = await fetch('/api/deadstock/import', {
                    method: 'POST',
                    body: formData,
                });
                const resData = await res.json();
                if (!res.ok) {
                    throw new Error(resData.message || 'インポートに失敗しました。');
                }
                window.showNotification(resData.message, 'success');
                // インポート後にリストを再表示
                view.querySelector('#run-dead-stock-btn').click();
            } catch (err) {
                console.error(err);
                window.showNotification(`エラー: ${err.message}`, 'error');
            } finally {
                window.hideLoading();
                event.target.value = ''; // ファイル入力をリセット
            }
        });
    }
    // ▲▲▲【修正ここまで】▲▲▲
}