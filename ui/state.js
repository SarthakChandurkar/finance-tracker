// state.js — Data layer, API fetching, and State management

import { fetchJSON, showToast } from './utils.js';
import {
  renderWallets, renderCategories, renderTransactions,
  renderLoanLedger, renderInterWalletLoanLedger, renderLoanLedger_settle, 
  renderInterWalletLoanLedger_settle, renderDashboard,
  renderTasks, renderTodayTasks, showTasksConnecting, buildSelectOptions,
  populateDestinationOptions, renderSortIndicators,
  renderAnalysisTransactions, renderAnalysisSortIndicators, renderAnalysisCharts
} from './render.js';
import {
  txSource, txCategory, sourceCategorySelect, destCategorySelect, settleSource
} from './dom.js';

// --- State Variables ---
export let currentWallets = [];
export let currentCategories = [];
export let currentTransactions = [];
export let currentTasks = [];
export let currentCategoryTotals = { monthly: [], global: [] };
export let pendingTransaction = null;
export let editingWalletId = null;
export let editingTransactionId = null;
export let editingTaskId = null;
export let confirmCallback = null;
export let tasksLoaded = false;
export let pendingInterWalletSettlement = null;

// --- Sort & Search Configs ---
export let txSortConfig = { key: 'date', dir: 'desc' };
export let walletSortConfig = { key: 'scope', dir: 'asc' };
export let analysisTxSortConfig = { key: 'date', dir: 'desc' };
export let analysisSearchQuery = '';

// --- State Setters for main.js ---
export const setPendingTransaction = (val) => pendingTransaction = val;
export const setEditingWalletId = (val) => editingWalletId = val;
export const setEditingTransactionId = (val) => editingTransactionId = val;
export const setEditingTaskId = (val) => editingTaskId = val;
export const setConfirmCallback = (val) => confirmCallback = val;
export const setPendingInterWalletSettlement = (val) => pendingInterWalletSettlement = val;
export const setAnalysisSearchQuery = (val) => analysisSearchQuery = val;

// --- Sorting Logic ---
export function applySorts() {
  const compare = (a, b, config) => {
    let vA = a[config.key];
    let vB = b[config.key];

    if (['source_wallet_id', 'destination_wallet_id', 'eom_sweep_destination'].includes(config.key)) {
        vA = currentWallets.find(w => String(w.id) === String(vA))?.name || '';
        vB = currentWallets.find(w => String(w.id) === String(vB))?.name || '';
    }

    if (config.key === 'amount' || config.key === 'target_amount') {
        vA = Number(vA) || 0;
        vB = Number(vB) || 0;
    } else {
        vA = String(vA || '').toLowerCase();
        vB = String(vB || '').toLowerCase();
    }

    if (vA < vB) return config.dir === 'asc' ? -1 : 1;
    if (vA > vB) return config.dir === 'asc' ? 1 : -1;
    return 0;
  };
  
  currentTransactions.sort((a, b) => compare(a, b, txSortConfig));
  currentWallets.sort((a, b) => compare(a, b, walletSortConfig));
}

export function setTxSort(key) {
  if (txSortConfig.key === key) txSortConfig.dir = txSortConfig.dir === 'asc' ? 'desc' : 'asc';
  else { txSortConfig.key = key; txSortConfig.dir = 'asc'; }
  applySorts();
  renderTransactions(currentTransactions, currentWallets);
  renderSortIndicators(txSortConfig, walletSortConfig);
}

export function setWalletSort(key) {
  if (walletSortConfig.key === key) walletSortConfig.dir = walletSortConfig.dir === 'asc' ? 'desc' : 'asc';
  else { walletSortConfig.key = key; walletSortConfig.dir = 'asc'; }
  applySorts();
  renderWallets(currentWallets);
  renderSortIndicators(txSortConfig, walletSortConfig);
}

// --- Analysis Specific Filtering & Sorting ---
export function getFilteredAndSortedAnalysisTransactions() {
  let txs = [...currentTransactions];
  
  if (analysisSearchQuery) {
    const q = analysisSearchQuery.toLowerCase();
    txs = txs.filter(t => {
      const source = currentWallets.find(w => String(w.id) === String(t.source_wallet_id))?.name || '';
      const dest = currentWallets.find(w => String(w.id) === String(t.destination_wallet_id))?.name || '';
      
      return (
        (t.date && String(t.date).toLowerCase().includes(q)) ||
        (t.type && String(t.type).toLowerCase().includes(q)) ||
        (t.amount && String(t.amount).toLowerCase().includes(q)) ||
        source.toLowerCase().includes(q) ||
        dest.toLowerCase().includes(q) ||
        (t.category && String(t.category).toLowerCase().includes(q)) ||
        (t.counterparty && String(t.counterparty).toLowerCase().includes(q)) ||
        (t.payment_instrument && String(t.payment_instrument).toLowerCase().includes(q)) ||
        (t.details && String(t.details).toLowerCase().includes(q))
      );
    });
  }

  const compare = (a, b, config) => {
    let vA = a[config.key];
    let vB = b[config.key];

    if (['source_wallet_id', 'destination_wallet_id'].includes(config.key)) {
        vA = currentWallets.find(w => String(w.id) === String(vA))?.name || '';
        vB = currentWallets.find(w => String(w.id) === String(vB))?.name || '';
    }

    if (config.key === 'amount') {
        vA = Number(vA) || 0;
        vB = Number(vB) || 0;
    } else {
        vA = String(vA || '').toLowerCase();
        vB = String(vB || '').toLowerCase();
    }

    if (vA < vB) return config.dir === 'asc' ? -1 : 1;
    if (vA > vB) return config.dir === 'asc' ? 1 : -1;
    return 0;
  };

  txs.sort((a, b) => compare(a, b, analysisTxSortConfig));
  return txs;
}

export function setAnalysisTxSort(key) {
  if (analysisTxSortConfig.key === key) analysisTxSortConfig.dir = analysisTxSortConfig.dir === 'asc' ? 'desc' : 'asc';
  else { analysisTxSortConfig.key = key; analysisTxSortConfig.dir = 'asc'; }
  refreshAnalysisTab();
}

// --- Helper Functions ---
export function findWallet(id) {
  return currentWallets.find((q) => String(q.id) === String(id));
}

export function findCategory(id) {
  return currentCategories.find((c) => String(c.id) === String(id));
}

export function refreshWalletsAndCategoriesDropdowns() {
  buildSelectOptions(txSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  populateDestinationOptions(currentWallets);
  buildSelectOptions(txCategory, currentCategories.map((c) => ({ value: c.name, label: c.name })));
  buildSelectOptions(sourceCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(destCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(settleSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  const eomSelect = document.getElementById('wallet-eom-destination');
  if (eomSelect) {
    buildSelectOptions(eomSelect, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  }
}

export function refreshWalletsDropdowns() {
  buildSelectOptions(txSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  populateDestinationOptions(currentWallets);
  buildSelectOptions(settleSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  const eomSelect = document.getElementById('wallet-eom-destination');
  if (eomSelect) {
    buildSelectOptions(eomSelect, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  }
}

export function refreshCategoriesDropdowns() { 
  buildSelectOptions(txCategory, currentCategories.map((c) => ({ value: c.name, label: c.name })));
  buildSelectOptions(sourceCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(destCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
}

// --- API Functions ---
export async function loadWalletsAndCategories() {
  const [wallets, categories] = await Promise.all([
    fetchJSON('/api/wallets'),
    fetchJSON('/api/categories'),
  ]);
  currentWallets = wallets || [];
  currentCategories = categories || [];
  refreshWalletsAndCategoriesDropdowns();
}

export async function loadWallets() {
  const wallets = await fetchJSON('/api/wallets');
  currentWallets = wallets || [];
  refreshWalletsDropdowns();
}

export async function loadCategories() {
  const categories = await fetchJSON('/api/categories');
  currentCategories = categories || [];
  refreshCategoriesDropdowns();
}

export async function refreshTasks() {
  // if (!tasksLoaded) showTasksConnecting();
  // try {
  //   const rawTasks = await fetchJSON('/api/tasks');
    
  //   currentTasks = (rawTasks || []).map(t => {
  //     t.id = t._id || t.id;
  //     return t;
  //   });
    
  //   tasksLoaded = true;
  //   renderTasks(currentTasks, editingTaskId);
  //   renderTodayTasks(currentTasks);
  // } catch (err) {
  //   showToast(err.message || 'Failed to load tasks', 'error');
  //   if (!tasksLoaded) {
  //     const taskList = document.getElementById('task-list');
  //     const todayTasksEl = document.getElementById('today-tasks');
  //     if (taskList) taskList.innerHTML = '<li class="task-empty">Couldn\u2019t reach the task server. Try again shortly.</li>';
  //     if (todayTasksEl) todayTasksEl.innerHTML = '<p class="note">Couldn\u2019t reach the task server.</p>';
  //   }
  // }
}

export async function refreshWalletsTab() {
  try {
    await loadWallets();
    applySorts();
    renderWallets(currentWallets);
    renderSortIndicators(txSortConfig, walletSortConfig);
  } catch (err) {
    showToast(err.message || 'Failed to refresh wallets', 'error');
  }
}

export async function refreshCategoriesTab() {
  try {
    await loadCategories();
    renderCategories(currentCategories);
  } catch (err) {
    showToast(err.message || 'Failed to refresh categories', 'error');
  }
}

export async function refreshTransactionsTab() {
  try {
    await loadWalletsAndCategories();
    const transactions = await fetchJSON('/api/transactions');
    currentTransactions = transactions || [];
    applySorts();
    renderTransactions(currentTransactions, currentWallets);
    renderSortIndicators(txSortConfig, walletSortConfig);
  } catch (err) {
    showToast(err.message || 'Failed to refresh transactions', 'error');
  }
}

export async function refreshAnalysisTab() {
  try {
    // Unconditionally fetch fresh data from the server
    await refreshData();
    
    const filteredAndSorted = getFilteredAndSortedAnalysisTransactions();
    renderAnalysisTransactions(filteredAndSorted, currentWallets);
    renderAnalysisSortIndicators(analysisTxSortConfig);
    renderAnalysisCharts(currentCategoryTotals);
  } catch (err) {
    showToast(err.message || 'Failed to refresh analysis', 'error');
  }
}

export async function refreshData() {
  try {
    const [wallets, categories, transactions, walletTotals, categoryTotalsData, loanEntries, interWalletLoanEntries] = await Promise.all([
      fetchJSON('/api/wallets'),
      fetchJSON('/api/categories'),
      fetchJSON('/api/transactions'),
      fetchJSON('/api/wallets/breakdowns'),
      fetchJSON('/api/categories/totals'),
      fetchJSON('/api/loan-ledger'),
      fetchJSON('/api/inter-wallet-loans')
    ]);
    currentWallets = wallets || [];
    currentCategories = categories || [];
    currentTransactions = transactions || [];
    currentCategoryTotals = categoryTotalsData || { monthly: [], global: [] };
    
    applySorts();
    
    renderWallets(currentWallets);
    renderCategories(currentCategories);
    renderTransactions(currentTransactions, currentWallets);
    renderSortIndicators(txSortConfig, walletSortConfig);
    
    renderLoanLedger(loanEntries || []);
    renderInterWalletLoanLedger(interWalletLoanEntries || [], currentWallets);
    renderLoanLedger_settle(loanEntries || []);
    renderInterWalletLoanLedger_settle(interWalletLoanEntries || [], currentWallets);
    
    renderDashboard(walletTotals, categoryTotalsData, currentWallets);
    refreshWalletsAndCategoriesDropdowns();
  } catch (err) {
    showToast(err.message || 'Failed to refresh data', 'error');
  }
}