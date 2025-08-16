let view, outputContainer;

function renderBackorders(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>現在、発注残はありません。</p>";
        return;
    }

    let html = `
        <table class="data-table">
            <thead>
                <tr>
                    <th style="width: 10%;">発注日</th>
                    <th style="width: 10%;">YJコード</th>
                    <th style="width: 35%;">製品名</th>
                    <th style="width: 35%;">包装仕様</th>
                    <th style="width: 10%;">発注残数量</th>
                </tr>
            </thead>
            <tbody>
    `;

    data.forEach(bo => {
        const pkgSpec = `${bo.packageForm} ${bo.janPackInnerQty}${bo.yjUnitName}`;
        html += `
            <tr>
                <td>${bo.orderDate}</td>
                <td>${bo.yjCode}</td>
                <td class="left">${bo.productName}</td>
                <td class="left">${pkgSpec}</td>
                <td class="right">${bo.yjQuantity.toFixed(2)}</td>
            </tr>
        `;
    });

    html += `</tbody></table>`;
    outputContainer.innerHTML = html;
}

async function loadAndRenderBackorders() {
    outputContainer.innerHTML = '<p>読み込み中...</p>';
    try {
        const res = await fetch('/api/backorders');
        if (!res.ok) throw new Error('発注残リストの読み込みに失敗しました。');
        const data = await res.json();
        renderBackorders(data);
    } catch (err) {
        outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
    }
}

export function initBackorderView() {
    view = document.getElementById('backorder-view');
    if (!view) return;
    outputContainer = document.getElementById('backorder-output-container');
    
    // viewが表示されるたびに最新のデータを読み込む
    view.addEventListener('show', loadAndRenderBackorders);
}