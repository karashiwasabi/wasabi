import { wholesalerMap } from './master_data.js';

let view, wholesalerSelect, exportBtn, uploadInput, bulkUpdateBtn, outputContainer, makerFilterInput;
let fullPricingData = []; 
let orderedWholesalers = []; // 卸の順序を保持する変数を追加
let setLowestPriceBtn;
let unregisteredFilterCheckbox; // ★ 新しいフィルター用の変数を追加
let exportUnregisteredBtn; // ★ 新しいエクスポートボタン用の変数を追加

/**
 * テーブルを描画する関数
 */
function renderComparisonTable(data) {
    if (!data || data.length === 0) {
        outputContainer.innerHTML = '<p>表示対象のデータが見つかりませんでした。</p>';
        return;
    }

    // 順序が保証されないSetではなく、サーバーから受け取った順序リストを使用する
    const wholesalerHeaders = orderedWholesalers.length > 0 ? orderedWholesalers.map(w => `<th>${w}</th>`).join('') : '';
    
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
        
        // 順序リストを元にプルダウンを生成
        orderedWholesalers.forEach(wName => {
            const wCode = wholesalerReverseMap.get(wName) || '';
            const isSelected = (wCode === p.supplierWholesale);
            wholesalerOptions += `<option value="${wCode}" ${isSelected ? 'selected' : ''}>${wName}</option>`;
        });

        // 順序リストを元に価格セルを生成
        const quoteCells = orderedWholesalers.length > 0 ? orderedWholesalers.map(w => {
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

/**
 * サーバーに更新データを送信し、成功なら画面を再描画する共通関数
 */
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


/**
 * 「一括更新」ボタンが押されたときの処理
 */
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

/**
 * フィルターを適用してテーブルを再描画する
 */
function applyFiltersAndRender() {
    let dataToRender = fullPricingData;
    // 「未登録のみ」フィルターの適用
    if (unregisteredFilterCheckbox.checked) {
        dataToRender = dataToRender.filter(p => !p.supplierWholescale);
    }

    // メーカー名フィルターの適用
    const filterText = makerFilterInput.value.trim().toLowerCase();
    if (filterText) {
        dataToRender = dataToRender.filter(p => 
            p.makerName && p.makerName.toLowerCase().includes(filterText)
        );
    }
    
    renderComparisonTable(dataToRender); 
}

/**
 * ファイルアップロード時の処理
 */
async function handleUpload() {
    const files = uploadInput.files;
    if (files.length === 0) return;
    window.showLoading();
    try {
        const formData = new FormData();
        const fileList = Array.from(files);
        fileList.sort((a, b) => {
            const aNum = parseInt(a.name.split('_')[0], 10);
            const bNum = parseInt(b.name.split('_')[0], 10);
            if (!isNaN(aNum) && !isNaN(bNum)) return aNum - bNum;
            return a.name.localeCompare(b.name);
        });
        const wholesalerNames = [];
        for (const file of fileList) {
            const parts = file.name.split('_');
            if (parts.length >= 3 && parts[1] === '価格見積依頼') {
                formData.append('files', file);
                wholesalerNames.push(parts[2]);
            } else {
                 window.showNotification(`ファイル名が不正です: ${file.name} (形式: 1_価格見積依頼_卸名_日付.csv)`, 'error');
            }
        }
        
        if(formData.getAll('files').length === 0) throw new Error('処理できる有効なファイルがありませんでした。');
        formData.delete('wholesalerNames');
        wholesalerNames.forEach(name => formData.append('wholesalerNames', name)); 

        const res = await fetch('/api/pricing/upload', {
            method: 'POST',
            body: formData,
        });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'アップロード処理に失敗しました。'); 
        }
        
        // サーバーから返されたデータ（マスター＋見積もり）で既存のデータを更新
        const responseData = await res.json();
        fullPricingData = responseData.productData;
        orderedWholesalers = responseData.wholesalerOrder; // 卸の順序を保存
        applyFiltersAndRender(); 
        
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        uploadInput.value = '';
    }
}

/**
 * 見積もり依頼CSVをエクスポートする
 */
async function handleExport(unregisteredOnly = false) {
    let dataToExport = fullPricingData;
    let fileNameSuffix = "全品";
    // 未登録品のみをエクスポートする場合のフィルタリング
    if (unregisteredOnly) {
        dataToExport = fullPricingData.filter(p => !p.supplierWholesale);
        fileNameSuffix = "未登録品";
        if (dataToExport.length === 0) {
            window.showNotification('エクスポート対象の未登録品がありません。', 'error');
            return;
        }
    }
    
    const selectedWholesalerName = wholesalerSelect.options[wholesalerSelect.selectedIndex].text;
    if (!wholesalerSelect.value) {
        window.showNotification('テンプレートを出力する卸業者を選択してください。', 'error'); 
        return;
    }

    window.showLoading();
    try {
        const header = ["product_code", "product_name", "maker_name", "package_spec", "purchase_price"];
        const csvRows = dataToExport.map(d => 
            [ `"${d.productCode}"`, `"${d.productName}"`, `"${d.makerName}"`, `"${d.formattedPackageSpec}"`, `""` ].join(',')
        );
        const csvContent = [header.join(','), ...csvRows].join('\r\n'); 
        const blob = new Blob([new Uint8Array([0xEF, 0xBB, 0xBF]), csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement("a"); 
        const date = new Date();
        const dateStr = `${date.getFullYear()}${(date.getMonth()+1).toString().padStart(2, '0')}${date.getDate().toString().padStart(2, '0')}`;
        const fileName = `価格見積依頼_${selectedWholesalerName}_${fileNameSuffix}_${dateStr}.csv`;
        link.setAttribute("href", URL.createObjectURL(blob)); 
        link.setAttribute("download", fileName); 
        link.style.visibility = 'hidden'; 
        document.body.appendChild(link); 
        link.click(); 
        document.body.removeChild(link); 
        window.showNotification('CSVファイルをエクスポートしました。', 'success');
    } catch (err) {
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
    }
}

/**
 * 画面表示時に最初にマスターデータを読み込む関数
 */
async function loadInitialMasters() {
    outputContainer.innerHTML = '<p>製品マスターを読み込んでいます...</p>';
    window.showLoading();
    try {
        const res = await fetch('/api/pricing/all_masters');
        if (!res.ok) throw new Error('製品マスターの読み込みに失敗しました。');
        const responseData = await res.json();
        fullPricingData = responseData;
        orderedWholesalers = []; // 初期表示では見積もりがないので卸リストは空
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

/**
 * 価格更新ビューの初期化
 */
export function initPricingView() {
    view = document.getElementById('pricing-view');
    if (!view) return;
    // DOM要素の取得
    wholesalerSelect = document.getElementById('pricing-wholesaler-select');
    exportBtn = document.getElementById('pricing-export-btn');
    uploadInput = document.getElementById('pricing-upload-input'); 
    bulkUpdateBtn = document.getElementById('pricing-bulk-update-btn'); 
    outputContainer = document.getElementById('pricing-output-container');
    makerFilterInput = document.getElementById('pricing-maker-filter');
    setLowestPriceBtn = document.getElementById('set-lowest-price-btn');
    unregisteredFilterCheckbox = document.getElementById('pricing-unregistered-filter');
    exportUnregisteredBtn = document.getElementById('pricing-export-unregistered-btn');
    
    // イベントリスナーの設定
    view.addEventListener('show', () => {
        loadWholesalerDropdown();
        loadInitialMasters(); // 最初に全マスターを読み込む
    });
    
    uploadInput.addEventListener('change', handleUpload); 
    bulkUpdateBtn.addEventListener('click', handleBulkUpdate); 
    setLowestPriceBtn.addEventListener('click', handleSetLowestPrice);
    
    exportBtn.addEventListener('click', () => handleExport(false)); // 全品エクスポート
    exportUnregisteredBtn.addEventListener('click', () => handleExport(true)); // 未登録品エクスポート

    makerFilterInput.addEventListener('input', applyFiltersAndRender);
    unregisteredFilterCheckbox.addEventListener('change', applyFiltersAndRender);

    outputContainer.addEventListener('change', handleSupplierChange);
}