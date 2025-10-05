// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\master_data.js

export const clientMap = new Map();
export const wholesalerMap = new Map();

async function fetchAndPopulateClients() {
	const res = await fetch('/api/clients');
	if (!res.ok) {
		throw new Error('得意先マスターの取得に失敗しました。');
	}
	const clients = await res.json();
	clientMap.clear();
	if (clients) {
		clients.forEach(c => clientMap.set(c.code, c.name));
	}
}

export async function refreshClientMap() {
	try {
		await fetchAndPopulateClients();
		console.log('得意先マップを更新しました。');
	} catch (error) {
		console.error("得意先マップの更新に失敗しました:", error);
		window.showNotification('得意先リストの更新に失敗しました。', 'error');
	}
}

// ▼▼▼【ここから修正】▼▼▼
async function fetchAndPopulateWholesalers() {
	const res = await fetch('/api/settings/wholesalers');
	if (!res.ok) {
		throw new Error('卸業者マスターの読み込みに失敗しました。');
	}
	const wholesalers = await res.json();
	wholesalerMap.clear();
	if (wholesalers) {
		wholesalers.forEach(w => wholesalerMap.set(w.code, w.name));
	}
}

export async function refreshWholesalerMap() {
    try {
        await fetchAndPopulateWholesalers();
        console.log('卸業者マップを更新しました。');
    } catch (error) {
        console.error("卸業者マップの更新に失敗しました:", error);
        window.showNotification('卸業者リストの更新に失敗しました。', 'error');
    }
}

export async function loadMasterData() {
	try {
		await Promise.all([
			fetchAndPopulateClients(),
			fetchAndPopulateWholesalers()
		]);
		console.log('得意先と卸業者のマスターデータを読み込みました。');
	} catch (error) {
		console.error("マスターデータの読み込み中にエラーが発生しました:", error);
		window.showNotification('マスターデータの読み込みに失敗しました。', 'error');
	}
}
// ▲▲▲【修正ここまで】▲▲▲