// C:\Dev\WASABI\static\js\inout_modal.js

import { hiraganaToKatakana } from './utils.js';

let activeCallback = null;
let activeRowElement = null;

const DEFAULT_SEARCH_API = '/api/products/search';
const modal = document.getElementById('search-modal');
const closeModalBtn = document.getElementById('closeModalBtn');
const searchInput = document.getElementById('product-search-input');
const searchBtn = document.getElementById('product-search-btn');
const searchResultsBody = document.querySelector('#search-results-table tbody');

function hideModal() {
    if (modal) {
        modal.classList.add('hidden');
        document.body.classList.remove('modal-open');
    }
}

function handleResultClick(event) {
  if (event.target && event.target.classList.contains('select-product-btn')) {
    const product = JSON.parse(event.target.dataset.product);
    if (typeof activeCallback === 'function') {
      activeCallback(product, activeRowElement);
    }
    hideModal();
  }
}

async function performSearch() {
  // ▼▼▼ [修正点] 検索実行時にカナ変換 ▼▼▼
  const query = hiraganaToKatakana(searchInput.value.trim());
  // ▲▲▲ 修正ここまで ▲▲▲
   // ▼▼▼【ここから修正】▼▼▼
  // 検索文字が2文字未満の場合はAPIを呼び出さずに処理を中断する
  if (query.length < 2) {
    alert('検索キーワードを2文字以上入力してください。');
    // 検索結果をクリアしてメッセージを表示
    searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">2文字以上入力して検索してください。</td></tr>';
    return;
  }
  // ▲▲▲【修正ここまで】▲▲▲
  const searchApi = modal.dataset.searchApi || DEFAULT_SEARCH_API;
  searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">検索中...</td></tr>';
  
  // ▼▼▼ [修正点] URLの組み立てロジックを修正 ▼▼▼
  try {
    const separator = searchApi.includes('?') ? '&' : '?';
    const fullUrl = query ? `${searchApi}${separator}q=${encodeURIComponent(query)}` : searchApi;
    const res = await fetch(fullUrl);
  // ▲▲▲ 修正ここまで ▲▲▲

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

export function initModal() {
  if (!modal || !closeModalBtn || !searchInput || !searchBtn || !searchResultsBody) {
    console.error("薬品検索モーダルの必須要素が見つかりません。");
    return;
  }
  closeModalBtn.addEventListener('click', hideModal);
  searchBtn.addEventListener('click', performSearch);
  searchInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      performSearch();
    }
  });
  searchResultsBody.addEventListener('click', handleResultClick);
}

export function showModal(rowElement, callback, options = {}) {
  if (modal) {
    document.body.classList.add('modal-open');
    activeRowElement = rowElement;
    activeCallback = callback; 
    
    const searchApi = options.searchApi || DEFAULT_SEARCH_API;
    modal.dataset.searchApi = searchApi;
    
    modal.classList.remove('hidden');
    searchInput.value = '';
    // ▼▼▼ [修正点] focus()をsetTimeoutで囲む ▼▼▼
    setTimeout(() => {
        searchInput.focus();
    }, 0);
    // ▲▲▲ 修正ここまで ▲▲▲

    if (options.initialResults) {
        renderSearchResults(options.initialResults);
    } else {
        searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">製品名を入力して検索してください。</td></tr>';
    }
  }
}