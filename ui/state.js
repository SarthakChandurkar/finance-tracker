// state.js — Data layer, API fetching, and State management

import { fetchJSON, showToast } from './utils.js';
import {
  renderWallets, renderCategories, renderTransactions,
  renderLoanLedger, renderInterWalletLoanLedger, renderLoanLedger_settle, renderInterWalletLoanLedger_settle, renderDashboard,
  renderTasks, renderTodayTasks, showTasksConnecting, buildSelectOptions,
  populateDestinationOptions
} from './render.js';
import {
  txSource, txCategory, sourceCategorySelect, destCategorySelect, settleSource
} from './dom.js';

// --- State Variables ---
export let currentWallets = [];
export let currentCategories = [];
export let currentTransactions = [];
export let currentTasks = [];
export let pendingTransaction = null;
export let editingWalletId = null;
export let editingTransactionId = null;
export let editingTaskId = null;
export let confirmCallback = null;
export let tasksLoaded = false;
export let pendingInterWalletSettlement = null;

// --- State Setters for main.js ---
export const setPendingTransaction = (val) => pendingTransaction = val;
export const setEditingWalletId = (val) => editingWalletId = val;
export const setEditingTransactionId = (val) => editingTransactionId = val;
export const setEditingTaskId = (val) => editingTaskId = val;
export const setConfirmCallback = (val) => confirmCallback = val;
export const setPendingInterWalletSettlement = (val) => pendingInterWalletSettlement = val;

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
  if (!tasksLoaded) showTasksConnecting();
  try {
    const rawTasks = await fetchJSON('/api/tasks');
    
    currentTasks = (rawTasks || []).map(t => {
      t.id = t._id || t.id;
      return t;
    });
    
    tasksLoaded = true;
    renderTasks(currentTasks, editingTaskId);
    renderTodayTasks(currentTasks);
  } catch (err) {
    showToast(err.message || 'Failed to load tasks', 'error');
    if (!tasksLoaded) {
      const taskList = document.getElementById('task-list');
      const todayTasksEl = document.getElementById('today-tasks');
      if (taskList) taskList.innerHTML = '<li class="task-empty">Couldn\u2019t reach the task server. Try again shortly.</li>';
      if (todayTasksEl) todayTasksEl.innerHTML = '<p class="note">Couldn\u2019t reach the task server.</p>';
    }
  }
}

export async function refreshWalletsTab() {
  try {
    await loadWallets();
    renderWallets(currentWallets);
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
    renderTransactions(currentTransactions, currentWallets);
  } catch (err) {
    showToast(err.message || 'Failed to refresh transactions', 'error');
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
    
    renderWallets(currentWallets);
    renderCategories(currentCategories);
    renderTransactions(currentTransactions, currentWallets);
    renderLoanLedger(loanEntries || []);
    renderInterWalletLoanLedger(interWalletLoanEntries || []);
    renderLoanLedger_settle(loanEntries || []);
    renderInterWalletLoanLedger_settle(interWalletLoanEntries || []);
    renderDashboard(walletTotals, categoryTotalsData);
    refreshWalletsAndCategoriesDropdowns();
  } catch (err) {
    showToast(err.message || 'Failed to refresh data', 'error');
  }
}