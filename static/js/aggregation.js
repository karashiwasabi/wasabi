// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\aggregation.js

import { hiraganaToKatakana, getLocalDateString } from './utils.js';
import { transactionTypeMap, createUploadTableHTML, renderUploadTableRows } from './common_table.js';

let view, runBtn, printBtn, outputContainer, kanaNameInput, dosageFormInput, coefficientInput, drugTypeCheckboxes, reorderNeededCheckbox, movementOnlyCheckbox;
let lastData = [];

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

    let html = dataToRender.map((yjGroup, yjIndex) => {
        let yjReorderPointText = formatBalance(yjGroup.totalReorderPoint);
        if (yjGroup.totalPrecompounded > 0) {
            yjReorderPointText = `${formatBalance(yjGroup.totalBaseReorderPoint)} + 予${formatBalance(yjGroup.totalPrecompounded)} = ${formatBalance(yjGroup.totalReorderPoint)}`;
        }
        
        const yjHeader = `
            <div class="agg-yj-header" ${yjGroup.isReorderNeeded ? 'style="background-color: #ff0015ff; color: white;"' : ''}>
                <div style="flex-grow: 1;">
                    <span>YJ: ${yjGroup.yjCode}</span>
                    <span class="product-name">${yjGroup.productName}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(yjGroup.endingBalance)} | 
                        発注点: ${yjReorderPointText} | 
                        変動: ${formatBalance(yjGroup.netChange)}
                    </span>
                </div>
                <button class="btn inventory-adjust-link-btn" data-yj-code="${yjGroup.yjCode}" style="margin-left: 15px; background-color: #ffc107; color: black;">棚卸調整</button>
            </div>
        `;
     
        const packagesHtml = yjGroup.packageLedgers.map((pkg, pkgIndex) => {
            const tableId = `agg-table-${yjIndex}-${pkgIndex}`;
            let pkgReorderPointText = formatBalance(pkg.reorderPoint);
            if (pkg.precompoundedTotal > 0) {
                pkgReorderPointText = `${formatBalance(pkg.baseReorderPoint)} + 予${formatBalance(pkg.precompoundedTotal)} = ${formatBalance(pkg.reorderPoint)}`;
            }

            const pkgHeader = `
                <div class="agg-pkg-header">
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.endingBalance)} |
                        在庫(発注残含): ${formatBalance(pkg.effectiveEndingBalance)} | 
                        発注点: ${pkgReorderPointText} |  
                        変動: ${formatBalance(pkg.netChange)}
                    </span>
                </div>
            `;
            const tableShell = createUploadTableHTML(tableId);
            const tableBodyContent = renderUploadTableRows(pkg.transactions);
            
            const fullTableHtml = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);
            return pkgHeader + `<div id="${tableId}-container">${fullTableHtml}</div>`;
        }).join('');

        return yjHeader + packagesHtml;
    }).join('');

    outputContainer.innerHTML = html;
}

export function initAggregation() {
    view = document.getElementById('aggregation-view');
    if (!view) return;
    runBtn = document.getElementById('run-aggregation-btn');
    printBtn = document.getElementById('print-aggregation-btn');
    outputContainer = document.getElementById('aggregation-output-container');
    kanaNameInput = document.getElementById('agg-kanaName');
    dosageFormInput = document.getElementById('agg-dosageForm');
    coefficientInput = document.getElementById('reorder-coefficient');
    drugTypeCheckboxes = document.querySelectorAll('input[name="drugType"]');
    reorderNeededCheckbox = document.getElementById('reorder-needed-filter');
    movementOnlyCheckbox = document.getElementById('movement-only-filter');
    
    printBtn.addEventListener('click', () => {
        if (lastData && lastData.length > 0) {
            document.getElementById('valuation-view').classList.remove('print-this-view');
            view.classList.add('print-this-view');
            window.print();
        } else {
            window.showNotification('先に「集計実行」を押してデータを表示してください。', 'error');
        }
    });

    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });
    reorderNeededCheckbox.addEventListener('change', () => renderResults());
    
    // 棚卸調整ボタンのクリックイベント
    outputContainer.addEventListener('click', (e) => {
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

    runBtn.addEventListener('click', async () => {
        window.showLoading('データを取得・計算中...');

        const selectedDrugTypes = Array.from(drugTypeCheckboxes)
            .filter(cb => cb.checked)
            .map(cb => cb.value)
            .join(',');

        const params = new URLSearchParams({
            kanaName: hiraganaToKatakana(kanaNameInput.value),
            dosageForm: dosageFormInput.value,
            coefficient: coefficientInput.value,
            drugTypes: selectedDrugTypes,
            movementOnly: movementOnlyCheckbox.checked,
        });

        try {
            const res = await fetch(`/api/aggregation?${params.toString()}`);
            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || '集計に失敗しました');
            }
            lastData = await res.json();
            
            window.showLoading('集計結果を描画中...');
            
            setTimeout(() => {
                try {
                    renderResults();
                    window.hideLoading();
                    window.showNotification('描画が完了しました。', 'success');
                } catch (renderErr) {
                    outputContainer.innerHTML = `<p style="color:red;">描画エラー: ${renderErr.message}</p>`;
                    window.hideLoading();
                    window.showNotification('結果の描画中にエラーが発生しました。', 'error');
                }
            }, 10);

        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
            window.hideLoading();
            window.showNotification(err.message, 'error');
        }
    });
}