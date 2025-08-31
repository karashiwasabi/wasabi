// C:\Dev\WASABI\static\js\aggregation.js

import { hiraganaToKatakana } from './utils.js';
// transactionTypeMapは直接は使わないが、renderUploadTableRowsが内部で使うため、
// 共通テーブル描画の完全な自己完結のためにはインポートしておくのが望ましい
// ただし、現在のコードでは renderUploadTableRows が transactionTypeMap を直接参照しているため、
// aggregation.js 側でのインポートは厳密には不要です。
import { transactionTypeMap, createUploadTableHTML, renderUploadTableRows } from './common_table.js'; 

let view, runBtn, printBtn, outputContainer, startDateInput, endDateInput, kanaNameInput, dosageFormInput, coefficientInput, drugTypeCheckboxes, reorderNeededCheckbox, movementOnlyCheckbox;
let lastData = []; 

function formatBalance(balance) {
    if (typeof balance === 'number') {
        return balance.toFixed(2); 
    }
    return balance; 
}

/**
 * [修正点]
 * 描画ロジックを方針書に沿って1段階プロセスに変更。
 */
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

    // 1回のループで、取引履歴テーブルまで含んだ完全なHTML文字列を生成する
    let html = dataToRender.map((yjGroup, yjIndex) => {
        let yjReorderPointText = formatBalance(yjGroup.totalReorderPoint);
        if (yjGroup.totalPrecompounded > 0) {
            yjReorderPointText = `${formatBalance(yjGroup.totalBaseReorderPoint)} + 予${formatBalance(yjGroup.totalPrecompounded)} = ${formatBalance(yjGroup.totalReorderPoint)}`;
        }
        
        const yjHeader = `
            <div class="agg-yj-header" ${yjGroup.isReorderNeeded ? 'style="background-color: #ff0015ff; color: white;"' : ''}>
                <span>YJ: ${yjGroup.yjCode}</span>
                <span class="product-name">${yjGroup.productName}</span>
                <span class="balance-info">
                    在庫: ${formatBalance(yjGroup.endingBalance)} | 
                    発注点: ${yjReorderPointText} | 
                    変動: ${formatBalance(yjGroup.netChange)}
                </span>
            </div>
        `; 
     
        const packagesHtml = yjGroup.packageLedgers.map((pkg, pkgIndex) => {
            const tableId = `agg-table-${yjIndex}-${pkgIndex}`;
            let pkgReorderPointText = formatBalance(pkg.reorderPoint);
            if (pkg.precompoundedTotal > 0) {
                pkgReorderPointText = `${formatBalance(pkg.baseReorderPoint)} + 予${formatBalance(pkg.precompoundedTotal)} = ${formatBalance(pkg.reorderPoint)}`; 
            }

            const pkgHeader = `
                <div class="agg-pkg-header" ${pkg.isReorderNeeded ? 'style="background-color: #ff0015ff; color: white;"' : ''}>
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.endingBalance)} |
                        在庫(発注残含): ${formatBalance(pkg.effectiveEndingBalance)} | 
                        発注点: ${pkgReorderPointText} |  
                        変動: ${formatBalance(pkg.netChange)}
                    </span>
                </div>
            `; 

            // テーブルの枠と中身のHTML文字列をそれぞれ取得
            const tableShell = createUploadTableHTML(tableId);
            const tableBodyContent = renderUploadTableRows(pkg.transactions);
            
            // 文字列を結合して完全なテーブルHTMLを生成
            const fullTableHtml = tableShell.replace('<tbody></tbody>', `<tbody>${tableBodyContent}</tbody>`);

            return pkgHeader + `<div id="${tableId}-container">${fullTableHtml}</div>`;
        }).join('');

        return yjHeader + packagesHtml;
    }).join('');

    // 完成したHTMLを一度だけDOMに書き込む
    outputContainer.innerHTML = html;
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
    movementOnlyCheckbox = document.getElementById('movement-only-filter'); 
    
    // ▼▼▼ [ここから修正] ▼▼▼
    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, 1); // 3ヶ月前の1日
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);
    // ▲▲▲ [修正ここまで] ▲▲▲

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
    
    runBtn.addEventListener('click', async () => { 
        window.showLoading();

        const selectedDrugTypes = Array.from(drugTypeCheckboxes)
            .filter(cb => cb.checked)
            .map(cb => cb.value)
            .join(',');

        const params = new URLSearchParams({
            startDate: startDateInput.value.replace(/-/g, ''),
            endDate: endDateInput.value.replace(/-/g, ''), 
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