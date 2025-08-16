// C:\Dev\WASABI\static\js\master_data.js

export const clientMap = new Map();
export const wholesalerMap = new Map();

/**
 * 得意先マスターをサーバーから取得し、clientMapを更新する内部関数
 */
async function fetchAndPopulateClients() {
	const res = await fetch('/api/clients');
	if (!res.ok) {
		throw new Error('得意先マスターの取得に失敗しました。');
	}
	const clients = await res.json();
	clientMap.clear(); // 古いデータをクリア
	if (clients) { // APIからの応答がnullでないことを確認
		clients.forEach(c => clientMap.set(c.code, c.name));
	}
}

/**
 * 新しい得意先が追加された後に呼び出すための関数を新設
 * clientMapだけを再読み込みして更新します。
 */
export async function refreshClientMap() {
	try {
		await fetchAndPopulateClients();
		console.log('得意先マップを更新しました。');
	} catch (error) {
		console.error("得意先マップの更新に失敗しました:", error);
		window.showNotification('得意先リストの更新に失敗しました。', 'error');
	}
}

/**
 * アプリケーション起動時に一度だけ実行する関数
 */
export async function loadMasterData() {
	try {
		// ▼▼▼ [修正点] 先頭にカンマを追加して、1つ目の結果を無視するように修正します ▼▼▼
		const [, wholesalerRes] = await Promise.all([
		// ▲▲▲ 修正ここまで ▲▲▲
			fetchAndPopulateClients(), // 内部関数を呼び出す
			fetch('/api/settings/wholesalers')
		]);

		if (!wholesalerRes.ok) {
			throw new Error('卸業者マスターの読み込みに失敗しました。');
		}

		const wholesalers = await wholesalerRes.json();
		wholesalerMap.clear(); // 古いデータをクリア
		if (wholesalers) { // APIからの応答がnullでないことを確認
			wholesalers.forEach(w => wholesalerMap.set(w.code, w.name));
		}
		
		console.log('得意先と卸業者のマスターデータを読み込みました。');

	} catch (error) {
		console.error("マスターデータの読み込み中にエラーが発生しました:", error);
		window.showNotification('マスターデータの読み込みに失敗しました。', 'error');
	}
}