// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\returns.js

import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';
import { hiraganaToKatakana, getLocalDateString } from './utils.js';


function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    return balance;
}

function renderReturnCandidates(data, container) {
    if (!data || data.length === 0) {
        container.innerHTML = "<p>返品可能な品目はありませんでした。</p>";
        return;
    }

    let html = data.map((yjGroup, yjIndex) => {
        const yjHeader = `
            <div class="agg-yj-header" style="background-color: #0d6efd; color: white;">
                <span>YJ: ${yjGroup.yjCode}</span>
                <span class="product-name">${yjGroup.productName}</span>
            </div>
        `;
        
        const packagesHtml = yjGroup.packageLedgers.map((pkg, pkgIndex) => {
            const surplus = pkg.effectiveEndingBalance - pkg.reorderPoint;
            let pkgHeader = `
                <div class="agg-pkg-header" style="border-left: 5px solid #0d6efd;">
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.effectiveEndingBalance)} | 
                        発注点: ${formatBalance(pkg.reorderPoint)} | 
                        <strong style="color: #0d6efd;">余剰数(目安): ${formatBalance(surplus)}</strong>
                    </span>
                </div>
            `;

            if (pkg.deliveryHistory && pkg.deliveryHistory.length > 0) {
                const tableId = `delivery-history-${yjIndex}-${pkgIndex}`;
                
                const tableShell = createUploadTableHTML(tableId);
                const tableBodyContent = renderUploadTableRows(pkg.deliveryHistory);
                
                const fullTableHtml = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);
                
                pkgHeader += `<div id="${tableId}-container" style="padding: 0 10px 10px 10px;">${fullTableHtml}</div>`;
            }
            return pkgHeader;
        }).join('');

        return yjHeader + packagesHtml;
    }).join('');

    container.innerHTML = html;
}

export function initReturnsView() {
    const view = document.getElementById('returns-view');
    if (!view) return;
    const runBtn = document.getElementById('run-returns-list-btn');
    const outputContainer = document.getElementById('returns-list-output-container');
    const kanaNameInput = document.getElementById('ret-kanaName');
    const dosageFormInput = document.getElementById('ret-dosageForm');
    const coefficientInput = document.getElementById('ret-coefficient');
    const printBtn = document.getElementById('print-returns-list-btn');
    const shelfNumberInput = document.getElementById('ret-shelf-number');

    runBtn.addEventListener('click', async () => {
        window.showLoading();
        const params = new URLSearchParams({
            kanaName: hiraganaToKatakana(kanaNameInput.value),
            dosageForm: dosageFormInput.value,
            shelfNumber: shelfNumberInput.value,
            coefficient: coefficientInput.value,
        });

        try {
            const res = await fetch(`/api/returns/candidates?${params.toString()}`);
            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || 'List generation failed');
            }
            const data = await res.json();
            renderReturnCandidates(data, outputContainer);
        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    });

    printBtn.addEventListener('click', () => {
        view.classList.add('print-this-view');
        window.print();
    });

    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });
}