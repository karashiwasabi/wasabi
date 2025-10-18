// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\valuation.js

import { hiraganaToKatakana, getLocalDateString } from './utils.js';
import { showModal } from './inout_modal.js';

let view, dateInput, runBtn, outputContainer, kanaNameInput, dosageFormInput, exportBtn, pdfExportBtn;
let reportDataCache = null;
let fullPricingData = [];
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

        const sortedRows = group.detailRows.sort((a, b) => {
            const masterA = fullPricingData.find(m => m.productCode === a.productCode);
            const masterB = fullPricingData.find(m => m.productCode === b.productCode);
            const kanaA = (masterA && masterA.kanaName) ? masterA.kanaName : a.productName;
            const kanaB = (masterB && masterB.kanaName) ? masterB.kanaName : b.productName;
            return kanaA.localeCompare(kanaB, 'ja');
        });

        sortedRows.forEach(row => {
            let warningHtml = '';
            if (row.showAlert) {
                warningHtml = `<span class="warning-link" data-yj-code="${row.yjCode}" data-product-name="${row.productName}" data-provisional-code="${row.productCode}" style="color: red; font-weight: bold; cursor: pointer; text-decoration: underline; margin-left: 15px;">[JCSHMS登録]</span>`;
            }
            
            html += `
                <div class="item-row" style="padding: 8px 12px; border: 1px solid #ccc; border-top: none; display: flex; justify-content: space-between; align-items: center; line-height: 1.6;">
                    <div style="flex: 1; min-width: 0; display: flex; align-items: center; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                        <span style="font-weight: bold;">${row.productName}</span>
                        <span style="margin-left: 10px; overflow: hidden; text-overflow: ellipsis;">${row.packageSpec}</span>
                        ${warningHtml}
                    </div>
                    <div style="white-space: nowrap; text-align: right; padding-left: 20px;">
                        <span style="display: inline-block; min-width: 180px;">在庫: <span style="font-weight: bold;">${row.stock.toFixed(2)} ${row.yjUnitName}</span></span>
                        <span style="display: inline-block; min-width: 180px;">納入価金額: ${formatCurrency(row.totalPurchaseValue)}</span>
                        <span style="display: inline-block; min-width: 180px;">薬価金額: ${formatCurrency(row.totalNhiValue)}</span>
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
        html += `<h3 style="font-size: 12pt; padding: 10px 0; page-break-before: auto; border-bottom: 1px solid #666;">${ucName}</h3>
            <table class="data-table" style="font-size: 10pt; width: 100%; border-collapse: collapse;">
                    <thead>
                        <tr>
                            <th style="width: 35%; text-align: left; padding: 4px;">製品名</th>
                            <th style="width: 25%; text-align: left; padding: 4px;">包装</th>
                            <th style="width: 10%; text-align: right; padding: 4px;">在庫数</th>
                            <th style="width: 15%; text-align: right; padding: 4px;">薬価金額</th>
                            <th style="width: 15%; text-align: right; padding: 4px;">納入価金額</th>
                        </tr>
                    </thead>
                    <tbody>`;
        group.detailRows.forEach(row => {
            html += `
                <tr>
                    <td class="left" style="padding: 4px;">${row.productName}</td>
                    <td class="left" style="padding: 4px;">${row.packageSpec}</td>
                    <td class="right" style="padding: 4px;">${row.stock.toFixed(2)} ${row.yjUnitName}</td>
                    <td class="right" style="padding: 4px;">${formatCurrency(row.totalNhiValue)}</td>
                    <td class="right" style="padding: 4px;">${formatCurrency(row.totalPurchaseValue)}</td>
                </tr>`;
        });
        html += `</tbody>
                 <tfoot>
                    <tr style="border-top: 1px solid #666;">
                        <td colspan="3" class="right" style="font-weight: bold; padding: 4px;">${ucName} 合計</td>
                        <td class="right" style="font-weight: bold; padding: 4px;">${formatCurrency(group.totalNhiValue)}</td>
                        <td class="right" style="font-weight: bold; padding: 4px;">${formatCurrency(group.totalPurchaseValue)}</td>
                    </tr>
                 </tfoot>
                </table>`;
    });

    html += `
        <table class="data-table" style="margin-top: 20px; width: 100%;">
            <tfoot>
                <tr style="border-top: 2px solid black;">
                    <td colspan="3" class="right" style="font-weight: bold; padding: 8px;">総合計</td>
                    <td class="right" style="font-weight: bold; padding: 8px; width: 15%;">${formatCurrency(grandTotalNhiValue)}</td>
                    <td class="right" style="font-weight: bold; padding: 8px; width: 15%;">${formatCurrency(grandTotalPurchaseValue)}</td>
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
        const masterRes = await fetch('/api/pricing/all_masters');
        if(!masterRes.ok) console.warn('ソート用のマスターデータ取得に失敗しました。');
        fullPricingData = await masterRes.json();

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

function handlePdfExport() {
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
    // ▼▼▼【ここが修正点です】 'window.location.href' から 'window.open' に変更 ▼▼▼
    window.open(`/api/valuation/export_pdf?${params.toString()}`, '_blank');
    // ▲▲▲【修正ここまで】▲▲▲
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
    pdfExportBtn = document.getElementById('export-valuation-pdf-btn');

    dateInput.value = getLocalDateString();
    runBtn.addEventListener('click', runCalculation);
    exportBtn.addEventListener('click', handleExport);
    pdfExportBtn.addEventListener('click', handlePdfExport);
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