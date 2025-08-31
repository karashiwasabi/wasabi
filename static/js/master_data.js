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

export async function loadMasterData() {
	try {
        // ▼▼▼【ここから修正】▼▼▼
		const results = await Promise.all([
			fetchAndPopulateClients(),
			fetch('/api/settings/wholesalers')
		]);
        const wholesalerRes = results[1]; // Promise.allの結果配列から2番目の要素を取得
        // ▲▲▲【修正ここまで】▲▲▲

		if (!wholesalerRes.ok) {
			throw new Error('卸業者マスターの読み込みに失敗しました。');
		}

		const wholesalers = await wholesalerRes.json();
		wholesalerMap.clear();
		if (wholesalers) {
			wholesalers.forEach(w => wholesalerMap.set(w.code, w.name));
		}
		
		console.log('得意先と卸業者のマスターデータを読み込みました。');

	} catch (error) {
		console.error("マスターデータの読み込み中にエラーが発生しました:", error);
		window.showNotification('マスターデータの読み込みに失敗しました。', 'error');
	}
}