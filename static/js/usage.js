import { createUploadTableHTML, renderUploadTableRows } from './common_table.js';

export function initUsageUpload() {
    const usageInput = document.getElementById('usageFileInput');
    if (!usageInput) return;

    usageInput.addEventListener('change', async (e) => {
        const files = e.target.files;
        if (!files.length) return;

        const uploadContainer = document.getElementById('upload-output-container');
        uploadContainer.innerHTML = createUploadTableHTML('upload-output-table');
        const tbody = uploadContainer.querySelector('#upload-output-table tbody');
        tbody.innerHTML = `<tr><td colspan="14" style="text-align:center;">Processing...</td></tr>`;

        window.showLoading();
        try {
            const formData = new FormData();
            for (const file of files) formData.append('file', file);
            
            const res = await fetch('/api/usage/upload', { method: 'POST', body: formData });
            const data = await res.json();
            if (!res.ok) throw new Error(data.message || 'USAGE file processing failed.');
            
            renderUploadTableRows('upload-output-table', data.records);
            window.showNotification('USAGE file processed successfully.', 'success');

        } catch (err) {
            tbody.innerHTML = `<tr><td colspan="14" style="color:red; text-align:center;">Error: ${err.message}</td></tr>`;
            window.showNotification(err.message, 'error');
        } finally {
            window.hideLoading();
            e.target.value = ''; // Reset file input
        }
    });
}