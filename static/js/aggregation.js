// C:\Dev\WASABI\static\js\aggregation.js

import { transactionTypeMap, createUploadTableHTML, renderUploadTableRows } from './common_table.js';
let view, runBtn, printBtn, outputContainer, startDateInput, endDateInput, kanaNameInput, dosageFormInput, coefficientInput, drugTypeCheckboxes, reorderNeededCheckbox;
let lastData = []; // サーバーから受け取った元のデータを保持

/**
 * 在庫数をフォーマットするヘルパー関数
 * @param {number | string} balance - 在庫数
 * @returns {string} 表示用にフォーマットされた文字列
 */
function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    return balance;
}

function renderResults() {
    let dataToRender = lastData;
    if (reorderNeededCheckbox.checked) {
        dataToRender = lastData.filter(yjGroup => yjGroup.isReorderNeeded)
            .map(yjGroup => ({
                ...yjGroup,
                packageLedgers: yjGroup.packageLedgers.filter(pkg => pkg.isReorderNeeded)
            }));
    }

    if (!dataToRender || dataToRender.length === 0) {
        outputContainer.innerHTML = "<p>対象データが見つかりませんでした。</p>";
        return;
    }

    let html = '';
    dataToRender.forEach((yjGroup, yjIndex) => {
        let yjReorderPointText = formatBalance(yjGroup.totalReorderPoint);
        if (yjGroup.totalPrecompounded > 0) {
            yjReorderPointText = `${formatBalance(yjGroup.totalBaseReorderPoint)} + 予${formatBalance(yjGroup.totalPrecompounded)} = ${formatBalance(yjGroup.totalReorderPoint)}`;
        }
        html += `
            <div class="agg-yj-header" ${yjGroup.isReorderNeeded ? 'style="background-color: #ff0015ff;"' : ''}>
                <span>YJ: ${yjGroup.yjCode}</span>
                <span class="product-name">${yjGroup.productName}</span>
                <span class="balance-info">
                    在庫: ${formatBalance(yjGroup.endingBalance)} | 
                    発注点: ${yjReorderPointText} | 
                    変動: ${formatBalance(yjGroup.netChange)}
                </span>
            </div>
        `;
     
        yjGroup.packageLedgers.forEach((pkg, pkgIndex) => {
            const tableId = `agg-table-${yjIndex}-${pkgIndex}`;
            let pkgReorderPointText = formatBalance(pkg.reorderPoint);
            if (pkg.precompoundedTotal > 0) {
                pkgReorderPointText = `${formatBalance(pkg.baseReorderPoint)} + 予${formatBalance(pkg.precompoundedTotal)} = ${formatBalance(pkg.reorderPoint)}`;
            }
            // ▼▼▼ [修正点] ご指示の表示形式に修正 ▼▼▼
            html += `
                <div class="agg-pkg-header" ${pkg.isReorderNeeded ? 'style="background-color: #ff0000ff;"' : ''}>
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.endingBalance)} | 在庫(発注残含): ${formatBalance(pkg.effectiveEndingBalance)} | 
                        発注点: ${pkgReorderPointText} |  
                        変動: ${formatBalance(pkg.netChange)}
                    </span>
                </div>
                <div id="${tableId}-container"></div>`;
            // ▲▲▲ 修正ここまで ▲▲▲
        });
    });
    outputContainer.innerHTML = html;
    dataToRender.forEach((yjGroup, yjIndex) => {
        yjGroup.packageLedgers.forEach((pkg, pkgIndex) => {
            const tableId = `agg-table-${yjIndex}-${pkgIndex}`;
            const container = document.getElementById(`${tableId}-container`);
            if (container) {
                container.innerHTML = createUploadTableHTML(tableId);
                renderUploadTableRows(tableId, pkg.transactions);
            }
        });
    });
}

export function initAggregation() {
    view = document.getElementById('aggregation-view');
    if (!view) return;
    runBtn = document.getElementById('run-aggregation-btn');
    printBtn = document.getElementById('print-aggregation-btn');
    outputContainer = document.getElementById('aggregation-output-container');
    startDateInput = document.getElementById('startDate');
    endDateInput = document.getElementById('endDate');
    kanaNameInput = document.getElementById('agg-kanaName');
    dosageFormInput = document.getElementById('agg-dosageForm');
    coefficientInput = document.getElementById('reorder-coefficient');
    drugTypeCheckboxes = document.querySelectorAll('input[name="drugType"]');
    reorderNeededCheckbox = document.getElementById('reorder-needed-filter');
    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, today.getDate());
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);

    // ▼▼▼ [修正点] 印刷ボタンのイベントリスナーを修正 ▼▼▼
    printBtn.addEventListener('click', () => {
        if (lastData && lastData.length > 0) {
             // 他のビューから印刷用クラスを削除
            document.getElementById('valuation-view').classList.remove('print-this-view');
            // 集計ビューに印刷用クラスを追加
            view.classList.add('print-this-view');
            window.print();
        } else {
            window.showNotification('先に「集計実行」を押してデータを表示してください。', 'error');
        }
    });

    // 印刷後にクラスを削除する後処理
    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });
    // ▲▲▲ 修正ここまで ▲▲▲

    reorderNeededCheckbox.addEventListener('change', () => renderResults());
    
    runBtn.addEventListener('click', async () => {
        window.showLoading();

        const selectedDrugTypes = Array.from(drugTypeCheckboxes)
            .filter(cb => cb.checked)
            .map(cb => cb.value)
            .join(',');

        const params = new URLSearchParams({
            startDate: startDateInput.value.replace(/-/g, ''),
            endDate: endDateInput.value.replace(/-/g, ''),
            kanaName: kanaNameInput.value,
            dosageForm: dosageFormInput.value,
            coefficient: coefficientInput.value,
            drugTypes: selectedDrugTypes,
        });

        try {
            const res = await fetch(`/api/aggregation?${params.toString()}`);
            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || 'Aggregation failed');
            }
            lastData = await res.json();
            renderResults();
        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    });
}