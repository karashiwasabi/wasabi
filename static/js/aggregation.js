import { transactionTypeMap, createUploadTableHTML, renderUploadTableRows } from './common_table.js';
let view, runBtn, printBtn, outputContainer, startDateInput, endDateInput, kanaNameInput, dosageFormInput, coefficientInput, drugTypeCheckboxes, reorderNeededCheckbox;
let lastData = []; // サーバーから受け取った元のデータを保持

/**
 * 在庫数をフォーマットするヘルパー関数
 * @param {number | string} balance - 在庫数（数値または "期間棚卸なし" の文字列）
 * @returns {string} 表示用にフォーマットされた文字列
 */
function formatBalance(balance) {
    // 渡された値が数値の場合のみ、小数点以下2桁にフォーマットする
    if (typeof balance === 'number') {
        return balance.toFixed(2);
    }
    // 文字列（"期間棚卸なし" など）の場合は、そのまま返す
    return balance;
}

function renderResults() {
    let dataToRender = lastData; // 「不足品のみ表示」フィルターを適用
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
        // formatBalance ヘルパー関数を使用して、数値・文字列の両方に対応
        html += `
            <div class="agg-yj-header" ${yjGroup.isReorderNeeded ? 'style="background-color: #f8d7da;"' : ''}>
                <span>YJ: ${yjGroup.yjCode}</span>
                <span class="product-name">${yjGroup.productName}</span>
                <span class="balance-info">
                    在庫: ${formatBalance(yjGroup.endingBalance)} | 
                    発注点: ${formatBalance(yjGroup.totalReorderPoint)} | 
                    変動: ${formatBalance(yjGroup.netChange)}
                </span>
            </div>
        `;
     
        yjGroup.packageLedgers.forEach((pkg, pkgIndex) => {
            const tableId = `agg-table-${yjIndex}-${pkgIndex}`;
            // formatBalance ヘルパー関数を使用して、数値・文字列の両方に対応
            html += `
                <div class="agg-pkg-header" ${pkg.isReorderNeeded ? 'style="background-color: #fff3cd;"' : ''}>
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.endingBalance)} |
                        発注点: ${formatBalance(pkg.reorderPoint)} | 
                        変動: ${formatBalance(pkg.netChange)}
                    </span>
                </div>
                <div id="${tableId}-container"></div>`;
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
    if (!view) return; // 新しいフィルター要素を取得
    runBtn = document.getElementById('run-aggregation-btn');
    printBtn = document.getElementById('print-aggregation-btn');
    outputContainer = document.getElementById('aggregation-output-container');
    startDateInput = document.getElementById('startDate');
    endDateInput = document.getElementById('endDate');
    kanaNameInput = document.getElementById('kanaName');
    dosageFormInput = document.getElementById('dosageForm');
    coefficientInput = document.getElementById('reorder-coefficient');
    drugTypeCheckboxes = document.querySelectorAll('input[name="drugType"]');
    reorderNeededCheckbox = document.getElementById('reorder-needed-filter'); // 日付のデフォルト値を設定
    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, today.getDate());
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);

    printBtn.addEventListener('click', () => window.print());
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
            lastData = await res.json(); // 元データを保持

         renderResults();       // ← 既存：DOM 更新
         // ← DOM 更新後、次のペイント直前に発火するカスタムイベント
        requestAnimationFrame(() => {
        window.dispatchEvent(new Event('aggregationRendered'));
        });

        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    });

  // ← ここでリスナー登録
  window.addEventListener('aggregationRendered', () => {
    console.log('集計描画完了！');
    // たとえばボタンを有効化するとか、別コンポーネントに通知するとか
  });
}