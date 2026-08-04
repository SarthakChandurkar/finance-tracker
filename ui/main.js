// main.js — Controller, Event Listeners, and Application Flow

import { 
  views, navButtons, navMenu, hamburgerToggle, txType, txSource, txDestination, 
  txCategory, txPaymentMode, txPaymentInstrument, txDate, transactionForm, 
  txSubmit, txCancel, walletMode, walletForm, walletSubmit, walletCancel, 
  categoryForm, recategorizeForm, walletModeNote, fundingPanel, fundingMessage, 
  fundingOptionsList, fundingCancel, loanSettlePanel, settleCounterparty, 
  settleAmount, settleSource, settleSend, settleCancel, interWalletSettlePanel, 
  interWalletSettleLabel, interWalletSettleAmount, interWalletSettleSend, 
  interWalletSettleCancel, systemStatus, monthEndButton, taskForm, taskTitleInput, 
  taskDueInput, deleteAllTasksBtn, confirmModal, confirmTitle, confirmMessage, 
  confirmDetails, confirmYes, confirmNo 
} from './dom.js';

import { showToast, setFormError, fetchJSON, normalizeFormNumbers, formatDate } from './utils.js';
import { buildSelectOptions, populateDestinationOptions, renderTasks } from './render.js';
import { 
  currentWallets, currentTransactions, currentTasks, pendingTransaction, 
  editingWalletId, editingTransactionId, editingTaskId, pendingInterWalletSettlement,
  setPendingTransaction, setEditingWalletId, setEditingTransactionId, 
  setEditingTaskId, setPendingInterWalletSettlement, findWallet,
  refreshData, refreshTransactionsTab, refreshTasks, refreshWalletsTab, refreshCategoriesTab 
} from './state.js';

// --- Constants ---
const transactionTypes = ['Debit', 'Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Wallet Loan'];
const walletModeOptions = ['Both', 'Monthly', 'Global'];
const paymentModes = ['Self', 'On Behalf of Other'];
const paymentInstruments = ['UPI', 'Cash', 'Card', 'Bank Transfer', 'Cheque'];
let localConfirmCallback = null;

// --- View & Navigation Logic ---
function setActiveView(viewId) {
  views.forEach((view) => view.classList.toggle('active', view.id === viewId));
  navButtons.forEach((button) => button.classList.toggle('active', button.dataset.view === viewId));
  if (navMenu) navMenu.classList.remove('open');
}

const viewRefreshers = {
  dashboard: refreshData,
  transactions: refreshTransactionsTab,
  wallets: refreshWalletsTab,
  categories: refreshCategoriesTab,
  tasks: refreshTasks,
  system: () => {},
};

navButtons.forEach((button) => button.addEventListener('click', () => {
  const view = button.dataset.view;
  setActiveView(view);
  const refresh = viewRefreshers[view];
  if (refresh) refresh();
}));

hamburgerToggle && hamburgerToggle.addEventListener('click', () => {
  if (navMenu) navMenu.classList.toggle('open');
});

// --- Form UI Logic ---
function updateTransactionFormFields() {
  if (!txType) return;
  const type = txType.value;
  
  const categoryLabel = document.querySelector('label[for="tx-category"]');
  const destinationLabel = document.querySelector('label[for="tx-destination"]');
  const counterpartyLabel = document.querySelector('label[for="tx-counterparty"]');

  if (categoryLabel?.parentElement) {
    categoryLabel.parentElement.style.display = type === 'Debit' ? '' : 'none';
  }
  
  if (destinationLabel?.parentElement) {
    const destinationRequiredTypes = ['Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Wallet Loan'];
    destinationLabel.parentElement.style.display = destinationRequiredTypes.includes(type) ? '' : 'none';
  }
  
  if (counterpartyLabel?.parentElement) {
    counterpartyLabel.parentElement.style.display = type === 'Salary' ? 'none' : '';
  }

  populateDestinationOptions(currentWallets);
}

function updateWalletFormFields() {
  if (!walletMode) return;
  const mode = walletMode.value;
  const targetField = document.getElementById('wallet-target-amount').parentElement;
  const eomField = document.getElementById('wallet-eom-destination').parentElement;
  
  if (mode === 'Monthly') {
    eomField.style.display = '';
    targetField.style.display = 'none';
  } else if (mode === 'Global') {
    eomField.style.display = 'none';
    targetField.style.display = '';
  } else {
    eomField.style.display = '';
    targetField.style.display = '';
  }
}

function resetTransactionForm() {
  if (!transactionForm) return;
  transactionForm.reset();
  if (txDate) txDate.value = formatDate(new Date());
  setEditingTransactionId(null);
  transactionForm.querySelector('#transaction-form-heading').textContent = 'Create Transaction';
  txSubmit.textContent = 'Save Transaction';
  setFormError('tx-error', '');
  updateTransactionFormFields();
}

function resetWalletForm() {
  if (!walletForm) return;
  walletForm.reset();
  setEditingWalletId(null);
  walletMode.disabled = false;
  walletModeNote.textContent = '';
  walletForm.querySelector('#wallet-form-heading').textContent = 'Create Wallet';
  walletSubmit.textContent = 'Create Wallet';
  setFormError('wallet-error', '');
  updateWalletFormFields();
}

// --- Modals & Panels ---
function openConfirm(title, message, details, callback) {
  if (!confirmModal) return callback && callback();
  confirmTitle.textContent = title || 'Confirm';
  confirmMessage.textContent = message || '';
  confirmDetails.textContent = details || '';
  localConfirmCallback = callback;
  confirmModal.classList.remove('hidden');
}

function closeConfirm() {
  if (!confirmModal) return;
  localConfirmCallback = null;
  confirmModal.classList.add('hidden');
}

confirmYes && confirmYes.addEventListener('click', () => {
  if (typeof localConfirmCallback === 'function') {
    try { localConfirmCallback(); } catch (e) { console.error(e); }
  }
  closeConfirm();
});
confirmNo && confirmNo.addEventListener('click', closeConfirm);

function showFundingOptions(errorBody) {
  if (!fundingPanel) return;
  const options = (errorBody && errorBody.options) || [];
  if (!options.length) {
    fundingMessage.textContent = errorBody.message || 'No funding options are available for this shortfall.';
    fundingOptionsList.innerHTML = '';
  } else {
    fundingMessage.textContent = errorBody.message || 'Choose a funding option to remediate the shortfall.';
    fundingOptionsList.innerHTML = options.map((opt) => `
      <li>
        <button class="fund-option" data-mechanism="${opt.mechanism}" data-source-wallet-id="${opt.source_wallet_id}">${opt.label || opt.mechanism}</button>
      </li>
    `).join('');
  }
  fundingPanel.classList.remove('hidden');
}

function hideFundingOptions() {
  if (!fundingPanel) return;
  fundingPanel.classList.add('hidden');
  fundingMessage.textContent = '';
  fundingOptionsList.innerHTML = '';
}

function openLoanSettlePanel(counterparty) {
  if (!loanSettlePanel) return;
  settleCounterparty.textContent = counterparty;
  settleSource.innerHTML = currentWallets.map((q) => `<option value="${q.id}">${q.name} (${q.scope})</option>`).join('');
  settleAmount.value = '';
  loanSettlePanel.classList.remove('hidden');
}

function openInterWalletSettlePanel(lenderId, lenderName, borrowerId, borrowerName) {
  if (!interWalletSettlePanel) return;
  setPendingInterWalletSettlement({ lenderId, borrowerId });
  interWalletSettleLabel.textContent = `${borrowerName} owes ${lenderName}`;
  interWalletSettleAmount.value = '';
  interWalletSettlePanel.classList.remove('hidden');
}

// --- Edit Helpers ---
function openEditWallet(id) {
  const wallet = findWallet(id);
  if (!wallet) return showToast('Wallet not found', 'error');
  
  setEditingWalletId(id);
  walletForm.querySelector('#wallet-name').value = wallet.name || '';
  walletForm.querySelector('#wallet-mode').value = wallet.scope || 'Both';
  walletForm.querySelector('#wallet-target-amount').value = wallet.target_amount ?? '';
  walletForm.querySelector('#wallet-eom-destination').value = wallet.eom_sweep_destination || '';
  walletForm.querySelector('#wallet-form-heading').textContent = 'Edit Wallet';
  walletSubmit.textContent = 'Save Wallet';
  walletMode.disabled = true;
  walletModeNote.textContent = 'Wallet scope cannot be changed while editing. Create a new wallet to change scope.';
  
  updateWalletFormFields();
}

function openEditTransaction(id) {
  const txn = currentTransactions.find((t) => String(t.id) === String(id));
  if (!txn) return showToast('Transaction not found', 'error');
  
  setEditingTransactionId(id);
  transactionForm.querySelector('#tx-type').value = txn.type || 'Debit';
  transactionForm.querySelector('#tx-amount').value = txn.amount ?? '';
  transactionForm.querySelector('#tx-source').value = txn.source_wallet_id || '';
  transactionForm.querySelector('#tx-destination').value = txn.destination_wallet_id || '';
  transactionForm.querySelector('#tx-category').value = txn.category || '';
  transactionForm.querySelector('#tx-counterparty').value = txn.counterparty || '';
  transactionForm.querySelector('#tx-payment-mode').value = txn.payment_mode || paymentModes[0];
  transactionForm.querySelector('#tx-payment-instrument').value = txn.payment_instrument || paymentInstruments[0];
  transactionForm.querySelector('#tx-date').value = txn.date ? txn.date.split('T')[0] : '';
  transactionForm.querySelector('#tx-details').value = txn.details || '';
  transactionForm.querySelector('#transaction-form-heading').textContent = 'Edit Transaction';
  txSubmit.textContent = 'Save Transaction';
  
  updateTransactionFormFields();
}

// --- Specific Form Submits & Actions ---
transactionForm && transactionForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  setFormError('tx-error', '');
  const fd = new FormData(transactionForm);
  const payload = Object.fromEntries(fd.entries());
  normalizeFormNumbers(payload, ['amount']);
  const method = editingTransactionId ? 'PUT' : 'POST';
  const url = editingTransactionId ? `/api/transactions/${editingTransactionId}` : '/api/transactions';
  
  fetchJSON(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      showToast('Transaction saved', 'success');
      resetTransactionForm();
      refreshTransactionsTab();
    })
    .catch((err) => {
      if (err.status === 422 && err.body && err.body.options) {
        setPendingTransaction(payload);
        showFundingOptions(err.body);
      } else {
        setFormError('tx-error', err.message || 'Failed to save transaction');
      }
    });
});

walletForm && walletForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  setFormError('wallet-error', '');
  const fd = new FormData(walletForm);
  const payload = Object.fromEntries(fd.entries());
  normalizeFormNumbers(payload, ['target_amount']);
  
  if (payload.mode === 'Global' && payload.monthly_allocation) {
    return setFormError('wallet-error', 'Monthly allocation is not applicable to Global wallets');
  }
  
  const method = editingWalletId ? 'PUT' : 'POST';
  const url = editingWalletId ? `/api/wallets/${editingWalletId}` : '/api/wallets';
  
  fetchJSON(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      showToast(editingWalletId ? 'Wallet updated' : 'Wallet created', 'success');
      resetWalletForm();
      refreshWalletsTab();
    })
    .catch((err) => setFormError('wallet-error', err.message || 'Failed to save wallet'));
});

categoryForm && categoryForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(categoryForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category created', 'success'); categoryForm.reset(); refreshCategoriesTab(); })
    .catch((err) => showToast(err.message || 'Failed to create category', 'error'));
});

recategorizeForm && recategorizeForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(recategorizeForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories/recategorize', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category recategorized', 'success'); recategorizeForm.reset(); refreshCategoriesTab(); })
    .catch((err) => showToast(err.message || 'Failed to recategorize category', 'error'));
});

taskForm && taskForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  setFormError('task-error', '');
  const title = taskTitleInput.value.trim();
  if (!title) return setFormError('task-error', 'Task title is required');
  
  const payload = { title, due_date: taskDueInput.value || formatDate(new Date()) };
  fetchJSON('/api/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      taskForm.reset();
      if (taskDueInput) taskDueInput.value = formatDate(new Date());
      showToast('Task added', 'success');
      refreshTasks();
    })
    .catch((err) => setFormError('task-error', err.message || 'Failed to add task'));
});

// --- Click Delegation (Tables & Lists) ---
document.addEventListener('click', (event) => {
  const target = event.target;
  
  if (target.classList.contains('delete-category')) {
    openConfirm('Delete category', 'Delete this category? Transactions will be reassigned to Settled.', '', () => {
      fetchJSON(`/api/categories/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshCategoriesTab(); showToast('Category deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete category', 'error'));
    });
  }
  
  if (target.classList.contains('edit-wallet')) openEditWallet(target.dataset.id);
  
  if (target.classList.contains('delete-wallet')) {
    openConfirm('Delete wallet', 'Delete this wallet permanently? Its remaining balance will be transferred to Savings. This cannot be undone.', '', () => {
      fetchJSON(`/api/wallets/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshWalletsTab(); showToast('Wallet deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete wallet', 'error'));
    });
  }
  
  if (target.classList.contains('edit-transaction')) openEditTransaction(target.dataset.id);
  
  if (target.classList.contains('delete-transaction')) {
    openConfirm('Delete transaction', 'Delete this transaction? This action cannot be undone.', '', () => {
      fetchJSON(`/api/transactions/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshTransactionsTab(); showToast('Transaction deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete transaction', 'error'));
    });
  }
  
  if (target.classList.contains('task-edit-btn')) {
    setEditingTaskId(target.dataset.id);
    renderTasks(currentTasks, editingTaskId);
  }
  
  if (target.classList.contains('task-cancel-btn')) {
    setEditingTaskId(null);
    renderTasks(currentTasks, editingTaskId);
  }
  
  if (target.classList.contains('task-save-btn')) {
    const id = target.dataset.id;
    const li = target.closest('.task-item');
    const titleInput = li && li.querySelector('.task-title-edit');
    const dueInput = li && li.querySelector('.task-due-edit');
    const title = titleInput ? titleInput.value.trim() : '';
    
    if (!title) return showToast('Task title cannot be empty', 'error');
    
    const payload = { title, due_date: (dueInput && dueInput.value) || '' };
    fetchJSON(`/api/tasks/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
      .then(() => {
        setEditingTaskId(null);
        showToast('Task updated', 'success');
        refreshTasks();
      })
      .catch((err) => showToast(err.message || 'Failed to update task', 'error'));
  }
  
  if (target.classList.contains('settle-loan')) openLoanSettlePanel(target.dataset.counterparty);
  
  if (target.classList.contains('settle-interwallet-loan')) {
    openInterWalletSettlePanel(target.dataset.lenderId, target.dataset.lenderName, target.dataset.borrowerId, target.dataset.borrowerName);
  }
});

// Task Checkbox Delegation
document.addEventListener('change', (event) => {
  const checkbox = event.target.closest('.task-delete-checkbox');
  if (!checkbox) return;
  const id = checkbox.dataset.id;
  const li = checkbox.closest('.task-item');
  if (!li) return;

  checkbox.disabled = true;
  li.classList.add('task-strike');

  setTimeout(() => {
    fetchJSON(`/api/tasks/${id}`, { method: 'DELETE' })
      .then(() => { showToast('Task deleted', 'success'); refreshTasks(); })
      .catch((err) => {
        showToast(err.message || 'Failed to delete task', 'error');
        checkbox.disabled = false;
        checkbox.checked = false;
        li.classList.remove('task-strike');
      });
  }, 2000);
});

// --- General Buttons ---
fundingOptionsList && fundingOptionsList.addEventListener('click', (event) => {
  const button = event.target.closest('.fund-option');
  if (!button) return;
  if (!pendingTransaction) return showToast('No pending transaction to remediate.', 'error');
  
  const payload = {
    mechanism: button.dataset.mechanism,
    source_wallet_id: button.dataset.sourceWalletId,
    target_wallet_id: pendingTransaction.source_wallet_id,
    amount: pendingTransaction.amount,
    category: pendingTransaction.category,
    counterparty: pendingTransaction.counterparty,
    payment_mode: pendingTransaction.payment_mode,
    payment_instrument: pendingTransaction.payment_instrument,
    date: pendingTransaction.date,
    details: pendingTransaction.details,
  };
  
  fetchJSON('/api/funding-remediation', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      setPendingTransaction(null);
      hideFundingOptions();
      refreshTransactionsTab();
      showToast('Funding remediation applied', 'success');
    })
    .catch((err) => showToast(err.message || 'Remediation failed', 'error'));
});

fundingCancel && fundingCancel.addEventListener('click', () => {
  setPendingTransaction(null);
  hideFundingOptions();
});

settleCancel && settleCancel.addEventListener('click', () => {
  if (loanSettlePanel) loanSettlePanel.classList.add('hidden');
});

settleSend && settleSend.addEventListener('click', () => {
  const amount = parseFloat(settleAmount.value);
  const source = settleSource.value;
  const counterparty = settleCounterparty.textContent;
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!source) return showToast('Select a source wallet', 'error');
  
  const payload = {
    type: 'Debit', amount, source_wallet_id: source, category: 'Settle', 
    counterparty, date: new Date().toISOString().split('T')[0], details: 'Loan settlement'
  };
  
  fetchJSON('/api/transactions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      if (loanSettlePanel) loanSettlePanel.classList.add('hidden');
      refreshData();
      showToast('Loan settlement recorded', 'success');
    })
    .catch((err) => showToast(err.message || 'Failed to settle loan', 'error'));
});

monthEndButton && monthEndButton.addEventListener('click', () => {
  if (systemStatus) {
    systemStatus.textContent = 'Running month-end sweep...';
    systemStatus.style.color = '#111827';
  }
  fetchJSON('/api/schedule/end', { method: 'POST' })
    .then(() => { 
      if (systemStatus) systemStatus.textContent = 'Month-end sweep completed.'; 
      refreshTransactionsTab(); 
    })
    .catch((err) => {
      if (systemStatus) {
        systemStatus.textContent = `Month-end failed: ${err.message}`;
        systemStatus.style.color = '#dc2626';
      }
    });
});

interWalletSettleCancel && interWalletSettleCancel.addEventListener('click', () => {
  setPendingInterWalletSettlement(null);
  if (interWalletSettlePanel) interWalletSettlePanel.classList.add('hidden');
});

interWalletSettleSend && interWalletSettleSend.addEventListener('click', () => {
  const amount = parseFloat(interWalletSettleAmount.value);
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!pendingInterWalletSettlement) return showToast('No loan selected to settle', 'error');
  
  const payload = { borrower_wallet_id: pendingInterWalletSettlement.borrowerId, lender_wallet_id: pendingInterWalletSettlement.lenderId, amount };
  
  fetchJSON('/api/inter-wallet-loans/settle', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      setPendingInterWalletSettlement(null);
      if (interWalletSettlePanel) interWalletSettlePanel.classList.add('hidden');
      refreshData();
      showToast('Inter-Wallet Loan settlement recorded', 'success');
    })
    .catch((err) => showToast(err.message || 'Failed to settle loan', 'error'));
});

deleteAllTasksBtn && deleteAllTasksBtn.addEventListener('click', () => {
  if (!currentTasks.length) return showToast('No tasks to delete', 'error');
  openConfirm('Delete all tasks', `Delete all ${currentTasks.length} task${currentTasks.length === 1 ? '' : 's'}? This cannot be undone.`, '', () => {
    fetchJSON(`/api/tasks`, { method: 'DELETE' })
      .then(() => { showToast('All tasks deleted', 'success'); refreshTasks(); })
      .catch((err) => { showToast(err.message || 'Failed to delete all tasks', 'error'); refreshTasks(); });
  });
});

// --- Standard Input Event Listeners ---
txType && txType.addEventListener('change', updateTransactionFormFields);
walletMode && walletMode.addEventListener('change', updateWalletFormFields);
txCancel && txCancel.addEventListener('click', resetTransactionForm);
walletCancel && walletCancel.addEventListener('click', resetWalletForm);

// --- Initialization ---
function init() {
  if (txDate && !txDate.value) txDate.value = formatDate(new Date());
  if (taskDueInput && !taskDueInput.value) taskDueInput.value = formatDate(new Date());
  
  buildSelectOptions(txType, transactionTypes, false);
  buildSelectOptions(walletMode, walletModeOptions, false);
  buildSelectOptions(txPaymentMode, paymentModes, false);
  buildSelectOptions(txPaymentInstrument, paymentInstruments, false);
  
  updateTransactionFormFields();
  updateWalletFormFields();
  refreshData();
  refreshTasks();
}

init();