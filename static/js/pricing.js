import { wholesalerMap } from './master_data.js';
let view, wholesalerSelect, exportBtn, uploadInput, bulkUpdateBtn, outputContainer, makerFilterInput;
let fullPricingData = []; 
let orderedWholesalers = []; 
let setLowestPriceBtn;
let unregisteredFilterCheckbox;
let exportUnregisteredBtn; 
let lastSelectedWholesaler = '';

function renderComparisonTable(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = '<p>表示対象のデータがありませんでした。</p>';
        return;
    }

    const wholesalerHeaders = orderedWholesalers.length > 0 ?
        orderedWholesalers.map(w => `<th>${w}</th>`).join('') : '';
    const wholesalerReverseMap = new Map();
    for (const [code, name] of wholesalerMap.entries()) {
        wholesalerReverseMap.set(name, code);
    }

    let tableHTML = `
        <table class="data-table">
            <thead>
                <tr>
                    <th rowspan="2">製品名</th>
                    <th rowspan="2">包装</th>
                    <th rowspan="2">メーカー</th>
                    <th rowspan="2">現納入価</th>
                    <th colspan="${orderedWholesalers.length || 1}">卸提示価格</th>
                    <th rowspan="2">採用卸</th>
                    <th rowspan="2">決定納入価</th>
                </tr>
                <tr>
                    ${wholesalerHeaders}
                </tr>
            </thead>
            <tbody>
    `;
    data.forEach(p => {
        const productCode = p.productCode;
        let wholesalerOptions = '<option value="">--- 選択 ---</option>';
        for (const [wCode, wName] of wholesalerMap.entries()) {
            const isSelected = (wCode === p.supplierWholesale);
            wholesalerOptions += `<option value="${wCode}" ${isSelected ? 'selected' : ''}>${wName}</option>`;
        }
        const quoteCells = orderedWholesalers.length > 0 
            ? orderedWholesalers.map(w => {
            const price = (p.quotes || {})[w];
            if (price === undefined) return '<td>-</td>';
            const lowestPrice = Math.min(...Object.values(p.quotes || {}).filter(v => typeof v === 'number'));
            const style = (price === lowestPrice) ? 'style="background-color: #d1e7dd; font-weight: bold;"' : '';
            return `<td class="right" ${style}>${price.toFixed(2)}</td>`;
        }).join('') : '<td>-</td>';
        const initialPrice = p.purchasePrice > 0 ? p.purchasePrice.toFixed(2) : '';
        tableHTML += `
            <tr data-product-code="${productCode}">
                <td class="left">${p.productName}</td>
                <td class="left">${p.formattedPackageSpec || ''}</td>
                <td class="left">${p.makerName}</td>
                <td class="right">${p.purchasePrice.toFixed(2)}</td>
                ${quoteCells}
                <td><select class="supplier-select">${wholesalerOptions}</select></td>
                <td><input type="number" class="manual-price-input" step="0.01" style="width: 100px;" value="${initialPrice}"></td>
            </tr>
        `;
    });
    tableHTML += `</tbody></table>`;
    outputContainer.innerHTML = tableHTML;
}

async function sendUpdatePayload(payload) {
    if (payload.length === 0) {
        window.showNotification('更新するデータがありません。', 'error');
        return;
    }
    window.showLoading();
    try {
        const res = await fetch('/api/pricing/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        const resData = await res.json();
        if (!res.ok) {
            throw new Error(resData.message || 'マスターの更新に失敗しました。');
        }
        window.showNotification(resData.message, 'success');
        payload.forEach(update => {
            const product = fullPricingData.find(p => p.productCode === update.productCode);
            if (product) {
                product.purchasePrice = update.newPrice;
                product.supplierWholesale = update.newWholesaler;
            }
        });
        applyFiltersAndRender();
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

async function handleBulkUpdate() {
    if (!confirm('表示されている全ての行の内容でマスターデータを一括更新します。よろしいですか？')) {
        return;
    }
    const rows = outputContainer.querySelectorAll('tbody tr');
    const payload = [];
    rows.forEach(row => {
        const productCode = row.dataset.productCode;
        const supplierSelect = row.querySelector('.supplier-select');
        const manualPriceInput = row.querySelector('.manual-price-input');
        const selectedWholesalerCode = supplierSelect.value;
        const price = parseFloat(manualPriceInput.value);
        if (productCode && selectedWholesalerCode && !isNaN(price)) {
            payload.push({ productCode, newPrice: price, newWholesaler: selectedWholesalerCode });
        } else if (productCode && !selectedWholesalerCode) {
             payload.push({ productCode, newPrice: 0, newWholesaler: '' });
        }
    });
    await sendUpdatePayload(payload);
}

function applyFiltersAndRender() {
    let dataToRender = fullPricingData;
    if (unregisteredFilterCheckbox.checked) {
        dataToRender = dataToRender.filter(p => !p.supplierWholesale);
    }
    const filterText = makerFilterInput.value.trim().toLowerCase();
    if (filterText) {
        dataToRender = dataToRender.filter(p => 
            p.makerName && p.makerName.toLowerCase().includes(filterText)
        );
    }
    renderComparisonTable(dataToRender); 
}

async function handleUpload() {
    const files = Array.from(uploadInput.files);
    if (files.length === 0) return;
    window.showLoading();
    try {
        const formData = new FormData();
        const wholesalerNames = [];
        const processedFiles = [];

        files.forEach(file => {
            const match = file.name.match(/^(\d+)_/);
            const priority = match ? parseInt(match[1], 10) : Infinity;
            
            // ファイル名から卸名部分を抽出（例: 1_価格見積依頼_卸名_... -> 卸名）
            const nameParts = file.name.replace(/^\d+_/, '').split('_');
            if (nameParts.length > 1) {
                 processedFiles.push({ file: file, priority: priority, wholesalerName: nameParts[1] });
            } else {
                 window.showNotification(`ファイル名から卸名を抽出できませんでした: ${file.name}`, 'error');
            }
        });

        // 優先順位でファイルをソート
        processedFiles.sort((a, b) => a.priority - b.priority);

        // ソートされた順にFormDataに追加
        processedFiles.forEach(item => {
            formData.append('files', item.file);
            wholesalerNames.push(item.wholesalerName);
        });

        if (formData.getAll('files').length === 0) {
            throw new Error('処理できる有効なファイルがありませんでした。');
        }
        
        wholesalerNames.forEach(name => formData.append('wholesalerNames', name));

        const res = await fetch('/api/pricing/upload', {
            method: 'POST',
            body: formData,
        });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'アップロード処理に失敗しました。'); 
        }
        
        const responseData = await res.json();
        fullPricingData = responseData.productData;
        orderedWholesalers = responseData.wholesalerOrder;
        applyFiltersAndRender();
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        uploadInput.value = '';
    }
}

async function handleExport(unregisteredOnly = false) {
    const wholesalerSelectEl = document.getElementById('pricing-wholesaler-select');
    const selectedWholesalerName = wholesalerSelectEl.options[wholesalerSelectEl.selectedIndex].text;
    if (!wholesalerSelectEl.value) {
        window.showNotification('テンプレートを出力する卸業者を選択してください。', 'error'); 
        return;
    }

    const date = new Date();
    const dateStr = `${date.getFullYear()}${(date.getMonth()+1).toString().padStart(2, '0')}${date.getDate().toString().padStart(2, '0')}`;
    
    const params = new URLSearchParams({
        wholesalerName: selectedWholesalerName,
        unregisteredOnly: unregisteredOnly,
        date: dateStr,
    });

    window.location.href = `/api/pricing/export?${params.toString()}`;
}

async function loadInitialMasters() {
    outputContainer.innerHTML = '<p>製品マスターを読み込んでいます...</p>';
    window.showLoading();
    try {
        const res = await fetch('/api/pricing/all_masters');
        if (!res.ok) throw new Error('製品マスターの読み込みに失敗しました。');
        const responseData = await res.json();
        fullPricingData = responseData;
        orderedWholesalers = [];
        applyFiltersAndRender();
    } catch(err) {
        outputContainer.innerHTML = `<p style="color:red;">${err.message}</p>`;
    } finally {
        window.hideLoading();
    }
}

function loadWholesalerDropdown() {
    wholesalerSelect.innerHTML = '<option value="">選択してください</option>';
    for (const [code, name] of wholesalerMap.entries()) {
        const opt = document.createElement('option');
        opt.value = code; 
        opt.textContent = name; 
        wholesalerSelect.appendChild(opt); 
    }
    if (lastSelectedWholesaler) {
        wholesalerSelect.value = lastSelectedWholesaler;
    }
}

function handleSetLowestPrice() {
    const rows = outputContainer.querySelectorAll('tbody tr');
    if (rows.length === 0) return;
    const wholesalerReverseMap = new Map();
    for (const [code, name] of wholesalerMap.entries()) {
        wholesalerReverseMap.set(name, code);
    }

    rows.forEach(row => {
        const productCode = row.dataset.productCode;
        const productData = fullPricingData.find(p => p.productCode === productCode);
        if (!productData || !productData.quotes) return;

        let lowestPrice = Infinity;
        let bestWholesalerName = '';
        
        orderedWholesalers.forEach(wholesalerName => {
            const price = productData.quotes[wholesalerName];
 
            if (price !== undefined && price < lowestPrice) {
                lowestPrice = price;
                bestWholesalerName = wholesalerName;
            }
        });

        if (bestWholesalerName) {
            const bestWholesalerCode = wholesalerReverseMap.get(bestWholesalerName);
            const supplierSelect = row.querySelector('.supplier-select');
            const priceInput = row.querySelector('.manual-price-input');
            
            if(supplierSelect) supplierSelect.value = bestWholesalerCode;
            if(priceInput) priceInput.value = lowestPrice.toFixed(2);
        }
    });
    window.showNotification('すべての品目を最安値に設定しました。', 'success');
}

function handleSupplierChange(event) {
    if (!event.target.classList.contains('supplier-select')) return;
    const row = event.target.closest('tr');
    const productCode = row.dataset.productCode;
    const selectedWholesalerName = event.target.options[event.target.selectedIndex].text;
    const priceInput = row.querySelector('.manual-price-input');
    const productData = fullPricingData.find(p => p.productCode === productCode);
    if (productData && productData.quotes) {
        const newPrice = productData.quotes[selectedWholesalerName];
        if (newPrice !== undefined) {
            priceInput.value = newPrice.toFixed(2);
        } else {
            priceInput.value = '';
        }
    }
}

export function initPricingView() {
    view = document.getElementById('pricing-view');
    if (!view) return;
    wholesalerSelect = document.getElementById('pricing-wholesaler-select');
    exportBtn = document.getElementById('pricing-export-btn');
    uploadInput = document.getElementById('pricing-upload-input'); 
    bulkUpdateBtn = document.getElementById('pricing-bulk-update-btn'); 
    outputContainer = document.getElementById('pricing-output-container');
    makerFilterInput = document.getElementById('pricing-maker-filter');
    setLowestPriceBtn = document.getElementById('set-lowest-price-btn');
    unregisteredFilterCheckbox = document.getElementById('pricing-unregistered-filter');
    exportUnregisteredBtn = document.getElementById('pricing-export-unregistered-btn');

    // ▼▼▼ Add these two lines ▼▼▼
    const directImportBtn = document.getElementById('pricing-direct-import-btn');
    const directImportInput = document.getElementById('pricing-direct-import-input');
    // ▲▲▲ Addition complete ▲▲▲    

    view.addEventListener('show', () => {
        loadWholesalerDropdown();
        loadInitialMasters();
    });

    // ▼▼▼ Add these event listeners ▼▼▼
    directImportBtn.addEventListener('click', () => {
        directImportInput.click();
    });

    directImportInput.addEventListener('change', async (event) => {
        const file = event.target.files[0];
        if (!file) return;

        if (!confirm('選択したファイルの内容で納入価と卸情報を一括更新します。この操作は元に戻せません。よろしいですか？')) {
            event.target.value = ''; // Reset the file input
            return;
        }

        const formData = new FormData();
        formData.append('file', file);

        window.showLoading();
        try {
            const res = await fetch('/api/pricing/direct_import', {
                method: 'POST',
                body: formData,
            });
            const resData = await res.json();
            if (!res.ok) {
                throw new Error(resData.message || 'インポートに失敗しました。');
            }
            window.showNotification(resData.message, 'success');
            loadInitialMasters(); // Refresh the main table with updated data
        } catch (err) {
            console.error(err);
            window.showNotification(`エラー: ${err.message}`, 'error');
        } finally {
            window.hideLoading();
            event.target.value = ''; // Reset the file input
        }
    });
    // ▲▲▲ Addition complete ▲▲▲

    uploadInput.addEventListener('change', handleUpload); 
    bulkUpdateBtn.addEventListener('click', handleBulkUpdate); 
    setLowestPriceBtn.addEventListener('click', handleSetLowestPrice);
    
    exportBtn.addEventListener('click', () => handleExport(false)); 
    exportUnregisteredBtn.addEventListener('click', () => handleExport(true));

    makerFilterInput.addEventListener('input', applyFiltersAndRender);
    unregisteredFilterCheckbox.addEventListener('change', applyFiltersAndRender);

    outputContainer.addEventListener('change', handleSupplierChange);
    wholesalerSelect.addEventListener('change', () => {
        lastSelectedWholesaler = wholesalerSelect.value;
    });
}