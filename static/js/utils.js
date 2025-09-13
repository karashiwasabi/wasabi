// C:\Users\wasab\OneDrive\デスクトップ\WASABI\static\js\utils.js

/**
 * 文字列内のひらがなをカタカナに変換します。
 * @param {string} str 変換する文字列
 * @returns {string} カタカナに変換された文字列
 */
export function hiraganaToKatakana(str) {
    if (!str) return ''; 
    return str.replace(/[\u3041-\u3096]/g, function(match) {
        const charCode = match.charCodeAt(0) + 0x60;
        return String.fromCharCode(charCode);
    }); 
}

// ▼▼▼ [ここから追加] ▼▼▼
/**
 * 現在のPCのローカル日付を 'YYYY-MM-DD' 形式の文字列で返します。
 * @returns {string} 'YYYY-MM-DD' 形式の文字列
 */
export function getLocalDateString() {
    const today = new Date();
    const yyyy = today.getFullYear();
    const mm = String(today.getMonth() + 1).padStart(2, '0');
    const dd = String(today.getDate()).padStart(2, '0');
    return `${yyyy}-${mm}-${dd}`;
}
// ▲▲▲ [追加ここまで] ▲▲▲