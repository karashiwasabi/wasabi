import { showModal } from './inout_modal.js';

let view, dateInput, runBtn, outputContainer;
let reportDataCache = null; // 計算結果をキャッシュする変数

// 金額を日本円形式にフォーマットするヘルパー関数
const formatCurrency = (value) => new Intl.NumberFormat('ja-JP', { style: 'currency', currency: 'JPY' }).format(value || 0);

/**
 * 操作画面（YJコード単位の集計）を描画する
 */
function renderInteractiveView() {
    if (!reportDataCache) {
        outputContainer.innerHTML = '<p>表示するデータがありません。</p>';
        return;
    }

    let html = '';
    const ucMap = {"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"};

    reportDataCache.forEach(group => {
        const ucName = ucMap[group.usageClassification.trim()] || group.usageClassification;
        html += `<div class="agg-yj-header">${ucName}</div>`;

        group.yjGroups.forEach(yj => {
            let warningHtml = '';
            // 警告表示の条件判定
            if (yj.containsOnlyNonJcshms && !['機', '他'].includes(ucName)) {
                warningHtml = `<span class="warning-link" data-yj-code="${yj.yjCode}" data-product-name="${yj.productName}">[JCSHMS掲載品を登録してください]</span>`;
            }

            html += `
                <div class="item-row" style="background-color: #f8f9fa; padding: 6px 10px; border: 1px solid #ccc; border-top: none; display: flex; justify-content: space-between; align-items: center;">
                    <div>
                        <span style="font-weight: bold;">${yj.productName}</span>
                        <span style="font-size: 12px; margin-left: 10px;">(${yj.yjCode})</span>
                        ${warningHtml}
                    </div>
                    <div style="font-size: 12px;">
                        <span>在庫: ${yj.totalYjStock.toFixed(2)} ${yj.yjUnitName}</span> |
                        <span>薬価金額: ${formatCurrency(yj.nhiValue)}</span> |
                        <span>納入価金額: ${formatCurrency(yj.purchaseValue)}</span>
                    </div>
                </div>
            `;
        });
    });

    html += `<div style="text-align: right; margin-top: 20px;"><button id="generate-report-btn" class="btn" style="background-color: #198754; color: white;">最終帳票を作成</button></div>`;
    outputContainer.innerHTML = html;
}

/**
 * 印刷用の最終帳票を描画する
 */
function renderPrintableReport() {
    const dateStr = new Date(dateInput.value).toLocaleDateString('ja-JP-u-ca-japanese', { year: 'numeric', month: 'long', day: 'numeric' });
    let html = `
        <div style="text-align: right; margin-bottom: 10px;">
            <button id="print-report-btn" class="btn">この帳票を印刷</button>
        </div>
        <div id="printable-area">
            <h2 style="text-align: center; margin-bottom: 20px;">${dateStr} 在庫一覧</h2>
    `;
    const ucMap = {"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"};
    
    reportDataCache.forEach(group => {
        const ucName = ucMap[group.usageClassification.trim()] || group.usageClassification;
        html += `<h3 style="font-size: 12pt; padding: 10px 0;">${ucName}</h3>
                 <table class="data-table" style="font-size: 10pt;">
                    <thead>
                        <tr>
                            <th style="width: 5%;">No.</th>
                            <th style="width: 45%;">製品名</th>
                            <th style="width: 10%;">在庫数</th>
                            <th style="width: 10%;">単位</th>
                            <th style="width: 15%;">薬価金額</th>
                            <th style="width: 15%;">納入価金額</th>
                        </tr>
                    </thead>
                    <tbody>`;
        
        group.yjGroups.forEach((yj, index) => {
            html += `
                <tr>
                    <td style="text-align: center;">${index + 1}</td>
                    <td class="left">${yj.productName}</td>
                    <td class="right">${yj.totalYjStock.toFixed(2)}</td>
                    <td style="text-align: center;">${yj.yjUnitName}</td>
                    <td class="right">${formatCurrency(yj.nhiValue)}</td>
                    <td class="right">${formatCurrency(yj.purchaseValue)}</td>
                </tr>`;
        });

        html += `</tbody>
                 <tfoot>
                    <tr>
                        <td colspan="4" class="right" style="font-weight: bold;">${ucName} 合計</td>
                        <td class="right" style="font-weight: bold;">${formatCurrency(group.totalNhiValue)}</td>
                        <td class="right" style="font-weight: bold;">${formatCurrency(group.totalPurchaseValue)}</td>
                    </tr>
                 </tfoot>
                 </table>`;
    });

    html += `</div>`;
    outputContainer.innerHTML = html;
}


/**
 * 計算実行ボタンの処理
 */
async function runCalculation() {
    const date = dateInput.value.replace(/-/g, '');
    if (!date) {
        window.showNotification('評価基準日を指定してください。', 'error');
        return;
    }
    window.showLoading();
    try {
        const res = await fetch(`/api/valuation?date=${date}`);
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

export function initValuationView() {
    view = document.getElementById('valuation-view');
    if (!view) return;

    dateInput = document.getElementById('valuation-date');
    runBtn = document.getElementById('run-valuation-btn');
    outputContainer = document.getElementById('valuation-output-container');

    dateInput.value = new Date().toISOString().slice(0, 10);

    runBtn.addEventListener('click', runCalculation);

    outputContainer.addEventListener('click', async (e) => {
        // 警告リンククリック時の処理
        if (e.target.classList.contains('warning-link')) {
            const target = e.target;
            const yjCode = target.dataset.yjCode;
            const productName = target.dataset.productName;
            
            showModal(target, async (selectedProduct) => {
                window.showLoading();
                try {
                    // JCSHMSから取得したデータを手動登録マスターと同じ形式で保存
                    const payload = { ...selectedProduct, origin: 'JCSHMS' };
                    const res = await fetch('/api/master/update', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload),
                    });
                    const resData = await res.json();
                    if (!res.ok) throw new Error(resData.message || 'マスターの登録に失敗しました。');
                    
                    window.showNotification(`「${selectedProduct.productName}」をマスターに登録しました。在庫評価を再計算します。`, 'success');
                    await runCalculation(); // 再計算して画面を更新
                } catch (err) {
                    window.showNotification(err.message, 'error');
                } finally {
                    window.hideLoading();
                }
            });
        }

        // 帳票作成ボタンクリック時の処理
        if (e.target.id === 'generate-report-btn') {
            renderPrintableReport();
        }

        // 印刷ボタンクリック時の処理
        if (e.target.id === 'print-report-btn') {
            window.print();
        }
    });
}