import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

let currentUploadType = '';
const fileInputs = {
    dat: document.getElementById('datFileInput'),
    usage: document.getElementById('usageFileInput'),
};

async function handleFileUpload(type, files) {
    if (!files.length) return;
    const uploadContainer = document.getElementById('upload-output-container');
    uploadContainer.innerHTML = createUploadTableHTML('upload-output-table');
    const tbody = uploadContainer.querySelector('tbody');
    tbody.innerHTML = `<tr><td colspan="14" class="center">Processing...</td></tr>`;
    window.showLoading();
    try {
        const formData = new FormData();
        for (const file of files) formData.append('file', file);
        const res = await fetch(`/api/${type}/upload`, { method: 'POST', body: formData });
        const data = await res.json();
        if (!res.ok) throw new Error(data.message || `${type.toUpperCase()} file processing failed.`);
        renderUploadTableRows('upload-output-table', data.records || data.details);
        window.showNotification(`${type.toUpperCase()} file(s) processed.`, 'success');
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="14" class="center" style="color:red;">Error: ${err.message}</td></tr>`;
        window.showNotification(err.message, 'error');
    } finally {
        window.hideLoading();
        if (fileInputs[type]) fileInputs[type].value = '';
    }
}

export function initUploadView() {
    fileInputs.dat.addEventListener('change', (e) => handleFileUpload('dat', e.target.files));
    fileInputs.usage.addEventListener('change', (e) => handleFileUpload('usage', e.target.files));
    document.addEventListener('showUploadView', (e) => {
        currentUploadType = e.detail.type;
        const title = document.getElementById('upload-view-title');
        const container = document.getElementById('upload-output-container');
        if (title) title.textContent = `${currentUploadType.toUpperCase()} File Upload`;
        if (container) container.innerHTML = `<p>Click the ${currentUploadType.toUpperCase()} button again to select files.</p>`;
        if (fileInputs[currentUploadType]) fileInputs[currentUploadType].click();
    });
}
export function resetUploadView() {
    const title = document.getElementById('upload-view-title');
    const container = document.getElementById('upload-output-container');
    if (title) title.textContent = 'File Upload';
    if (container) container.innerHTML = '';
}