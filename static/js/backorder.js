// C:\Dev\WASABI\static\js\backorder.js

let view, outputContainer;

function renderBackorders(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>現在、発注残はありません。</p>";
        return;
    }

    // ▼▼▼ [修正点] テーブルヘッダーに「操作」列を追加 ▼▼▼
    let html = `
        <table class="data-table">
            <thead>
                <tr>
                    <th style="width: 10%;">発注日</th>
                    <th style="width: 10%;">YJコード</th>
                    <th style="width: 30%;">製品名</th>
                    <th style="width: 30%;">包装仕様</th>
                    <th style="width: 10%;">発注残数量</th>
                    <th style="width: 10%;">操作</th>
                </tr>
            </thead>
            <tbody>
    `;
    // ▲▲▲ 修正ここまで ▲▲▲

    data.forEach(bo => {
        // ▼▼▼【ここから修正】▼▼▼
        // 以前の不正確な文字列組み立てロジックを削除し、
        // バックエンドから渡されたフォーマット済み文字列をそのまま使用する
        const pkgSpec = bo.formattedPackageSpec;
        // ▲▲▲【修正ここまで】▲▲▲

        // ▼▼▼ [修正点] 削除ボタンと、削除に必要な情報をdata属性として追加 ▼▼▼
        html += `
            <tr data-yj-code="${bo.yjCode}"
                data-package-form="${bo.packageForm}"
                data-jan-pack-inner-qty="${bo.janPackInnerQty}"
                data-yj-unit-name="${bo.yjUnitName}">
                <td>${bo.orderDate}</td>
                <td>${bo.yjCode}</td>
                <td class="left">${bo.productName}</td>
                <td class="left">${pkgSpec}</td>
                <td class="right">${bo.yjQuantity.toFixed(2)}</td>
                <td class="center"><button class="btn delete-backorder-btn">削除</button></td>
            </tr>
        `;
        // ▲▲▲ 修正ここまで ▲▲▲
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

// ▼▼▼ [修正点] 削除ボタンのクリックイベント処理を追加 ▼▼▼
async function handleDeleteClick(e) {
    if (!e.target.classList.contains('delete-backorder-btn')) {
        return;
    }

    const row = e.target.closest('tr');
    const productName = row.cells[2].textContent;
    if (!confirm(`「${productName}」の発注残を削除しますか？`)) {
        return;
    }

    const payload = {
        yjCode: row.dataset.yjCode,
        packageForm: row.dataset.packageForm,
        janPackInnerQty: parseFloat(row.dataset.janPackInnerQty),
        yjUnitName: row.dataset.yjUnitName,
    };

    window.showLoading();
    try {
        const res = await fetch('/api/backorders/delete', {
            method: 'POST', // DELETEメソッドも使えますが、POSTの方が安定することがあります
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || '削除に失敗しました。');
        }
        window.showNotification(resData.message, 'success');
        loadAndRenderBackorders(); // 成功したらリストを再読み込み
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}
// ▲▲▲ 修正ここまで ▲▲▲

export function initBackorderView() {
    view = document.getElementById('backorder-view');
    if (!view) return;
    outputContainer = document.getElementById('backorder-output-container');

    // viewが表示されるたびに最新のデータを読み込む
    view.addEventListener('show', loadAndRenderBackorders);
    
    // ▼▼▼ [修正点] クリックイベントリスナーを登録 ▼▼▼
    outputContainer.addEventListener('click', handleDeleteClick);
    // ▲▲▲ 修正ここまで ▲▲▲
}