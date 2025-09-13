// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\inventory_history.js (新規作成)
// ▼▼▼ [修正点] getLocalDateString をインポート ▼▼▼
import { getLocalDateString } from './utils.js';
// ▲▲▲ 修正ここまで ▲▲▲
let view, dateInput, showBtn, outputContainer;

/**
 * 棚卸履歴のテーブルを描画する
 * @param {Array} records - 表示する取引記録の配列
 */
function renderInventoryHistory(records) {
    if (!records || records.length === 0) {
        outputContainer.innerHTML = "<p>この日付の棚卸データはありません。</p>";
        return;
    }

    const tableHeader = `
        <thead>
            <tr>
                <th style="width: 30%;">製品名</th>
                <th style="width: 15%;">JANコード</th>
                <th style="width: 10%;">YJ数量</th>
                <th style="width: 10%;">YJ単位</th>
                <th style="width: 25%;">包装仕様</th>
                <th style="width: 10%;">操作</th>
            </tr>
        </thead>
    `;

    const tableBody = records.map(rec => `
        <tr data-id="${rec.id}">
            <td class="left">${rec.productName}</td>
            <td>${rec.janCode}</td>
            <td class="right">${rec.yjQuantity.toFixed(2)}</td>
            <td>${rec.yjUnitName}</td>
            <td class="left">${rec.packageSpec}</td>
            <td class="center"><button class="btn delete-inv-record-btn" data-id="${rec.id}">削除</button></td>
        </tr>
    `).join('');

    outputContainer.innerHTML = `<table class="data-table">${tableHeader}<tbody>${tableBody}</tbody></table>`;
}

/**
 * 削除ボタンが押されたときの処理
 * @param {number} transactionId - 削除する取引のID
 */
async function handleDelete(transactionId) {
    if (!confirm(`この棚卸レコードを完全に削除しますか？\nこの操作は元に戻せません。`)) {
        return;
    }

    window.showLoading();
    try {
        const res = await fetch(`/api/transaction/delete_by_id/${transactionId}`, {
            method: 'DELETE',
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || '削除に失敗しました。');
        }
        
        // 画面から該当の行を削除
        const rowToRemove = outputContainer.querySelector(`tr[data-id="${transactionId}"]`);
        if (rowToRemove) {
            rowToRemove.remove();
        }

        window.showNotification(resData.message, 'success');
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}


export function initInventoryHistory() {
    view = document.getElementById('inventory-history-view');
    if (!view) return;

    dateInput = document.getElementById('history-inv-date');
    showBtn = document.getElementById('history-inv-show-btn');
    outputContainer = document.getElementById('inventory-history-output');

    // ▼▼▼ [修正点] 日付設定処理を新しい関数に置き換え ▼▼▼
    dateInput.value = getLocalDateString();
    // ▲▲▲ 修正ここまで ▲▲▲

    showBtn.addEventListener('click', async () => {
        const date = dateInput.value.replace(/-/g, '');
        if (!date) {
            window.showNotification('日付を選択してください。', 'error');
            return;
        }

        window.showLoading();
        try {
            const res = await fetch(`/api/inventory/by_date?date=${date}`);
            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || '棚卸データの取得に失敗しました。');
            }
            const records = await res.json();
            renderInventoryHistory(records);
        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    });

    outputContainer.addEventListener('click', (e) => {
        if (e.target.classList.contains('delete-inv-record-btn')) {
            const transactionId = e.target.dataset.id;
            handleDelete(transactionId);
        }
    });
}