let onProductSelectCallback = null;
let activeRowElement = null;

const modal = document.getElementById('search-modal');
const closeModalBtn = document.getElementById('closeModalBtn');
const searchInput = document.getElementById('product-search-input');
const searchBtn = document.getElementById('product-search-btn');
const searchResultsBody = document.querySelector('#search-results-table tbody');

function handleResultClick(event) {
  if (event.target && event.target.classList.contains('select-product-btn')) {
    const product = JSON.parse(event.target.dataset.product);
    if (typeof onProductSelectCallback === 'function') {
      onProductSelectCallback(product, activeRowElement);
    }
    modal.classList.add('hidden');
  }
}

async function performSearch() {
  const query = searchInput.value.trim();
  if (query.length < 2) {
    alert('2文字以上入力してください。');
    return;
  }
  searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">検索中...</td></tr>';
  try {
    const res = await fetch(`/api/products/search?q=${encodeURIComponent(query)}`);
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

export function initModal(onSelect) {
  if (!modal || !closeModalBtn || !searchInput || !searchBtn || !searchResultsBody) {
    console.error("薬品検索モーダルの必須要素が見つかりません。");
    return;
  }
  onProductSelectCallback = onSelect;

  closeModalBtn.addEventListener('click', () => modal.classList.add('hidden'));
  searchBtn.addEventListener('click', performSearch);
  searchInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      performSearch();
    }
  });
  searchResultsBody.addEventListener('click', handleResultClick);
}

export function showModal(rowElement) {
  if (modal) {
    activeRowElement = rowElement;
    modal.classList.remove('hidden');
    searchInput.value = '';
    searchInput.focus();
    searchResultsBody.innerHTML = '<tr><td colspan="6" class="center">製品名を入力して検索してください。</td></tr>';
  }
}