// C:\Dev\WASABI\static\js\inout_header.js

import { setupDateDropdown, setupClientDropdown } from './common_table.js';
// ▼▼▼ [修正点] refreshClientMap をインポート ▼▼▼
import { refreshClientMap } from './master_data.js';
// ▲▲▲ 修正ここまで ▲▲▲

const NEW_ENTRY_VALUE = '--new--';
let clientSelect, receiptSelect, saveBtn, deleteBtn, headerDateInput, headerTypeSelect;
let newClientName = null;
let currentLoadedReceipt = null;

// (initializeClientDropdown, resetHeader 関数は変更ありません)
async function initializeClientDropdown() {
	clientSelect.innerHTML = `<option value="">選択してください</option>`;
	await setupClientDropdown(clientSelect);
	
	const newOption = document.createElement('option');
	newOption.value = NEW_ENTRY_VALUE;
	newOption.textContent = '--- 新規作成 ---';
	clientSelect.appendChild(newOption);
}

export function resetHeader() {
	if (!clientSelect || !headerDateInput) return;
	setupDateDropdown(headerDateInput);
	initializeClientDropdown();
	
	receiptSelect.innerHTML = `
		<option value="">日付を選択してください</option>
		<option value="${NEW_ENTRY_VALUE}">--- 新規作成 ---</option>
	`;
	headerTypeSelect.value = "入庫";
	newClientName = null;
	currentLoadedReceipt = null;
	deleteBtn.disabled = true;
	headerDateInput.dispatchEvent(new Event('change'));
}

export async function initHeader(getDetailsData, clearDetailsTable, populateDetailsTable) {
	clientSelect = document.getElementById('in-out-client');
	receiptSelect = document.getElementById('in-out-receipt');
	saveBtn = document.getElementById('saveBtn');
	deleteBtn = document.getElementById('deleteBtn');
	headerDateInput = document.getElementById('in-out-date');
	headerTypeSelect = document.getElementById('in-out-type');

	if (!clientSelect || !receiptSelect || !saveBtn || !deleteBtn) return;
	
    // (各種イベントリスナーは変更ありません)
	headerDateInput.addEventListener('change', async () => {
		const date = headerDateInput.value.replace(/-/g, '');
		if (!date) return;
		try {
			const res = await fetch(`/api/receipts?date=${date}`);
			if (!res.ok) throw new Error('伝票の取得に失敗');
			const receiptNumbers = await res.json();
			
			receiptSelect.innerHTML = `
				<option value="">選択してください</option>
				<option value="${NEW_ENTRY_VALUE}">--- 新規作成 ---</option>
			`;
			if (receiptNumbers && receiptNumbers.length > 0) {
				receiptNumbers.forEach(num => {
					const opt = document.createElement('option');
					opt.value = num;
					opt.textContent = num;
					receiptSelect.appendChild(opt);
				});
			}
		} catch (err) { console.error(err); }
	});

	clientSelect.addEventListener('change', () => {
		const selectedValue = clientSelect.value;
		if (selectedValue === NEW_ENTRY_VALUE) {
			const name = prompt('新しい得意先名を入力してください:');
			if (name && name.trim()) {
				newClientName = name.trim();
				const opt = document.createElement('option');
				opt.value = `new:${newClientName}`;
				opt.textContent = `[新規] ${newClientName}`;
				opt.selected = true;
				clientSelect.appendChild(opt);
			} else {
				clientSelect.value = '';
			}
		} else if (!selectedValue.startsWith('new:')) {
			newClientName = null;
		}
	});
	receiptSelect.addEventListener('change', async () => {
		const selectedValue = receiptSelect.value;
		deleteBtn.disabled = (selectedValue === NEW_ENTRY_VALUE || selectedValue === "");

		if (selectedValue === NEW_ENTRY_VALUE || selectedValue === "") {
			clearDetailsTable();
			currentLoadedReceipt = null;
		} else {
			window.showLoading();
			try {
				const res = await fetch(`/api/transaction/${selectedValue}`);
				if (!res.ok) throw new Error('明細の読込に失敗');
				const records = await res.json();
				if (records && records.length > 0) {
					currentLoadedReceipt = selectedValue;
					clientSelect.value = records[0].clientCode;
					headerTypeSelect.value = records[0].flag === 11 ? "入庫" : "出庫";
					newClientName = null;
				}
				populateDetailsTable(records);
			} catch (err) {
				console.error(err);
				window.showNotification(err.message, 'error');
			} finally {
				window.hideLoading();
			}
		}
	});

	saveBtn.addEventListener('click', async () => {
		let clientCode = clientSelect.value;
		let clientNameToSave = '';
		let isNewClient = false;
		if (newClientName && clientCode.startsWith('new:')) {
			clientNameToSave = newClientName;
			isNewClient = true;
			clientCode = '';
		} else {
			if (!clientCode || clientCode === NEW_ENTRY_VALUE) {
				window.showNotification('得意先を選択または新規作成してください。', 'error');
				return;
			}
		}
		const records = getDetailsData();
		if (records.length === 0) {
			window.showNotification('保存する明細データがありません。', 'error');
			return;
		}
		const payload = {
			isNewClient, clientCode, clientName: clientNameToSave,
			transactionDate: headerDateInput.value.replace(/-/g, ''),
			transactionType: headerTypeSelect.value,
			records: records,
			originalReceiptNumber: currentLoadedReceipt
		};
		window.showLoading();
		try {
			const res = await fetch('/api/inout/save', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload),
			});
			const resData = await res.json();
			if (!res.ok) {
				throw new Error(resData.message || `保存に失敗しました (HTTP ${res.status})`);
			}

			// ▼▼▼ [修正点] 新規得意先が作成された場合、変換マップを更新する ▼▼▼
			if (resData.newClient) {
				await refreshClientMap();
			}
			// ▲▲▲ 修正ここまで ▲▲▲

			window.showNotification(`データを保存しました。\n伝票番号: ${resData.receiptNumber}`, 'success');
			resetHeader();
			clearDetailsTable();
		} catch (err) {
			console.error(err);
			window.showNotification(err.message, 'error');
		} finally {
			window.hideLoading();
		}
	});

	deleteBtn.addEventListener('click', async () => {
		const receiptNumber = receiptSelect.value;
		if (!receiptNumber || receiptNumber === NEW_ENTRY_VALUE) {
			window.showNotification("削除対象の伝票が選択されていません。", 'error');
			return;
		}
		if (!confirm(`伝票番号 [${receiptNumber}] を完全に削除します。よろしいですか？`)) {
			return;
		}
		window.showLoading();
		try {
			const res = await fetch(`/api/transaction/delete/${receiptNumber}`, { method: 'DELETE' });
			const errData = await res.json().catch(() => null);
			if (!res.ok) {
				throw new Error(errData?.message || '削除に失敗しました。');
			}
			window.showNotification(`伝票 [${receiptNumber}] を削除しました。`, 'success');
			resetHeader();
			clearDetailsTable();
		} catch(err) {
			console.error(err);
			window.showNotification(err.message, 'error');
		} finally {
			window.hideLoading();
		}
	});
}