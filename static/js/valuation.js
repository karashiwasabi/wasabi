// C:\Dev\WASABI\static\js\valuation.js
import { hiraganaToKatakana } from './utils.js';
import { showModal } from './inout_modal.js';
let view, dateInput, runBtn, outputContainer, kanaNameInput, dosageFormInput, exportBtn;
let reportDataCache = null;
const formatCurrency = (value) => new Intl.NumberFormat('ja-JP', { style: 'currency', currency: 'JPY' }).format(value || 0);

function renderInteractiveView() {
    if (!reportDataCache || reportDataCache.length === 0) {
        outputContainer.innerHTML = '<p>表示するデータがありません。</p>';
        return;
    }

    let html = '';
    let grandTotalNhiValue = 0;
    let grandTotalPurchaseValue = 0;
    const ucMap = {"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"};
    reportDataCache.forEach(group => {
        grandTotalNhiValue += group.totalNhiValue;
        grandTotalPurchaseValue += group.totalPurchaseValue;

        const ucName = ucMap[group.usageClassification.trim()] || group.usageClassification;
        html += `<div class="agg-yj-header">${ucName} (合計薬価: ${formatCurrency(group.totalNhiValue)} | 合計納入価: ${formatCurrency(group.totalPurchaseValue)})</div>`;

        group.detailRows.forEach(row => {
            let warningHtml = '';
            if (row.showAlert) { 
                warningHtml = `<span class="warning-link" data-yj-code="${row.yjCode}" data-product-name="${row.productName}" data-provisional-code="${row.productCode}" style="color: red; font-weight: bold; cursor: pointer; text-decoration: underline; margin-left: 15px;">[JCSHMS掲載品を登録してください]</span>`;
            }

            html += `
                <div class="item-row" style="background-color: #f8f9fa; padding: 6px 10px; border: 1px solid #ccc; border-top: none; font-size: 12px; line-height: 1.6; display: flex; justify-content: space-between; align-items: center;">
                    <div class="left-section">
                        <span style="font-weight: bold;">${row.productName} (${row.yjCode})</span>
                        <span style="margin-left: 10px; color: #555;">${row.packageSpec}</span>
                        ${warningHtml} 
                    </div>
                    <div class="right-section" style="white-space: nowrap;">
                        <span>納入価金額: ${formatCurrency(row.totalPurchaseValue)}</span> | 
                        <span>薬価金額: ${formatCurrency(row.totalNhiValue)}</span> | 
                        <span>包装納入価: ${row.packagePurchasePrice > 0 ? row.packagePurchasePrice.toFixed(2) + '円' : '-'}</span> |
                        <span>包装薬価: ${row.packageNhiPrice > 0 ? row.packageNhiPrice.toFixed(2) + '円' : '-'}</span> |
                        <span style="font-weight: bold;">在庫: ${row.stock.toFixed(2)} ${row.yjUnitName}</span>
                    </div>
                </div>
            `;
        });
    });
    
    html += `
        <div style="text-align: right; margin-top: 20px; padding: 10px; border-top: 2px solid #333; font-weight: bold;">
            <span>総合計 (薬価): ${formatCurrency(grandTotalNhiValue)}</span> | 
            <span>総合計 (納入価): ${formatCurrency(grandTotalPurchaseValue)}</span>
        </div>
    `;

    html += `<div style="text-align: right; margin-top: 20px;"><button id="generate-report-btn" class="btn" style="background-color: #198754; color: white;">最終帳票を作成</button></div>`;
    outputContainer.innerHTML = html;
}

function renderPrintableReport() {
    const date = new Date(dateInput.value);
    const dateStr = `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`;
    
    let html = `
        <div id="valuation-print-controls" style="text-align: right; margin-bottom: 10px;">
            <button id="print-valuation-report-btn" class="btn">この帳票を印刷</button>
        </div>
        <div id="printable-area">
            <h2 style="text-align: center; margin-bottom: 20px;">${dateStr} 在庫評価一覧</h2>
    `;
    
    let grandTotalNhiValue = 0;
    let grandTotalPurchaseValue = 0;
    const ucMap = {"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"};

    reportDataCache.forEach(group => {
        grandTotalNhiValue += group.totalNhiValue;
        grandTotalPurchaseValue += group.totalPurchaseValue;

        const ucName = ucMap[group.usageClassification.trim()] || group.usageClassification;
        html += `<h3 style="font-size: 12pt; padding: 10px 0; page-break-before: auto;">${ucName}</h3>
                 <table class="data-table" style="font-size: 10pt;">
                    <thead>
                        <tr>
                            <th style="width: 35%;">製品名</th>
                            <th style="width: 25%;">包装</th>
                            <th style="width: 10%;">在庫数</th>
                            <th style="width: 15%;">薬価金額</th>
                            <th style="width: 15%;">納入価金額</th>
                        </tr>
                    </thead>
                    <tbody>`;
        group.detailRows.forEach(row => {
            html += `
                <tr>
                    <td class="left">${row.productName}</td>
                    <td class="left">${row.packageSpec}</td>
                    <td class="right">${row.stock.toFixed(2)} ${row.yjUnitName}</td>
                    <td class="right">${formatCurrency(row.totalNhiValue)}</td>
                    <td class="right">${formatCurrency(row.totalPurchaseValue)}</td>
                </tr>`;
        });
        html += `</tbody>
                 <tfoot>
                    <tr>
                        <td colspan="3" class="right" style="font-weight: bold;">${ucName} 合計</td>
                        <td class="right" style="font-weight: bold;">${formatCurrency(group.totalNhiValue)}</td>
                        <td class="right" style="font-weight: bold;">${formatCurrency(group.totalPurchaseValue)}</td>
                    </tr>
                 </tfoot>
                 </table>`;
    });

    html += `
        <table class="data-table" style="margin-top: 20px;">
            <tfoot>
                <tr>
                    <td colspan="3" class="right" style="font-weight: bold;">総合計</td>
                    <td class="right" style="font-weight: bold;">${formatCurrency(grandTotalNhiValue)}</td>
                    <td class="right" style="font-weight: bold;">${formatCurrency(grandTotalPurchaseValue)}</td>
                </tr>
            </tfoot>
        </table>
    `;
    html += `</div>`;
    outputContainer.innerHTML = html;
}

async function runCalculation() {
    const date = dateInput.value.replace(/-/g, '');
    if (!date) {
        window.showNotification('評価基準日を指定してください。', 'error');
        return;
    }
    window.showLoading();
    try {
        const kanaName = hiraganaToKatakana(kanaNameInput.value);
        const dosageForm = dosageFormInput.value;
        const params = new URLSearchParams({
            date: date,
            kanaName: kanaName,
            dosageForm: dosageForm,
        });
        const res = await fetch(`/api/valuation?${params.toString()}`);
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || '在庫評価の計算に失敗しました。');
        }
        reportDataCache = await res.json();
        renderInteractiveView();
    } catch (err) {
        outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

function handleExport() {
    const date = dateInput.value.replace(/-/g, '');
    if (!date) {
        window.showNotification('評価基準日を指定してください。', 'error');
        return;
    }
    const kanaName = hiraganaToKatakana(kanaNameInput.value);
    const dosageForm = dosageFormInput.value;
    const params = new URLSearchParams({
        date: date,
        kanaName: kanaName,
        dosageForm: dosageForm,
    });
    window.location.href = `/api/valuation/export?${params.toString()}`;
}

export function initValuationView() {
    view = document.getElementById('valuation-view');
    if (!view) return;

    dateInput = document.getElementById('valuation-date');
    runBtn = document.getElementById('run-valuation-btn');
    outputContainer = document.getElementById('valuation-output-container');
    kanaNameInput = document.getElementById('val-kanaName');
    dosageFormInput = document.getElementById('val-dosageForm');
    exportBtn = document.getElementById('export-valuation-btn');

    dateInput.value = new Date().toISOString().slice(0, 10);
    runBtn.addEventListener('click', runCalculation);
    exportBtn.addEventListener('click', handleExport);

    outputContainer.addEventListener('click', async (e) => {
        if (e.target.classList.contains('warning-link')) {
            showModal(e.target, async (selectedProduct) => {
                window.showLoading();
                try {
                    const payload = { ...selectedProduct, origin: 'JCSHMS' };
                    const resMaster = await fetch('/api/master/update', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload),
                    });
                    const resMasterData = await resMaster.json();
                    if (!resMaster.ok) throw new Error(resMasterData.message || 'マスターの登録に失敗しました。');
                    
                    window.showNotification(`「${selectedProduct.productName}」を登録しました。在庫評価を更新します。`, 'success');

                    await runCalculation();
                } catch (err) {
                    window.showNotification(err.message, 'error');
                } finally {
                    window.hideLoading();
                }
            });
        }
        
        if (e.target.id === 'generate-report-btn') {
            renderPrintableReport();
        }
        
        if (e.target.id === 'print-valuation-report-btn') {
            document.getElementById('aggregation-view').classList.remove('print-this-view');
            view.classList.add('print-this-view');
            window.print();
        }
    });

    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });
}