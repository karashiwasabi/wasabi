// C:\Dev\WASABI\static\js\master_edit.js

import { showModal } from './inout_modal.js';

let view, tableContainer, refreshBtn, addRowBtn;
let unitMap = {};

async function fetchUnitMap(){if(Object.keys(unitMap).length>0)return;try{const res=await fetch('/api/units/map');if(!res.ok)throw new Error('単位マスタの取得に失敗');unitMap=await res.json();}catch(err){console.error(err);window.showNotification(err.message,'error');}}
function formatPackageSpecForRow(tbody){if(!tbody)return;const getVal=(name)=>tbody.querySelector(`[name="${name}"]`)?.value||'';const packageForm=getVal('packageForm');const yjPackUnitQty=getVal('yjPackUnitQty');const yjUnitName=getVal('yjUnitName');const janPackInnerQty=getVal('janPackInnerQty');const janPackUnitQty=getVal('janPackUnitQty');const janUnitCode=getVal('janUnitCode');let formattedSpec=`${packageForm} ${yjPackUnitQty}${yjUnitName}`;if(parseFloat(janPackInnerQty)>0&&parseFloat(janPackUnitQty)>0){let janUnitName=(janUnitCode==='0'||janUnitCode==='')?'':(unitMap[janUnitCode]||'');formattedSpec+=` (${janPackInnerQty}${yjUnitName}×${janPackUnitQty}${janUnitName})`;}
const targetCell=tbody.querySelector('.formatted-spec-cell');if(targetCell)targetCell.textContent=formattedSpec;}
function createMasterRowHTML(master={}){const isNew=!master.productCode;const rowId=master.productCode||`new-${Date.now()}`;const topRowFields=[{key:'productCode',ph:'製品コード(JAN)',readonly:!isNew},{key:'yjCode',ph:'YJコード'},{key:'productName',ph:'製品名'},{key:'kanaName',ph:'カナ名'},{key:'makerName',ph:'メーカー名'},{key:'usageClassification',ph:'剤型(JC013)'},{key:'nhiPrice',ph:'薬価',type:'number',step:'0.0001',value:master.nhiPrice||0},];const flags=[{key:'flagPoison',ph:'毒',opts:[0,1]},{key:'flagDeleterious',ph:'劇',opts:[0,1]},{key:'flagNarcotic',ph:'麻',opts:[0,1]},{key:'flagPsychotropic',ph:'向',opts:[0,1,2,3]},{key:'flagStimulant',ph:'覚',opts:[0,1]},{key:'flagStimulantRaw',ph:'覚原',opts:[0,1]},];const bottomRowFields=[{key:'packageForm',ph:'包装(JC037)'},{key:'yjUnitName',ph:'YJ単位'},{key:'yjPackUnitQty',ph:'YJ包装数量',type:'number',value:master.yjPackUnitQty||0},{key:'janPackInnerQty',ph:'内包装数量',type:'number',value:master.janPackInnerQty||0},{key:'janUnitCode',ph:'JAN単位',type:'select',value:master.janUnitCode||0},{key:'janPackUnitQty',ph:'JAN包装数量',type:'number',value:master.janPackUnitQty||0},];let topRowCellsHTML=topRowFields.map(f=>`<td><input type="${f.type||'text'}" name="${f.key}" value="${f.value||master[f.key]||''}" placeholder="${f.ph}" ${f.readonly?'readonly':''} ${f.step?`step="${f.step}"`:''}></td>`).join('');topRowCellsHTML+=flags.map(f=>{const options=f.opts.map(o=>`<option value="${o}" ${o==(master[f.key]||0)?'selected':''}>${o}</option>`).join('');return`<td><select name="${f.key}">${options}</select></td>`;}).join('');topRowCellsHTML+=`<td><button class="save-master-btn btn">保存</button></td>`;let bottomRowCellsHTML=bottomRowFields.map(f=>{if(f.type==='select'){let options=`<option value="0">YJ単位と同じ</option>`;for(const[code,name]of Object.entries(unitMap)){if(code!=='0')options+=`<option value="${code}" ${code==f.value?'selected':''}>${name}</option>`;}
return`<td><select name="${f.key}">${options}</select></td>`;}
return`<td><input type="${f.type||'text'}" name="${f.key}" value="${f.value||master[f.key]||''}" placeholder="${f.ph}"></td>`;}).join('');bottomRowCellsHTML+=`<td class="formatted-spec-cell" colspan="7"></td><td><button class="quote-jcshms-btn btn">引用</button></td>`;return`<tbody data-record-id="${rowId}"><tr class="data-row-top">${topRowCellsHTML}</tr><tr class="data-row-bottom">${bottomRowCellsHTML}</tr></tbody>`;}
async function loadAndRenderMasters(){tableContainer.innerHTML=`<table><tbody><tr><td>読み込み中...</td></tr></tbody></table>`;try{const res=await fetch('/api/masters/editable');if(!res.ok)throw new Error('マスターの読み込みに失敗しました。');const masters=await res.json();const tableHeader=`<thead>
            <tr><th>製品コード(JAN)</th><th>YJコード</th><th>製品名</th><th>カナ名</th><th>メーカー名</th><th>剤型</th><th>薬価</th><th>毒</th><th>劇</th><th>麻</th><th>向</th><th>覚</th><th>覚原</th><th>操作</th></tr>
            <tr><th>包装</th><th>YJ単位</th><th>YJ包装数量</th><th>内包装数量</th><th>JAN単位</th><th>JAN包装数量</th><th colspan="7">組み立て包装</th><th>操作</th></tr>
        </thead>`;const tableContent=masters.map(createMasterRowHTML).join('');tableContainer.innerHTML=`<table class="data-table">${tableHeader}${tableContent}</table>`;tableContainer.querySelectorAll('tbody[data-record-id]').forEach(formatPackageSpecForRow);}catch(err){tableContainer.innerHTML=`<p style="color:red;">${err.message}</p>`;}}

function populateFormWithJcshms(selectedProduct, tbody) {
    if (!tbody) return;
    const setVal = (name, value) => {
        const el = tbody.querySelector(`[name="${name}"]`);
        if (el) el.value = value !== undefined ? value : '';
    };

    const productCodeInput = tbody.querySelector('[name="productCode"]');
    if (productCodeInput && !productCodeInput.readOnly) {
        productCodeInput.value = selectedProduct.productCode || '';
    }
    
    setVal('yjCode', selectedProduct.yjCode); setVal('productName', selectedProduct.productName);
    setVal('kanaName', selectedProduct.kanaName); setVal('makerName', selectedProduct.makerName);
    setVal('usageClassification', selectedProduct.usageClassification); setVal('packageForm', selectedProduct.packageForm);
    setVal('nhiPrice', selectedProduct.nhiPrice); setVal('flagPoison', selectedProduct.flagPoison);
    setVal('flagDeleterious', selectedProduct.flagDeleterious); setVal('flagNarcotic', selectedProduct.flagNarcotic);
    setVal('flagPsychotropic', selectedProduct.flagPsychotropic); setVal('flagStimulant', selectedProduct.flagStimulant);
    setVal('flagStimulantRaw', selectedProduct.flagStimulantRaw); setVal('yjUnitName', selectedProduct.yjUnitName);
    setVal('yjPackUnitQty', selectedProduct.yjPackUnitQty); setVal('janPackInnerQty', selectedProduct.janPackInnerQty);
    setVal('janUnitCode', selectedProduct.janUnitCode); setVal('janPackUnitQty', selectedProduct.janPackUnitQty);
    
    formatPackageSpecForRow(tbody);
}

export async function initMasterEdit() {
    view = document.getElementById('master-edit-view');
    if (!view) return;

    tableContainer = document.getElementById('master-edit-container');
    refreshBtn = document.getElementById('refreshMastersBtn');
    addRowBtn = document.getElementById('addMasterRowBtn');

    await fetchUnitMap();
    // ▼▼▼ [修正点] initModalの呼び出しを削除 ▼▼▼
    // initModal(populateFormWithJcshms);
    // ▲▲▲ 修正ここまで ▲▲▲

    refreshBtn.addEventListener('click', loadAndRenderMasters);
    addRowBtn.addEventListener('click', () => {
        const table = tableContainer.querySelector('table');
        if (table) table.insertAdjacentHTML('beforeend', createMasterRowHTML());
    });

    tableContainer.addEventListener('input', (e) => {
        const tbody = e.target.closest('tbody[data-record-id]');
        if (tbody) formatPackageSpecForRow(tbody);
    });

    tableContainer.addEventListener('click', async (e) => {
        const target = e.target;
        const tbody = target.closest('tbody[data-record-id]');
        if (!tbody) return;

        if (target.classList.contains('save-master-btn')) {
            const data = {};
            tbody.querySelectorAll('input, select').forEach(el => {
                const name = el.name;
                const value = el.value;
                if (el.tagName === 'SELECT' || el.type === 'number') {
                    data[name] = parseFloat(value) || 0;
                } else {
                    data[name] = value;
                }
            });
            
            data.packageSpec = data.packageForm;
            data.origin = "MANUAL"; data.purchasePrice = 0; data.supplierWholesale = '';

            if (!data.productCode) {
                window.showNotification('製品コードは必須です。', 'error');
                return;
            }
            
            window.showLoading();
            try {
                const res = await fetch('/api/master/update', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data),
                });
                const resData = await res.json();
                if (!res.ok) throw new Error(resData.message || '保存に失敗しました。');
                
                window.showNotification(resData.message, 'success');
                loadAndRenderMasters();
            } catch (err) {
                window.showNotification(err.message, 'error');
            } finally {
                window.hideLoading();
            }
        }
        // ▼▼▼ [修正点] モーダル呼び出し時に、実行したい処理（コールバック）を直接渡す ▼▼▼
        if (target.classList.contains('quote-jcshms-btn')) {
            showModal(tbody, populateFormWithJcshms);
        }
        // ▲▲▲ 修正ここまで ▲▲▲
    });
}

export function resetMasterEditView() {
    if (tableContainer) {
        loadAndRenderMasters();
    }
}