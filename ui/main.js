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
  confirmDetails, confirmYes, confirmNo, profileToggle, profileDropdown, 
  profileUsername, profileLogout
} from './dom.js';

import { showToast, setFormError, fetchJSON, normalizeFormNumbers, formatDate, escapeHTML } from './utils.js';
import { buildSelectOptions, populateDestinationOptions, renderTasks } from './render.js';
import { 
  currentWallets, currentTransactions, currentTasks, pendingTransaction, 
  editingWalletId, editingTransactionId, editingTaskId, pendingInterWalletSettlement,
  setPendingTransaction, setEditingWalletId, setEditingTransactionId, 
  setEditingTaskId, setPendingInterWalletSettlement, findWallet, setTxSort, setWalletSort,
  refreshData, refreshTransactionsTab, refreshTasks, refreshWalletsTab, refreshCategoriesTab 
} from './state.js';

// --- Constants & Helpers ---
const transactionTypes = ['Debit', 'Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Wallet Loan'];
const walletModeOptions = ['Both', 'Monthly', 'Global'];
const paymentModes = ['Self', 'On Behalf of Other'];
const paymentInstruments = ['UPI', 'Cash', 'Card', 'Bank Transfer', 'Cheque'];
let localConfirmCallback = null;

function sanitizeErrorMsg(msg) {
  if (!msg) return '';
  let sanitized = String(msg);
  (currentWallets || []).forEach((w) => {
    if (w.id && sanitized.includes(String(w.id))) {
      const regex = new RegExp(w.id, 'g');
      sanitized = sanitized.replace(regex, `'${w.name} (${w.scope})'`);
    }
  });
  return sanitized;
}

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

// --- Profile Menu ---
function closeProfileDropdown() {
  if (!profileDropdown) return;
  profileDropdown.classList.add('hidden');
  if (profileToggle) profileToggle.setAttribute('aria-expanded', 'false');
}

profileToggle && profileToggle.addEventListener('click', (event) => {
  event.stopPropagation();
  if (!profileDropdown) return;
  const isOpen = !profileDropdown.classList.contains('hidden');
  profileDropdown.classList.toggle('hidden', isOpen);
  profileToggle.setAttribute('aria-expanded', String(!isOpen));
});

document.addEventListener('click', (event) => {
  if (!profileDropdown || profileDropdown.classList.contains('hidden')) return;
  if (event.target.closest('.profile-menu')) return;
  closeProfileDropdown();
});

profileLogout && profileLogout.addEventListener('click', () => {
  closeProfileDropdown();
  fetchJSON('/api/logout', { method: 'POST' })
    .then(() => { window.location.href = '/login.html'; })
    .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to log out', 'error'));
});

function loadProfile() {
  if (!profileUsername) return;
  fetchJSON('/api/account')
    .then((user) => { profileUsername.textContent = (user && user.username) || 'Account'; })
    .catch(() => { /* not fatal - leave the default label in place */ });
}

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
  
  const options = errorBody.Options || errorBody.options || [];
  const requestedAmount = errorBody.Requested || (pendingTransaction && pendingTransaction.payload && pendingTransaction.payload.amount) || 0;
  
  const tableContainer = document.getElementById('funding-table-container');

  if (!options.length) {
    fundingMessage.textContent = 'Insufficient balance. No feasible mechanisms are available for this shortfall.';
    fundingMessage.style.color = '#dc2626'; 
    if (tableContainer) tableContainer.style.display = 'none';
  } else {
    fundingMessage.textContent = `Insufficient balance. Select a feasible mechanism below to satisfy the required ${Number(requestedAmount).toFixed(2)}.`;
    fundingMessage.style.color = '#475569';
    if (tableContainer) tableContainer.style.display = '';
    
    fundingOptionsList.innerHTML = options.map((opt) => {
      const sourceWallet = currentWallets.find(w => String(w.id) === String(opt.source_wallet_id));
      const rawName = opt.source_wallet_name || opt.source_wallet_id;
      let displayName = escapeHTML(rawName);
      
      if (sourceWallet) {
        const scopeClass = sourceWallet.scope === 'Monthly' ? 'monthly' : (sourceWallet.scope === 'Global' ? 'global' : 'accumulated');
        displayName = `${escapeHTML(rawName)} <span class="stat-badge ${scopeClass}">${escapeHTML(sourceWallet.scope)}</span>`;
      }
      
      let mechClass = 'accumulated'; 
      if (opt.mechanism === 'Debit') mechClass = 'spent';
      else if (opt.mechanism === 'Credit' || opt.mechanism === 'Salary') mechClass = 'available';
      else if (opt.mechanism === 'Inter-Wallet Loan') mechClass = 'iw-loan';
      else if (opt.mechanism === 'Loan Received') mechClass = 'loan-received';
      
      return `
      <tr>
        <td><span class="stat-badge ${mechClass}">${escapeHTML(opt.mechanism)}</span></td>
        <td><div style="display: flex; align-items: center; gap: 0.4rem;">${displayName}</div></td>
        <td class="font-mono">${Number(opt.available_balance || 0).toFixed(2)}</td>
        <td class="font-mono">${Number(requestedAmount).toFixed(2)}</td>
        <td class="actions-cell">
          <button class="btn-small fund-option" 
            data-mechanism="${escapeHTML(opt.mechanism)}" 
            data-source-wallet-id="${escapeHTML(opt.source_wallet_id)}"
            data-amount="${requestedAmount}">
            Commit & Pay
          </button>
        </td>
      </tr>
    `}).join('');
  }
  
  fundingPanel.classList.remove('hidden');
  
  setTimeout(() => {
    fundingPanel.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, 50); 
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
  
  const getBadgeHTML = (id, rawName) => {
    const wallet = currentWallets.find(w => String(w.id) === String(id));
    if (wallet) {
      const scopeClass = wallet.scope === 'Monthly' ? 'monthly' : (wallet.scope === 'Global' ? 'global' : 'accumulated');
      return `${escapeHTML(rawName)} <span class="stat-badge ${scopeClass}">${escapeHTML(wallet.scope)}</span>`;
    }
    return escapeHTML(rawName);
  };

  interWalletSettleLabel.innerHTML = `
    <div style="display: flex; align-items: center; gap: 0.4rem; flex-wrap: wrap;">
      ${getBadgeHTML(borrowerId, borrowerName)} 
      <span style="color: #64748b; font-weight: normal; margin: 0 0.2rem;">owes</span> 
      ${getBadgeHTML(lenderId, lenderName)}
    </div>
  `;
  
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
  hideFundingOptions();
  
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
      let parsedOptions = null;
      
      if (err.body && (err.body.Options || err.body.options)) {
        parsedOptions = err.body;
      } else if (err.message) {
        try { 
          const parsed = JSON.parse(err.message); 
          if (parsed.Options || parsed.options) {
             parsedOptions = parsed;
          }
        } catch(e) {}
      }

      if (parsedOptions) {
        setFormError('tx-error', ''); 
        setPendingTransaction({ payload, method, url });
        showFundingOptions(parsedOptions);
      } else {
        let msg = sanitizeErrorMsg(err.message) || 'Failed to save transaction';
        if (msg.includes('"Available":') || msg.toLowerCase().includes('insufficient balance')) {
           msg = 'Insufficient balance in the source wallet.';
        }
        setFormError('tx-error', msg);
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
    .catch((err) => setFormError('wallet-error', sanitizeErrorMsg(err.message) || 'Failed to save wallet'));
});

categoryForm && categoryForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(categoryForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category created', 'success'); categoryForm.reset(); refreshCategoriesTab(); })
    .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to create category', 'error'));
});

recategorizeForm && recategorizeForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(recategorizeForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories/recategorize', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category recategorized', 'success'); recategorizeForm.reset(); refreshCategoriesTab(); })
    .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to recategorize category', 'error'));
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
    .catch((err) => setFormError('task-error', sanitizeErrorMsg(err.message) || 'Failed to add task'));
});

// --- Click Delegation (Tables & Lists) ---
document.addEventListener('click', (event) => {
  const target = event.target;
  
  // Table Sorting Check
  const th = target.closest('th.sortable');
  if (th) {
    const tableId = th.closest('table').id;
    if (tableId === 'transaction-table') setTxSort(th.dataset.sort);
    else if (tableId === 'wallet-table') setWalletSort(th.dataset.sort);
    return;
  }
  
  if (target.classList.contains('delete-category')) {
    openConfirm('Delete category', 'Delete this category? Transactions will be reassigned to Settled.', '', () => {
      fetchJSON(`/api/categories/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshCategoriesTab(); showToast('Category deleted', 'success'); })
        .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to delete category', 'error'));
    });
  }
  
  if (target.classList.contains('edit-wallet')) openEditWallet(target.dataset.id);
  
  if (target.classList.contains('delete-wallet')) {
    openConfirm('Delete wallet', 'Delete this wallet permanently? Its remaining balance will be transferred to Savings. This cannot be undone.', '', () => {
      fetchJSON(`/api/wallets/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshWalletsTab(); showToast('Wallet deleted', 'success'); })
        .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to delete wallet', 'error'));
    });
  }
  
  if (target.classList.contains('edit-transaction')) openEditTransaction(target.dataset.id);
  
  if (target.classList.contains('delete-transaction')) {
    openConfirm('Delete transaction', 'Delete this transaction? This action cannot be undone.', '', () => {
      fetchJSON(`/api/transactions/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshTransactionsTab(); showToast('Transaction deleted', 'success'); })
        .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to delete transaction', 'error'));
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
      .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to update task', 'error'));
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
        showToast(sanitizeErrorMsg(err.message) || 'Failed to delete task', 'error');
        checkbox.disabled = false;
        checkbox.checked = false;
        li.classList.remove('task-strike');
      });
  }, 2000);
});

// --- General Buttons ---
fundingOptionsList && fundingOptionsList.addEventListener('click', async (event) => {
  const button = event.target.closest('.fund-option');
  if (!button) return;
  
  if (!pendingTransaction || !pendingTransaction.payload) {
    return showToast('No pending transaction to remediate.', 'error');
  }
  
  const { payload, method, url } = pendingTransaction;
  
  button.disabled = true;
  button.textContent = 'Processing...';
  
  const mechanism = button.dataset.mechanism;
  const sourceWalletId = button.dataset.sourceWalletId;
  const amount = parseFloat(button.dataset.amount);
  
  const remediationPayload = {
    type: mechanism,
    amount: amount,
    source_wallet_id: sourceWalletId,
    destination_wallet_id: payload.source_wallet_id,
    date: payload.date || new Date().toISOString().split('T')[0],
    details: `Funding remediation for shortfall via ${mechanism}`
  };
  
  try {
    await fetchJSON('/api/transactions', { 
      method: 'POST', 
      headers: { 'Content-Type': 'application/json' }, 
      body: JSON.stringify(remediationPayload) 
    });
    
    await fetchJSON(url, { 
      method: method, 
      headers: { 'Content-Type': 'application/json' }, 
      body: JSON.stringify(payload) 
    });
    
    setPendingTransaction(null);
    hideFundingOptions();
    refreshTransactionsTab();
    resetTransactionForm();
    showToast('Remediation and transaction applied successfully', 'success');
    
  } catch (err) {
    button.disabled = false;
    button.textContent = 'Commit & Pay';
    showToast(sanitizeErrorMsg(err.message) || 'Remediation failed', 'error');
  }
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
    .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to settle loan', 'error'));
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
        systemStatus.textContent = `Month-end failed: ${sanitizeErrorMsg(err.message)}`;
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
    .catch((err) => showToast(sanitizeErrorMsg(err.message) || 'Failed to settle loan', 'error'));
});

deleteAllTasksBtn && deleteAllTasksBtn.addEventListener('click', () => {
  if (!currentTasks.length) return showToast('No tasks to delete', 'error');
  openConfirm('Delete all tasks', `Delete all ${currentTasks.length} task${currentTasks.length === 1 ? '' : 's'}? This cannot be undone.`, '', () => {
    fetchJSON(`/api/tasks`, { method: 'DELETE' })
      .then(() => { showToast('All tasks deleted', 'success'); refreshTasks(); })
      .catch((err) => { showToast(sanitizeErrorMsg(err.message) || 'Failed to delete all tasks', 'error'); refreshTasks(); });
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
  loadProfile();
  refreshData();
  refreshTasks();
}

init();