// C:/Users/wasab/OneDrive/デスクトップ/WASABI/static/js/master_edit.js

import { initLogic, resetMasterEditView } from './master_edit_logic.js';
import { setUnitMap } from './master_edit_ui.js';

// ▼▼▼【ここから修正】欠落していたCSS定義を追加▼▼▼
const style = document.createElement('style');
style.innerHTML = `
    .master-edit-table .field-group { display: flex; flex-direction: column; gap: 2px; }
    .master-edit-table .field-group label { font-size: 10px; font-weight: bold; color: #555; }
    .master-edit-table .field-group input, .master-edit-table .field-group select, .master-edit-table .field-group textarea { width: 100%; font-size: 12px; padding: 4px; border: 1px solid #ccc; }
    .master-edit-table textarea { resize: vertical; min-height: 40px; }
    .master-edit-table .flags-container { display: flex; gap: 5px; }
    .master-edit-table .flags-container .field-group { flex: 1; }
    .master-edit-table input:read-only, .master-edit-table select:disabled, .master-edit-table input:disabled { background-color: #e9ecef; color: #6c757d; cursor: not-allowed; }
    
    .save-success {
        animation: flash-green 1.5s ease-out;
    }
    @keyframes flash-green {
        0% { background-color: #d1e7dd; }
        100% { background-color: transparent; }
    }
`;
document.head.appendChild(style);
// ▲▲▲【修正ここまで】▲▲▲

export async function initMasterEdit() {
    let unitMap = {};
    try {
        const res = await fetch('/api/units/map');
        if (!res.ok) throw new Error('単位マスタの取得に失敗');
        unitMap = await res.json();
        setUnitMap(unitMap); // UIモジュールに単位マップを設定
    } catch (err) {
        console.error(err);
        window.showNotification(err.message, 'error');
    }

    initLogic();
}

export { resetMasterEditView };