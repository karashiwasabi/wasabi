// C:\Dev\WASABI\static\js\manual_inventory.js

import { hiraganaToKatakana } from './utils.js';

export function initManualInventory() {
    const view = document.getElementById('manual-inventory-view');
    if (!view) return;
    const dateInput = document.getElementById('manual-inv-date');
    const saveBtn = document.getElementById('save-manual-inv-btn');
    const container = document.getElementById('manual-inventory-container');
    const kanaNameInput = document.getElementById('manual-inv-kanaName');
    const dosageFormInput = document.getElementById('manual-inv-dosageForm');
    let allProducts = [];

    // ▼▼▼ [修正点] 日付を自動設定する関数を追加 ▼▼▼
    async function setDefaultDate() {
        // サーバーに問い合わせるのではなく、今日の日付をデフォルトにする
        dateInput.value = new Date().toISOString().slice(0, 10);
    }
    // ▲▲▲ 修正ここまで ▲▲▲

    async function loadProducts() {
        container.innerHTML = '<p>製品マスターと理論在庫を読み込んでいます...</p>';
        window.showLoading();
        try {
            // ▼▼▼ [修正点] 製品リスト取得APIには最終棚卸日が含まれるようになった ▼▼▼
            const [mastersRes, stockRes] = await Promise.all([
                fetch('/api/inventory/list'),
                fetch('/api/stock/all_current')
            ]);
            // ▲▲▲ 修正ここまで ▲▲▲
            if (!mastersRes.ok) throw new Error('製品リストの取得に失敗しました。');
            if (!stockRes.ok) throw new Error('理論在庫の取得に失敗しました。');

            allProducts = await mastersRes.json();
            const stockMap = await stockRes.json();
            if(!allProducts) allProducts = [];

            allProducts.forEach(p => {
                p.currentStock = stockMap[p.productCode] || 0;
            });
            
            applyFiltersAndRender();
        } catch (err) {
            container.innerHTML = `<p style="color:red;">${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    }
    
    function applyFiltersAndRender() {
    // ▼▼▼ [修正点] フィルター実行時にカナ変換 ▼▼▼
    const kanaFilter = hiraganaToKatakana(kanaNameInput.value).toLowerCase();
    // ▲▲▲ 修正ここまで ▲▲▲
        const dosageFilter = dosageFormInput.value.toLowerCase();
        
        let filteredProducts = allProducts;

        if (kanaFilter) {
            filteredProducts = filteredProducts.filter(p => 
                p.productName.toLowerCase().includes(kanaFilter) || p.kanaName.toLowerCase().includes(kanaFilter)
            );
        }
        if (dosageFilter) {
            filteredProducts = filteredProducts.filter(p => 
                p.usageClassification && p.usageClassification.toLowerCase().includes(dosageFilter)
            );
        }
        
        renderProducts(filteredProducts);
    }

    function renderProducts(productsToRender) {
        if (productsToRender.length === 0) {
            container.innerHTML = '<p>表示する製品マスターがありません。</p>';
            return;
        }

        const groups = productsToRender.reduce((acc, p) => {
            const key = p.yjCode || 'YJコードなし';
            if (!acc[key]) {
                acc[key] = {
                    productName: p.productName,
                    yjCode: p.yjCode,
                    packages: []
                };
            }
            acc[key].packages.push(p);
            return acc;
        }, {});
        
        Object.values(groups).forEach(group => {
            group.packages = group.packages.reduce((acc, p) => {
                 const key = `${p.packageForm}|${p.janPackInnerQty}|${p.yjUnitName}`;
                 if (!acc[key]) {
                     acc[key] = {
                         packageKey: key,
                         masters: []
                     };
                 }
                 acc[key].masters.push(p);
                 return acc;
            }, {});
        });
        
        let html = '';
        for (const group of Object.values(groups)) {
            html += `<div class="agg-yj-header"><span>YJ: ${group.yjCode}</span><span class="product-name">${group.productName}</span></div>`;
            for (const pkg of Object.values(group.packages)) {
                const firstMaster = pkg.masters[0];
                const productCodes = pkg.masters.map(m => m.productCode).join(',');
                const theoreticalStockForPackage = pkg.masters.reduce((sum, master) => sum + (master.currentStock || 0), 0);

                // ▼▼▼ [修正点] 最終棚卸日を表示するロジックを追加 ▼▼▼
                let lastInvDateStr = '';
                if (firstMaster.lastInventoryDate) {
                    const d = firstMaster.lastInventoryDate; // YYYYMMDD
                    lastInvDateStr = ` (最終棚卸: ${d.slice(0,4)}-${d.slice(4,6)}-${d.slice(6,8)})`;
                }

                html += `
                    <div class="agg-pkg-header" style="display:flex; align-items:center; gap: 10px;">
                        <span>包装: ${pkg.packageKey}</span>
                        <div style="margin-left:auto; display:flex; align-items: center; gap: 10px;">
                            <span style="font-size: 12px; color: #333;">理論在庫: ${theoreticalStockForPackage.toFixed(2)}${lastInvDateStr}</span>
                            <label for="inv-qty-${firstMaster.productCode}" style="font-weight: bold;">実在庫数:</label>
                            <input type="number" step="any" class="manual-inv-qty" data-product-codes="${productCodes}" id="inv-qty-${firstMaster.productCode}" style="width: 100px;">
                            <span>${firstMaster.yjUnitName || ''}</span>
                        </div>
                    </div>
                `;
                // ▲▲▲ 修正ここまで ▲▲▲
            }
        }
        container.innerHTML = html;
    }
    
    view.addEventListener('show', () => {
        setDefaultDate();
        loadProducts();
    });

    // ▼▼▼ [修正点] フィルター入力時のイベントリスナーを追加 ▼▼▼
    kanaNameInput.addEventListener('input', applyFiltersAndRender);
    dosageFormInput.addEventListener('input', applyFiltersAndRender);
    // ▲▲▲ 修正ここまで ▲▲▲
    
    saveBtn.addEventListener('click', async () => {
        const date = dateInput.value.replace(/-/g, '');
        if (!date) {
            window.showNotification('棚卸日を指定してください。', 'error');
            return;
        }
        if (!confirm(`${dateInput.value} の棚卸データとして保存します。よろしいですか？`)) {
            return;
        }

        const records = [];
        container.querySelectorAll('.manual-inv-qty').forEach(input => {
            if (input.value !== '') { 
                const quantity = parseFloat(input.value);
                if (!isNaN(quantity)) {
                    const productCode = input.dataset.productCodes.split(',')[0];
                    records.push({
                        productCode: productCode,
                        yjQuantity: quantity
                    });
                }
            }
        });

        window.showLoading();
        try {
            const res = await fetch('/api/inventory/save_manual', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ date, records }),
            });
            const resData = await res.json();
            if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
            window.showNotification(resData.message, 'success');
            loadProducts();
        } catch (err) {
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
        }
    });
}