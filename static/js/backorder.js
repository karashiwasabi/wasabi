// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\backorder.js

let view, outputContainer;

// ▼▼▼【ここから修正】▼▼▼
function renderBackorders(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = "<p>現在、発注残はありません。</p>";
        return;
    }

    let html = `
        <div class="controls-grid" style="margin-bottom: 10px; display: flex; gap: 10px;">
            <button class="btn" id="bulk-delete-backorder-btn" style="background-color: #dc3545; color: white;">選択した項目を一括削除</button>
            <button class="btn" id="delete-all-backorder-btn">表示されている項目を全て削除</button>
        </div>
        <table class="data-table">
            <thead>
                <tr>
                    <th style="width: 5%;"><input type="checkbox" id="select-all-backorders-checkbox"></th>
                    <th style="width: 10%;">発注日</th>
                    <th style="width: 10%;">YJコード</th>
                    <th style="width: 25%;">製品名</th>
                    <th style="width: 20%;">包装仕様</th>
                    <th style="width: 10%;">発注数量</th>
                    <th style="width: 10%;">残数量</th>
                    <th style="width: 10%;">個別操作</th>
                </tr>
            </thead>
            <tbody>
    `;

    data.forEach(bo => {
        const pkgSpec = bo.formattedPackageSpec;

        html += `
            <tr data-id="${bo.id}">
                <td class="center"><input type="checkbox" class="backorder-select-checkbox"></td>
                <td>${bo.orderDate}</td>
                <td>${bo.yjCode}</td>
                <td class="left">${bo.productName}</td>
                <td class="left">${pkgSpec}</td>
                <td class="right">${bo.orderQuantity.toFixed(2)}</td>
                <td class="right">${bo.remainingQuantity.toFixed(2)}</td>
                <td class="center"><button class="btn delete-backorder-btn">削除</button></td>
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

async function handleBackorderEvents(e) {
    const target = e.target;

    // 個別削除ボタン
    if (target.classList.contains('delete-backorder-btn')) {
        const row = target.closest('tr');
        if (!confirm(`「${row.cells[3].textContent}」の発注残（発注日: ${row.cells[1].textContent}）を削除しますか？`)) {
            return;
        }
        const payload = {
            id: parseInt(row.dataset.id, 10),
        };
        window.showLoading();
        try {
            const res = await fetch('/api/backorders/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
            window.showNotification(resData.message, 'success');
            loadAndRenderBackorders();
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    }

    // 全選択チェックボックス
    if (target.id === 'select-all-backorders-checkbox') {
        const isChecked = target.checked;
        document.querySelectorAll('.backorder-select-checkbox').forEach(cb => cb.checked = isChecked);
    }

    // 全件削除ボタン
    if (target.id === 'delete-all-backorder-btn') {
        document.getElementById('select-all-backorders-checkbox').checked = true;
        document.querySelectorAll('.backorder-select-checkbox').forEach(cb => cb.checked = true);
        document.getElementById('bulk-delete-backorder-btn').click(); // 一括削除ボタンのクリックを擬似的に発火
    }

    // 選択項目の一括削除ボタン
    if (target.id === 'bulk-delete-backorder-btn') {
        const checkedRows = document.querySelectorAll('.backorder-select-checkbox:checked');
        if (checkedRows.length === 0) {
            window.showNotification('削除する項目が選択されていません。', 'error');
            return;
        }
        if (!confirm(`${checkedRows.length}件の発注残を削除します。よろしいですか？`)) {
            return;
        }

        const payload = Array.from(checkedRows).map(cb => {
            const row = cb.closest('tr');
            return {
                id: parseInt(row.dataset.id, 10),
            };
        });

        window.showLoading();
        try {
            const res = await fetch('/api/backorders/bulk_delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '一括削除に失敗しました。');
            window.showNotification(resData.message, 'success');
            loadAndRenderBackorders();
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    }
}

export function initBackorderView() {
    view = document.getElementById('backorder-view');
    if (!view) return;
    outputContainer = document.getElementById('backorder-output-container');
    
    view.addEventListener('show', loadAndRenderBackorders);
    outputContainer.addEventListener('click', handleBackorderEvents);
}
// ▲▲▲【修正ここまで】▲▲▲