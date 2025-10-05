// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\settings.js
import { refreshWholesalerMap } from './master_data.js';

let view, userIDInput, passwordInput, saveBtn, usageFolderPathInput, calculationPeriodDaysInput, edgePathInput;
let wholesalerCodeInput, wholesalerNameInput, addWholesalerBtn, wholesalersTableBody;
let migrateInventoryBtn, migrateInventoryInput;
let migrationResultContainer;

function initCleanupSection() {
    const getBtn = document.getElementById('getCleanupCandidatesBtn');
    const outputContainer = document.getElementById('cleanup-output-container');

    getBtn.addEventListener('click', async () => {
         if (!confirm('在庫ゼロかつ3ヶ月間動きのない製品マスターを検索します。よろしいですか？')) {
            return;
        }
        window.showLoading('整理対象のマスターを検索中...');
        try {
             const res = await fetch('/api/masters/cleanup/candidates');
            if (!res.ok) throw new Error('候補リストの取得に失敗しました。');
            
            const candidates = await res.json();
            renderCleanupCandidates(candidates, outputContainer);
        } catch (err) {
            outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
         }
    });

    outputContainer.addEventListener('click', async (e) => {
        if (e.target.id === 'executeCleanupBtn') {
            const checkedBoxes = outputContainer.querySelectorAll('.cleanup-check:checked');
    
             if (checkedBoxes.length === 0) {
                window.showNotification('削除するマスターが選択されていません。', 'error');
                 return;
            }
            if (!confirm(`本当に選択された ${checkedBoxes.length} 件の製品マスターを削除しますか？\nこの操作は元に戻せません。`)) {
                 return;
            }

            const productCodes = Array.from(checkedBoxes).map(cb => cb.dataset.productCode);
        
             window.showLoading('マスターを削除中...');
            try {
                const res = await fetch('/api/masters/cleanup/execute', {
                     method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
 
                     body: JSON.stringify({ productCodes }),
                });
    
                 const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
                
                window.showNotification(resData.message, 'success');
                outputContainer.innerHTML = '';
            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        } else if (e.target.id === 'cleanup-select-all') {
            const isChecked = e.target.checked;
            outputContainer.querySelectorAll('.cleanup-check').forEach(chk => chk.checked = isChecked);
        }
    });
}

function renderCleanupCandidates(candidates, container) {
    if (!candidates || candidates.length === 0) {
        container.innerHTML = '<p>整理対象の製品マスターは見つかりませんでした。</p>';
        return;
    }
    
    let tableHTML = `
        <p><strong>${candidates.length}件</strong>の整理対象マスターが見つかりました。</p>
        <table class="data-table" style="margin-top: 5px;">
            <thead>
                <tr>
                     <th style="width: 5%;"><input type="checkbox" id="cleanup-select-all" checked></th>
                    <th style="width: 35%;">製品名</th>
                    <th style="width: 15%;">JANコード</th>
                     <th style="width: 15%;">メーカー名</th>
                </tr>
            </thead>
             <tbody>
    `;
    candidates.forEach(p => {
        tableHTML += `
            <tr>
                 <td class="center"><input type="checkbox" class="cleanup-check" data-product-code="${p.productCode}" checked></td>
                <td class="left">${p.productName}</td>
                 <td>${p.productCode}</td>
                <td class="left">${p.makerName}</td>
            </tr>
        `;
    });
    tableHTML += `
            </tbody>
        </table>
        <div style="margin-top: 10px; text-align: right;">
             <button id="executeCleanupBtn" class="btn" style="background-color: #dc3545; color: white;">チェックしたマスターを削除</button>
        </div>
    `;
    container.innerHTML = tableHTML;
}

async function loadSettings() {
    try {
         const res = await fetch('/api/settings/get');
        if (!res.ok) throw new Error('設定の読み込みに失敗しました。');
        const settings = await res.json();
        userIDInput.value = settings.emednetUserId || '';
        passwordInput.value = settings.emednetPassword || '';
        if (usageFolderPathInput) {
            usageFolderPathInput.value = settings.usageFolderPath || '';
        }
        if (calculationPeriodDaysInput) {
            calculationPeriodDaysInput.value = settings.calculationPeriodDays || 90;
        }
        if (edgePathInput) {
             edgePathInput.value = settings.edgePath || '';
        }
        const edgeBtn = document.getElementById('edgeDownloadBtn');
        if(edgeBtn) {
            edgeBtn.disabled = !settings.edgePath;
        }

    } catch (err) {
         console.error(err);
        window.showNotification(err.message, 'error');
    }
}

async function saveSettings() {
    window.showLoading();
    try {
        const currentSettingsRes = await fetch('/api/settings/get');
        if (!currentSettingsRes.ok) throw new Error('現在の設定の読み込みに失敗しました。');
        const currentSettings = await currentSettingsRes.json();

        const userId = userIDInput.value;
        const password = passwordInput.value;
        const usagePath = usageFolderPathInput.value;
        const periodDays = parseInt(calculationPeriodDaysInput.value, 10);
        const edgePath = edgePathInput.value.trim();

        const newSettings = {
             ...currentSettings,
            emednetUserId: userId,
            emednetPassword: password,
           
             edeUserId: userId,
            edePassword: password,
            usageFolderPath: usagePath,
            calculationPeriodDays: periodDays,
 
             edgePath: edgePath,
        };

        const res = await fetch('/api/settings/save', {
            method: 'POST',
             headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newSettings),
        });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '設定の保存に失敗しました。');
        
        window.showNotification(resData.message, 'success');
        
        const edgeBtn = document.getElementById('edgeDownloadBtn');
        if(edgeBtn) {
            edgeBtn.disabled = !newSettings.edgePath;
        }

    } catch (err) {
        
         console.error(err);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

function renderWholesalers(wholesalers) {
    if (!wholesalers) {
        wholesalersTableBody.innerHTML = '<tr><td colspan="3">登録されている卸業者がありません。</td></tr>';
        return;
    }
    wholesalersTableBody.innerHTML = wholesalers.map(w => `
        <tr data-code="${w.code}">
            <td>${w.code}</td>
            <td class="left">${w.name}</td>
      
             <td class="center"><button class="delete-wholesaler-btn btn">削除</button></td>
        </tr>
    `).join('');
}

async function loadWholesalers() {
    try {
        const res = await fetch('/api/settings/wholesalers');
        if (!res.ok) throw new Error('卸業者リストの読み込みに失敗しました。');
        const data = await res.json();
        renderWholesalers(data);
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    }
}

async function addWholesaler() {
    const code = wholesalerCodeInput.value.trim();
    const name = wholesalerNameInput.value.trim();
    if (!code || !name) {
        window.showNotification('卸コードと卸業者名の両方を入力してください。', 'error');
        return;
    }

    window.showLoading();
    try {
        const res = await fetch('/api/settings/wholesalers', {
             method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code, name }),
      
         });
        const resData = await res.json();
        if (!res.ok) throw new Error(resData.message || '卸業者の追加に失敗しました。');
        
        window.showNotification(resData.message, 'success');
        wholesalerCodeInput.value = '';
        wholesalerNameInput.value = '';
        loadWholesalers();
        await refreshWholesalerMap();
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

// ▼▼▼【ここから修正】▼▼▼
function renderMigrationResults(results) {
    if (!results || results.length === 0) {
        migrationResultContainer.innerHTML = '<p>処理対象のデータがありませんでした。</p>';
        return;
    }

    const headers = `
        <thead>
            <tr>
                <th>CSV行</th>
                <th>読込JAN</th>
                <th>読込数量(YJ)</th>
                <th>マスター</th>
                <th>登録製品名</th>
                <th>登録JAN</th>
                <th>登録YJ数量</th>
                <th>エラー</th>
            </tr>
        </thead>
    `;

    const rows = results.map(r => {
        const masterStatusMap = {
            "EXISTED": "既存",
            "JCSHMS": "新規(JCSHMS)",
            "PROVISIONAL": "新規(仮)"
        };
        const masterStatus = masterStatusMap[r.masterCreated] || '不明';
        const errorClass = r.error ? 'style="background-color: #f8d7da;"' : '';

        return `
            <tr ${errorClass}>
                <td class="left" style="font-size: 10px;">${(r.originalRow || []).join(', ')}</td>
                <td>${r.parsedRecord ? r.parsedRecord.janCode : ''}</td>
                <td class="right">${r.parsedRecord ? r.parsedRecord.yjQuantity.toFixed(2) : ''}</td>
                <td>${masterStatus}</td>
                <td class="left">${r.resultRecord ? r.resultRecord.productName : '-'}</td>
                <td>${r.resultRecord ? r.resultRecord.janCode : '-'}</td>
                <td class="right">${r.resultRecord ? r.resultRecord.yjQuantity.toFixed(2) : '-'}</td>
                <td class="left" style="color: red;">${r.error || ''}</td>
            </tr>
        `;
    }).join('');

    migrationResultContainer.innerHTML = `<table class="data-table">${headers}<tbody>${rows}</tbody></table>`;
}

export function initSettings() {
    view = document.getElementById('settings-view');
    if (!view) return;

    userIDInput = document.getElementById('emednetUserID');
    passwordInput = document.getElementById('emednetPassword');
    saveBtn = document.getElementById('saveSettingsBtn');
    usageFolderPathInput = document.getElementById('usageFolderPath');
    calculationPeriodDaysInput = document.getElementById('calculationPeriodDays');
    edgePathInput = document.getElementById('edgePath');
    wholesalerCodeInput = document.getElementById('wholesalerCode');
    wholesalerNameInput = document.getElementById('wholesalerName');
    addWholesalerBtn = document.getElementById('addWholesalerBtn');
    wholesalersTableBody = document.querySelector('#wholesalers-table tbody');
    const clearTransactionsBtn = document.getElementById('clearAllTransactionsBtn');
    const clearMastersBtn = document.getElementById('clearAllMastersBtn');
    
    migrateInventoryBtn = document.getElementById('migrateInventoryBtn');
    migrateInventoryInput = document.getElementById('migrateInventoryInput');
    migrationResultContainer = document.getElementById('migration-result-container');

    initCleanupSection();
    
    if (migrateInventoryBtn) {
        migrateInventoryBtn.addEventListener('click', () => {
            if (!confirm('旧システムから在庫データを移行します。\nCSVファイルの形式（inventory_date, product_code, quantity）が正しいことを確認してください。\nよろしいですか？')) {
                return;
            }
            migrateInventoryInput.click();
        });
    }

    if (migrateInventoryInput) {
        migrateInventoryInput.addEventListener('change', async (event) => {
            const file = event.target.files[0];
            if (!file) return;

            const formData = new FormData();
            formData.append('file', file);

            window.showLoading('在庫データを移行中...');
            migrationResultContainer.innerHTML = ''; // 結果表示エリアをクリア
            try {
                const res = await fetch('/api/inventory/migrate', {
                    method: 'POST',
                    body: formData,
                });
                const resData = await res.json();
                if (!res.ok) {
                    throw new Error(resData.message || '移行に失敗しました。');
                }
                window.showNotification(resData.message, 'success');
                if (resData.details) {
                    renderMigrationResults(resData.details);
                }
            } catch (err) {
                console.error(err);
                migrationResultContainer.innerHTML = `<p style="color:red;">エラー: ${err.message}</p>`;
                window.showNotification(`エラー: ${err.message}`, 'error');
            } finally {
                window.hideLoading();
                event.target.value = '';
            }
        });
    }
  
    // ▲▲▲【修正ここまで】▲▲▲

    clearMastersBtn.addEventListener('click', async () => {
        if (!confirm('本当に全ての製品マスターを削除しますか？\n\nJCSHMSマスターも削除されるため、再読み込みするまで品目情報が失われます。この操作は元に戻せません。')) {
            return;
        }
 
        window.showLoading();
        try {
            const res = await fetch('/api/masters/clear_all', { method: 'POST' });
      
             const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '製品マスターの削除に失敗しました。');
            window.showNotification(resData.message, 'success');
 
         } catch (err) {
            console.error(err);
            window.showNotification(err.message, 'error');
       
         } finally {
            window.hideLoading();
        }
    });

    saveBtn.addEventListener('click', saveSettings);
    addWholesalerBtn.addEventListener('click', addWholesaler);
    
    clearTransactionsBtn.addEventListener('click', async () => {
         if (!confirm('本当にすべての取引履歴（入出庫、納品、処方、棚卸など）を削除しますか？\n\nこの操作は元に戻せません。')) {
            return;
        }
        window.showLoading();
        try {
 
             const res = await fetch('/api/transactions/clear_all', { method: 'POST' });
            const resData = await res.json();
       
             if (!res.ok) throw new Error(resData.message || '取引データの削除に失敗しました。');
            window.showNotification(resData.message, 'success');
        } catch (err) {
       
             console.error(err);
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
  
         }
    });

    wholesalersTableBody.addEventListener('click', async (e) => {
        if (e.target.classList.contains('delete-wholesaler-btn')) {
            const row = e.target.closest('tr');
  
             const code = row.dataset.code;
            if (!confirm(`卸コード [${code}] を削除します。よろしいですか？`)) {
             
                 return;
            }
            window.showLoading();
            try {
  
                 const res = await fetch(`/api/settings/wholesalers/${code}`, { method: 'DELETE' });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '削除に失敗しました。');
                window.showNotification(resData.message, 'success');
   
                 loadWholesalers();
                await refreshWholesalerMap();
            
             } catch (err) {
                console.error(err); 
                window.showNotification(err.message, 'error');
     
             } finally {
                window.hideLoading();
            }
         }
    });
}

export function onViewShow() {
    loadSettings();
    loadWholesalers();
}