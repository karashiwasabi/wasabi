// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\returns.js

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

    let html = '';
    data.forEach(yjGroup => {
        html += `
            <div class="agg-yj-header" style="background-color: #0d6efd; color: white;">
                <span>YJ: ${yjGroup.yjCode}</span>
                <span class="product-name">${yjGroup.productName}</span>
            </div>
        `;
        yjGroup.packageLedgers.forEach(pkg => {
            // Surplus = 在庫 - 発注点 (返品可能な余裕分)
            const surplus = pkg.effectiveEndingBalance - pkg.reorderPoint;
            html += `
                <div class="agg-pkg-header" style="border-left: 5px solid #0d6efd;">
                    <span>包装: ${pkg.packageKey}</span>
                    <span class="balance-info">
                        在庫: ${formatBalance(pkg.effectiveEndingBalance)} | 
                        発注点: ${formatBalance(pkg.reorderPoint)} | 
                        <strong style="color: #0d6efd;">余剰数(目安): ${formatBalance(surplus)}</strong>
                    </span>
                </div>
            `;
        });
    });
    container.innerHTML = html;
}

export function initReturnsView() {
    const view = document.getElementById('returns-view');
    if (!view) return;

    const runBtn = document.getElementById('run-returns-list-btn');
    const outputContainer = document.getElementById('returns-list-output-container');
    const startDateInput = document.getElementById('ret-startDate');
    const endDateInput = document.getElementById('ret-endDate');
    const kanaNameInput = document.getElementById('ret-kanaName');
    const dosageFormInput = document.getElementById('ret-dosageForm');
    // ▼▼▼ [修正点] 係数入力欄と印刷ボタンの要素を取得 ▼▼▼
    const coefficientInput = document.getElementById('ret-coefficient');
    const printBtn = document.getElementById('print-returns-list-btn');
    // ▲▲▲ 修正ここまで ▲▲▲

    const today = new Date();
    const threeMonthsAgo = new Date(today.getFullYear(), today.getMonth() - 3, today.getDate());
    endDateInput.value = today.toISOString().slice(0, 10);
    startDateInput.value = threeMonthsAgo.toISOString().slice(0, 10);

    runBtn.addEventListener('click', async () => {
        window.showLoading();
        const params = new URLSearchParams({
            startDate: startDateInput.value.replace(/-/g, ''),
            endDate: endDateInput.value.replace(/-/g, ''),
            kanaName: kanaNameInput.value,
            dosageForm: dosageFormInput.value,
            // ▼▼▼ [修正点] 係数の値を入力欄から取得する ▼▼▼
            coefficient: coefficientInput.value,
            // ▲▲▲ 修正ここまで ▲▲▲
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

    // ▼▼▼ [修正点] 印刷ボタンのイベントリスナーを追加 ▼▼▼
    printBtn.addEventListener('click', () => {
        // 印刷対象のビューに専用クラスを付与
        view.classList.add('print-this-view');
        window.print();
    });

    // 印刷ダイアログが閉じた後にクラスを削除する
    window.addEventListener('afterprint', () => {
        view.classList.remove('print-this-view');
    });
    // ▲▲▲ 修正ここまで ▲▲▲
}