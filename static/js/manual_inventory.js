// C:\Dev\WASABI\static\js\manual_inventory.js

export function initManualInventory() {
    const view = document.getElementById('manual-inventory-view');
    if (!view) return;
    const dateInput = document.getElementById('manual-inv-date');
    const saveBtn = document.getElementById('save-manual-inv-btn');
    const container = document.getElementById('manual-inventory-container');
    let allProducts = [];
    dateInput.value = new Date().toISOString().slice(0, 10);

    // ▼▼▼ [修正点] 製品リストと理論在庫を並行して取得するロジックに変更 ▼▼▼
    async function loadProducts() {
        container.innerHTML = '<p>製品マスターと理論在庫を読み込んでいます...</p>';
        window.showLoading();
        try {
            // 製品リストと全在庫マップを同時に取得
            const [mastersRes, stockRes] = await Promise.all([
                fetch('/api/inventory/list'),
                fetch('/api/stock/all_current')
            ]);
            if (!mastersRes.ok) throw new Error('製品リストの取得に失敗しました。');
            if (!stockRes.ok) throw new Error('理論在庫の取得に失敗しました。');

            allProducts = await mastersRes.json();
            const stockMap = await stockRes.json();
            if(!allProducts) allProducts = [];

            // 製品リストに理論在庫情報を追加
            allProducts.forEach(p => {
                p.currentStock = stockMap[p.productCode] || 0;
            });
            
            renderProducts();
        } catch (err) {
            container.innerHTML = `<p style="color:red;">${err.message}</p>`;
        } finally {
            window.hideLoading();
        }
    }
    // ▲▲▲ 修正ここまで ▲▲▲

    // ▼▼▼ [修正点] 理論在庫を表示するHTML生成ロジックに変更 ▼▼▼
    function renderProducts() {
        if (allProducts.length === 0) {
            container.innerHTML = '<p>表示する製品マスターがありません。</p>';
            return;
        }

        const groups = allProducts.reduce((acc, p) => {
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
                // 包装単位での理論在庫を合算
                const theoreticalStockForPackage = pkg.masters.reduce((sum, master) => sum + (master.currentStock || 0), 0);
                
                html += `
                    <div class="agg-pkg-header" style="display:flex; align-items:center; gap: 10px;">
                        <span>包装: ${pkg.packageKey}</span>
                        <div style="margin-left:auto; display:flex; align-items: center; gap: 10px;">
                            <span style="font-size: 12px; color: #333;">理論在庫: ${theoreticalStockForPackage.toFixed(2)}</span>
                            <label for="inv-qty-${firstMaster.productCode}" style="font-weight: bold;">実在庫数:</label>
                            <input type="number" step="any" class="manual-inv-qty" data-product-codes="${productCodes}" id="inv-qty-${firstMaster.productCode}" style="width: 100px;">
                            <span>${firstMaster.yjUnitName || ''}</span>
                        </div>
                    </div>
                `;
            }
        }
        container.innerHTML = html;
    }
    // ▲▲▲ 修正ここまで ▲▲▲
    
    view.addEventListener('show', loadProducts);
    
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