import { initHeader, resetHeader } from './inout_header.js';
import { initDetailsTable, getDetailsData, clearDetailsTable, populateDetailsTable } from './inout_details_table.js';

export async function initInOut() {
  initDetailsTable();
  await initHeader(getDetailsData, clearDetailsTable, populateDetailsTable);
}

export function resetInOutView() {
    clearDetailsTable();
    resetHeader();
}