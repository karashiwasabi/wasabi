// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\inout_modal.js

import { hiraganaToKatakana } from './utils.js';

let activeCallback = null;
let activeRowElement = null;
let skipQueryLengthCheck = false; 

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
  const query = hiraganaToKatakana(searchInput.value.trim());
  if (!skipQueryLengthCheck && query.length < 2) {
    alert('検索キーワードを2文字以上入力してください。');
    searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">2文字以上入力して検索してください。</td></tr>';
    return;
  }
  
  const searchApi = modal.dataset.searchApi || DEFAULT_SEARCH_API;
  searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">検索中...</td></tr>';

  try {
    const separator = searchApi.includes('?') ? '&' : '?';
    const fullUrl = `${searchApi}${separator}q=${encodeURIComponent(query)}`;
    const res = await fetch(fullUrl);

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

  // ▼▼▼【ここから修正】▼▼▼
  let html = '';
  products.forEach(p => {
    const productData = JSON.stringify(p);
    const rowClass = p.isAdopted ? 'class="adopted-item"' : '';
    html += `
      <tr ${rowClass}>
        <td class="left">${p.productName || ''}</td>
        <td class="left">${p.makerName || ''}</td>
        <td class="left">${p.formattedPackageSpec}</td>
        <td>${p.yjCode || ''}</td>
        <td>${p.productCode || ''}</td>
        <td><button class="select-product-btn" data-product='${productData.replace(/'/g, "&apos;")}'>選択</button></td>
      </tr>
    `;
  });
  // ▲▲▲【修正ここまで】▲▲▲
  searchResultsBody.innerHTML = html;
}

export function initModal() {
  if (!modal || !closeModalBtn || !searchInput || !searchBtn || !searchResultsBody) {
    console.error("薬品検索モーダルの必須要素が見つかりません。");
    return;
  }
  closeModalBtn.addEventListener('click', hideModal);
  searchBtn.addEventListener('click', () => performSearch());
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
    
    skipQueryLengthCheck = options.skipQueryLengthCheck || false;
    searchInput.placeholder = skipQueryLengthCheck ? '製品名またはカナ名（絞り込みフィルタ有効）' : '製品名またはカナ名（2文字以上）';
    
    const searchApi = options.searchApi || DEFAULT_SEARCH_API;
    modal.dataset.searchApi = searchApi;
    
    modal.classList.remove('hidden');
    searchInput.value = '';
    setTimeout(() => {
        searchInput.focus();
    }, 0);

    if (options.initialResults) {
        renderSearchResults(options.initialResults);
    } else {
        searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">製品名を入力して検索してください。</td></tr>';
    }
  }
}