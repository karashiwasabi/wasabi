import { clientMap, wholesalerMap } from './master_data.js';

export const transactionTypeMap = { 
    0: "棚卸", 1: "納品", 2: "返品", 3: "処方", 4: "棚卸増", 
    5: "棚卸減", 11: "入庫", 12: "出庫", 30: "月末", 
};

function getClientOrWholesalerName(rec) { 
    if (!rec.clientCode) return '';
    if (rec.flag === 1 || rec.flag === 2) { 
        return wholesalerMap.get(rec.clientCode) || rec.clientCode;
    }
    return clientMap.get(rec.clientCode) || rec.clientCode;
}

/**
 * [修正点]
 * 汎用性を高めるため、tbodyの中身を空にして返すように変更。
 * 呼び出し元で中身を結合して使用する。
 */
export function createUploadTableHTML(tableId) { 
  const colgroup = `
    <colgroup>
      <col class="col-1"><col class="col-2"><col class="col-3"><col class="col-4"><col class="col-5">
      <col class="col-6"><col class="col-7"><col class="col-8"><col class="col-9"><col class="col-10">
      <col class="col-11"><col class="col-12"><col class="col-13"><col class="col-14">
    </colgroup>
  `;
  const header = `
    <thead>
      <tr>
        <th rowspan="2">－</th>
        <th>日付</th><th class="yj-jan-code">YJ</th><th colspan="2">製品名</th>
        <th>個数</th><th>YJ数量</th><th>YJ包装数</th><th>YJ単位</th>
        <th>単価</th><th>税額</th><th>期限</th><th>得意先</th><th>行</th>
      </tr>
      <tr>
        <th>種別</th><th class="yj-jan-code">JAN</th><th>包装</th><th>メーカー</th>
        <th>剤型</th><th>JAN数量</th><th>JAN包装数</th><th>JAN単位</th>
        <th>金額</th><th>税率</th><th>ロット</th><th>伝票番号</th><th>MA</th>
      </tr>
    </thead>
  `;
  // 中身が空のtbodyを持つテーブルの骨格を返す
  return `<table id="${tableId}" class="data-table">${colgroup}${header}<tbody></tbody></table>`;
}

/**
 * [修正点]
 * DOMを直接操作するのをやめ、テーブルの中身となるHTML文字列を生成して返すように変更。
 * 引数からtableIdを削除。
 * @param {Array} records - 表示する取引記録の配列
 * @returns {string} 生成された <tr>...</tr> のHTML文字列
 */
export function renderUploadTableRows(records) {
  if (!records || records.length === 0) { 
    return `<tr><td colspan="14">対象データがありません。</td></tr>`;
  }
  
  const getTxClass = (flag) => {
    switch (flag) {
      case 2:
        return 'tx-return';
      case 0:
      case 4:
      case 5:
        return 'tx-inventory';
      default:
        return '';
    }
  };

  let html = "";
  records.forEach(rec => {
    const rowClass = getTxClass(rec.flag);
    html += `
      <tr class="${rowClass}">
        <td rowspan="2"></td>
        <td>${rec.transactionDate || ""}</td>
        <td class="yj-jan-code">${rec.yjCode || ""}</td>
        <td class="left" colspan="2">${rec.productName || ""}</td>
        <td class="right">${rec.datQuantity?.toFixed(2) || ""}</td>
        <td class="right">${rec.yjQuantity?.toFixed(2) || ""}</td>
        <td class="right">${rec.yjPackUnitQty || ""}</td>
        <td>${rec.yjUnitName || ""}</td>
        <td class="right">${rec.unitPrice?.toFixed(4) || ""}</td>
        <td class="right">${rec.taxAmount?.toFixed(2) || ""}</td>
        <td>${rec.expiryDate || ""}</td>
        <td class="left">${getClientOrWholesalerName(rec)}</td>
        <td class="right">${rec.lineNumber || ""}</td>
      </tr>
      <tr class="${rowClass}">
        <td>${transactionTypeMap[rec.flag] ?? ""}</td>
        <td class="yj-jan-code">${rec.janCode || ""}</td>
        <td class="left">${rec.packageSpec || ""}</td>
        <td class="left">${rec.makerName || ""}</td>
        <td class="left">${rec.usageClassification || ""}</td>
        <td class="right">${rec.janQuantity?.toFixed(2) || ""}</td>
        <td class="right">${rec.janPackUnitQty || ""}</td>
        <td>${rec.janUnitName || ""}</td>
        <td class="right">${rec.subtotal?.toFixed(2) || ""}</td>
        <td class="right">${rec.taxRate != null ? (rec.taxRate * 100).toFixed(0) + "%" : ""}</td>
        <td class="left">${rec.lotNumber || ""}</td>
        <td class="left">${rec.receiptNumber || ""}</td>
        <td class="left">${rec.processFlagMA || ""}</td>
      </tr>
    `;
  });
  
  // DOM操作の代わりにHTML文字列を返す
  return html;
}

export function setupDateDropdown(inputEl) { 
  if (!inputEl) return; 
  inputEl.value = new Date().toISOString().slice(0, 10);
}

export async function setupClientDropdown(selectEl) { 
  if (!selectEl) return; 
  const preservedOptions = Array.from(selectEl.querySelectorAll('option[value=""]')); 
  selectEl.innerHTML = ''; 
  preservedOptions.forEach(opt => selectEl.appendChild(opt));
  try { 
    const res = await fetch('/api/clients'); 
    if (!res.ok) throw new Error('Failed to fetch clients');
    const clients = await res.json();
    if (clients) { 
      clients.forEach(c => { 
        const opt = document.createElement('option'); 
        opt.value = c.code; 
        opt.textContent = `${c.code}:${c.name}`; 
        selectEl.appendChild(opt); 
      });
    }
  } catch (err) { 
    console.error("得意先リストの取得に失敗:", err);
  }
}