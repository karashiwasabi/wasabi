// C:\Dev\WASABI\static\js\inout_modal.js

let activeCallback = null;
let activeRowElement = null;

const DEFAULT_SEARCH_API = '/api/products/search';
const modal = document.getElementById('search-modal');
const closeModalBtn = document.getElementById('closeModalBtn');
const searchInput = document.getElementById('product-search-input');
const searchBtn = document.getElementById('product-search-btn');
const searchResultsBody = document.querySelector('#search-results-table tbody');

// ▼▼▼ [修正点] モーダルを閉じる処理を共通関数化 ▼▼▼
function hideModal() {
    if (modal) {
        modal.classList.add('hidden');
        document.body.classList.remove('modal-open'); // bodyからクラスを削除
    }
}
// ▲▲▲ 修正ここまで ▲▲▲

function handleResultClick(event) {
  if (event.target && event.target.classList.contains('select-product-btn')) {
    const product = JSON.parse(event.target.dataset.product);
    if (typeof activeCallback === 'function') {
      activeCallback(product, activeRowElement);
    }
    // ▼▼▼ [修正点] 共通のhideModal関数を呼び出す ▼▼▼
    hideModal();
    // ▲▲▲ 修正ここまで ▲▲▲
  }
}

async function performSearch() {
  const query = searchInput.value.trim();
  if (query.length < 2) {
    alert('2文字以上入力してください。');
    return;
  }
  const searchApi = modal.dataset.searchApi || DEFAULT_SEARCH_API; // 保存したAPIエンドポイントを取得
  searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">検索中...</td></tr>';
  try {
    const res = await fetch(`${searchApi}?q=${encodeURIComponent(query)}`);
    if (!res.ok) {
        throw new Error(`サーバーエラー: ${res.status}`);
    }
    const products = await res.json();
    renderSearchResults(products);
  } catch (err) {
    searchResultsBody.innerHTML = `<tr><td colspan="6" class="center" style="color:red;">${err.message}</td></tr>`;
  }
}

function renderSearchResults(products) {
  if (!products || products.length === 0) {
    searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">該当する製品が見つかりません。</td></tr>';
    return;
  }

  let html = '';
  products.forEach(p => {
    const productData = JSON.stringify(p);
    html += `
      <tr>
        <td class="left">${p.productName || ''}</td>
        <td class="left">${p.makerName || ''}</td>
        <td class="left">${p.formattedPackageSpec}</td>
        <td>${p.yjCode || ''}</td>
        <td>${p.productCode || ''}</td>
        <td><button class="select-product-btn" data-product='${productData.replace(/'/g, "&apos;")}'>選択</button></td>
      </tr>
    `;
  });
  searchResultsBody.innerHTML = html;
}

// ▼▼▼ [修正点] 起動時に一度だけイベントリスナーを設定するinit関数に変更 ▼▼▼
export function initModal() {
  if (!modal || !closeModalBtn || !searchInput || !searchBtn || !searchResultsBody) {
    console.error("薬品検索モーダルの必須要素が見つかりません。");
    return;
  }
  // ▼▼▼ [修正点] 共通のhideModal関数を呼び出すように変更 ▼▼▼
  closeModalBtn.addEventListener('click', hideModal);
  // ▲▲▲ 修正ここまで ▲▲▲
  searchBtn.addEventListener('click', performSearch);
  searchInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      performSearch();
    }
  });
  searchResultsBody.addEventListener('click', handleResultClick);
}
// ▲▲▲ 修正ここまで ▲▲▲

export function showModal(rowElement, callback, searchApi = DEFAULT_SEARCH_API) {
  if (modal) {
    // ▼▼▼ [修正点] モーダル表示時にbodyにクラスを追加 ▼▼▼
    document.body.classList.add('modal-open');
    // ▲▲▲ 修正ここまで ▲▲▲
    activeRowElement = rowElement;
    activeCallback = callback; 
    modal.dataset.searchApi = searchApi; // APIエンドポイントを保存
    modal.classList.remove('hidden');
    searchInput.value = '';
    searchInput.focus();
    searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">製品名を入力して検索してください。</td></tr>';
  }
}