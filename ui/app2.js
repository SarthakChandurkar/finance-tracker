// app2.js — restored richer frontend behavior for Finance Tracker

const views = document.querySelectorAll('.view');
const navButtons = document.querySelectorAll('nav button');
const navMenu = document.querySelector('nav');
const hamburgerToggle = document.getElementById('hamburger-toggle');
const txType = document.getElementById('tx-type');
const txSource = document.getElementById('tx-source');
const txDestination = document.getElementById('tx-destination');
const txCategory = document.getElementById('tx-category');
const txPaymentMode = document.getElementById('tx-payment-mode');
const txPaymentInstrument = document.getElementById('tx-payment-instrument');
const txDate = document.getElementById('tx-date');
const transactionForm = document.getElementById('transaction-form');
const txSubmit = document.getElementById('tx-submit');
const txCancel = document.getElementById('tx-cancel');
const walletMode = document.getElementById('wallet-mode');
const walletForm = document.getElementById('wallet-form');
const walletSubmit = document.getElementById('wallet-submit');
const walletCancel = document.getElementById('wallet-cancel');
const categoryForm = document.getElementById('category-form');
const recategorizeForm = document.getElementById('recategorize-form');
const sourceCategorySelect = document.getElementById('source-category');
const destCategorySelect = document.getElementById('dest-category');
const walletModeNote = document.getElementById('wallet-mode-note');
const walletTableBody = document.querySelector('#wallet-table tbody');
const transactionTableBody = document.querySelector('#transaction-table tbody');
const categoryTableBody = document.querySelector('#category-table tbody');
const walletBreakdowns = document.getElementById('wallet-breakdowns');
const categoryTotals = document.getElementById('category-totals');
const loanLedger = document.getElementById('loan-ledger');
const interWalletLoanLedger = document.getElementById('interwallet-loan-ledger');
const fundingPanel = document.getElementById('funding-panel');
const fundingMessage = document.getElementById('funding-message');
const fundingOptionsList = document.getElementById('funding-options-list');
const fundingCancel = document.getElementById('funding-cancel');
const loanSettlePanel = document.getElementById('loan-settle-panel');
const settleCounterparty = document.getElementById('settle-counterparty');
const settleAmount = document.getElementById('settle-amount');
const settleSource = document.getElementById('settle-source');
const settleSend = document.getElementById('settle-send');
const settleCancel = document.getElementById('settle-cancel');
const interWalletSettlePanel = document.getElementById('interwallet-settle-panel');
const interWalletSettleLabel = document.getElementById('interwallet-settle-label');
const interWalletSettleAmount = document.getElementById('interwallet-settle-amount');
const interWalletSettleSend = document.getElementById('interwallet-settle-send');
const interWalletSettleCancel = document.getElementById('interwallet-settle-cancel');
const systemStatus = document.getElementById('system-status');
const monthEndButton = document.getElementById('run-month-end');
const taskForm = document.getElementById('task-form');
const taskTitleInput = document.getElementById('task-title');
const taskDueInput = document.getElementById('task-due');
const taskList = document.getElementById('task-list');
const deleteAllTasksBtn = document.getElementById('delete-all-tasks');
const todayTasksEl = document.getElementById('today-tasks');
const confirmModal = document.getElementById('confirm-modal');
const confirmTitle = document.getElementById('confirm-title');
const confirmMessage = document.getElementById('confirm-message');
const confirmDetails = document.getElementById('confirm-details');
const confirmYes = document.getElementById('confirm-yes');
const confirmNo = document.getElementById('confirm-no');

const transactionTypes = ['Debit', 'Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Wallet Loan'];
const walletModeOptions = ['Both', 'Monthly Only', 'Global Only'];
const paymentModes = ['Self', 'On Behalf of Other'];
const paymentInstruments = ['Cash', 'UPI', 'Card', 'Bank Transfer', 'Cheque'];

let currentWallets = [];
let currentCategories = [];
let currentTransactions = [];
let currentTasks = [];
let pendingTransaction = null;
let editingWalletId = null;
let editingTransactionId = null;
let editingTaskId = null;
let confirmCallback = null;

function showToast(message, type = 'success', ms = 4000) {
  const toasts = document.getElementById('toasts');
  if (!toasts) return;
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = message;
  toasts.appendChild(el);
  setTimeout(() => el.remove(), ms);
}

function setFormError(id, msg) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = msg || '';
}

async function fetchJSON(url, options = {}) {
  // 1. Explicitly tell the server we want JSON, not the HTML template
  options.headers = {
    'Accept': 'application/json',
    ...(options.headers || {})
  };

  const res = await fetch(url, options);
  const text = await res.text();
  let body = null;
  
  try { 
    body = text ? JSON.parse(text) : null; 
  } catch (e) { 
    body = text; 
  }
  
  if (!res.ok) {
    const err = new Error((body && body.error) || text || res.statusText);
    err.status = res.status;
    err.body = body;
    throw err;
  }
  
  return body;
}

function normalizeFormNumbers(payload, fields) {
  fields.forEach((field) => {
    if (!Object.prototype.hasOwnProperty.call(payload, field)) return;
    
    const value = payload[field];
    
    // Instead of deleting the key, we set it to null so the backend 
    // knows to clear out the existing data in the database.
    if (value === '' || value === null || value === undefined) {
      payload[field] = null; 
      return;
    }
    
    const num = Number(value);
    payload[field] = Number.isNaN(num) ? value : num;
  });
}

function setActiveView(viewId) {
  views.forEach((view) => view.classList.toggle('active', view.id === viewId));
  navButtons.forEach((button) => button.classList.toggle('active', button.dataset.view === viewId));
  if (navMenu) {
    navMenu.classList.remove('open');
  }
}

navButtons.forEach((button) => button.addEventListener('click', () => {
  setActiveView(button.dataset.view);
  refreshData();
}));

hamburgerToggle && hamburgerToggle.addEventListener('click', () => {
  if (navMenu) {
    navMenu.classList.toggle('open');
  }
});

function buildSelectOptions(select, values, includeEmpty = true) {
  if (!select) return;
  const items = includeEmpty ? [{ value: '', label: '-- Select --' }, ...values] : values;
  select.innerHTML = items.map((value) => {
    if (typeof value === 'object') {
      return `<option value="${value.value}">${value.label}</option>`;
    }
    return `<option value="${value}">${value}</option>`;
  }).join('');
}

function formatDate(dateString) {
  const d = new Date(dateString);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// Used any time task/user-entered text gets dropped into innerHTML, so a
// task titled e.g. "<script>" can't break out of its <span>.
function escapeHTML(str) {
  const div = document.createElement('div');
  div.textContent = str == null ? '' : String(str);
  return div.innerHTML;
}

// The backend sends due_date as a full RFC3339 timestamp (e.g.
// "2026-07-30T00:00:00Z"), since Task.Date is a Go time.Time with no
// custom JSON marshaling. <input type="date"> and our own "is this
// today?" check both need just the "YYYY-MM-DD" part, so this pulls that
// out — and still works unchanged if the backend ever starts sending a
// plain date string instead.
function toDateOnly(dateString) {
  if (!dateString) return '';
  return String(dateString).slice(0, 10);
}

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
    // Salary is a paycheck, never loan activity — hide Counterparty so it
    // can't accidentally get tagged with a loan counterparty and show up
    // in the Loan Ledger.
    counterpartyLabel.parentElement.style.display = type === 'Salary' ? 'none' : '';
  }

  populateDestinationOptions(type);
}



// Credit/Salary/Loan Received can land in either a Global Only wallet or a
// Monthly Only wallet — both are tracked correctly, so no scope filtering
// is needed here; every active wallet is a valid destination.
function populateDestinationOptions(type) {
  if (!txDestination) return;
  const pool = currentWallets;
  const previousValue = txDestination.value;
  buildSelectOptions(txDestination, pool.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  if (pool.some((q) => q.id === previousValue)) {
    txDestination.value = previousValue;
  }
}

function updateWalletFormFields() {
  if (!walletMode) return;
  const mode = walletMode.value;
  const targetField = document.getElementById('wallet-target-amount').parentElement;
  const eomField = document.getElementById('wallet-eom-destination').parentElement;
  if (mode === 'Monthly Only') {
    eomField.style.display = '';
    targetField.style.display = 'none';
  } else if (mode === 'Global Only') {
    eomField.style.display = 'none';
    targetField.style.display = '';
  } else {
    eomField.style.display = '';
    targetField.style.display = '';
  }
}

// Wallet deletion is permanent (the record is removed from storage), so an
// older transaction or an EOM Sweep Destination can end up pointing at an
// ID that no longer resolves to any wallet. Rather than leaking that raw
// internal ID into the UI, show a clear "DELETED" marker.
function walletLabelHTML(walletId) {
  if (!walletId) return '';
  const q = findWallet(walletId);
  if (q) return q.name;
  return '<span style="color:#dc2626;font-weight:600;">DELETED</span>';
}

function renderTransactions(transactions) {
  if (!transactionTableBody) return;
  transactionTableBody.innerHTML = (transactions || []).map((t) => `
    <tr>
      <td>${t.date || ''}</td>
      <td>${t.type || ''}</td>
      <td>${typeof t.amount === 'number' ? t.amount.toFixed(2) : t.amount || ''}</td>
      <td>${walletLabelHTML(t.source_wallet_id)}</td>
      <td>${walletLabelHTML(t.destination_wallet_id)}</td>
      <td>${escapeHTML(t.category || '')}</td>
      <td>${escapeHTML(t.counterparty || '')}</td>
      <td>${escapeHTML(t.details || '')}</td>
      <td class="actions-cell">
        <button class="edit-transaction" data-id="${t.id}">Edit</button>
        <button class="delete-transaction" data-id="${t.id}">Delete</button>
      </td>
    </tr>
  `).join('');
}


function renderWallets(wallets) {
  if (!walletTableBody) return;
  walletTableBody.innerHTML = (wallets || []).map((q) => `
    <tr>
      <td>${q.name}</td>
      <td>${q.scope || ''}</td>
      <td></td>
      <td>${q.target_amount || ''}</td>
      <td>${walletLabelHTML(q.eom_sweep_destination)}</td>
      <td>
        <button class="edit-wallet" data-id="${q.id}">Edit</button>
        ${q.id === 'savings' ? '' : `<button class="delete-wallet" data-id="${q.id}">Delete</button>`}
      </td>
    </tr>
  `).join('');
}

function renderCategories(categories) {
  if (!categoryTableBody) return;
  categoryTableBody.innerHTML = (categories || []).map((c) => `
    <tr>
      <td>${c.name}</td>
      <td>
        <button class="delete-category" data-id="${c.id}">Delete</button>
      </td>
    </tr>
  `).join('');
}

function renderLoanLedger(entries) {
  if (!loanLedger) return;
  loanLedger.innerHTML = (entries || []).map((entry) => `
    <div class="loan-entry">
      <span>${entry.counterparty}: outstanding ${Number(entry.outstanding_external_loan || 0).toFixed(2)}</span>
      <button class="settle-loan" data-counterparty="${entry.counterparty}">Settle</button>
    </div>
  `).join('');
}

function renderInterWalletLoanLedger(entries) {
  if (!interWalletLoanLedger) return;
  interWalletLoanLedger.innerHTML = (entries || []).map((entry) => `
    <div class="loan-entry">
      <span>${entry.lender_wallet_name} &rarr; ${entry.borrower_wallet_name}: outstanding ${Number(entry.outstanding || 0).toFixed(2)}</span>
      <button class="settle-interwallet-loan"
        data-lender-id="${entry.lender_wallet_id}"
        data-lender-name="${entry.lender_wallet_name}"
        data-borrower-id="${entry.borrower_wallet_id}"
        data-borrower-name="${entry.borrower_wallet_name}">Settle</button>
    </div>
  `).join('');
}

function renderDashboard(walletData, categoryData) {
  if (walletBreakdowns && walletData) {
    walletBreakdowns.innerHTML = `
      <div>
        <h4>Monthly Wallets</h4>
        <ul>${(walletData.monthly || []).map((q) => `<li>${q.wallet_name || q.wallet_id}: spent ${Number(q.debited).toFixed(2)}, available ${Number(q.available_balance).toFixed(2)}</li>`).join('')}</ul>
      </div>
      <div>
        <h4>Global Wallets</h4>
        <ul>${(walletData.global || []).map((q) => `<li>${q.wallet_name || q.wallet_id}: accumulated ${Number(q.accumulated).toFixed(2)}</li>`).join('')}</ul>
      </div>
    `;
  }
  if (categoryTotals && categoryData) {
    categoryTotals.innerHTML = `
      <div>
        <h4>Monthly Totals</h4>
        <ul>${(categoryData.monthly || []).map((c) => `<li>${c.category_name}: ${Number(c.total).toFixed(2)}</li>`).join('')}</ul>
      </div>
      <div>
        <h4>Global Totals</h4>
        <ul>${(categoryData.global || []).map((c) => `<li>${c.category_name}: ${Number(c.total).toFixed(2)}</li>`).join('')}</ul>
      </div>
    `;
  }
}

// A task row renders in one of two states: normal (checkbox + title +
// pencil), or editing (title/date inputs + save/cancel), tracked via the
// single `editingTaskId` — same one-at-a-time pattern as
// editingWalletId/editingTransactionId above.
function renderTasks(tasks) {
  if (!taskList) return;
  if (!tasks || !tasks.length) {
    taskList.innerHTML = '<li class="task-empty">No tasks yet — add one above.</li>';
    return;
  }
  taskList.innerHTML = tasks.map((t) => {
    if (String(editingTaskId) === String(t.id)) {
      return `
        <li class="task-item editing" data-id="${t.id}">
          <div class="task-body">
            <input class="task-title-edit" type="text" value="${escapeHTML(t.title || '')}" data-id="${t.id}" />
            <input class="task-due-edit" type="date" value="${toDateOnly(t.due_date)}" data-id="${t.id}" />
          </div>
          <div class="task-edit-actions">
            <button class="task-save-btn" data-id="${t.id}" type="button" aria-label="Save task">&#10003;</button>
            <button class="task-cancel-btn" data-id="${t.id}" type="button" aria-label="Cancel edit">&#10005;</button>
          </div>
        </li>
      `;
    }
    return `
      <li class="task-item" data-id="${t.id}">
        <label class="task-check">
          <input type="checkbox" class="task-delete-checkbox" data-id="${t.id}" />
          <span class="checkmark"></span>
        </label>
        <div class="task-body">
          <span class="task-title">${escapeHTML(t.title || '')}</span>
          ${t.due_date ? `<span class="task-due">${toDateOnly(t.due_date)}</span>` : ''}
        </div>
        <button class="task-edit-btn" data-id="${t.id}" type="button" aria-label="Edit task">&#9998;</button>
      </li>
    `;
  }).join('');
}

// Dashboard card: today's still-pending tasks. A task counts as "pending"
// simply by existing (deletion is real removal via DELETE, there's no
// separate done/not-done flag) — so this is just "due today".
function renderTodayTasks(tasks) {
  if (!todayTasksEl) return;
  const today = formatDate(new Date());
  const todays = (tasks || []).filter((t) => toDateOnly(t.due_date) === today);
  if (!todays.length) {
    todayTasksEl.innerHTML = '<p class="note">No tasks due today.</p>';
    return;
  }
  todayTasksEl.innerHTML = `<ul class="today-task-list">${todays.map((t) => `<li>${escapeHTML(t.title || '')}</li>`).join('')}</ul>`;
}

// Tasks are now relayed through our backend to an external task server
// (Server B) that isn't stored locally anymore. That server spins down
// when idle, so the very first request after a while can take up to a
// minute to come back while it wakes up. tasksLoaded tracks whether
// we've had at least one successful response since the page loaded, so
// that wait is only shown once — later refreshes (after adding/editing/
// deleting a task) don't blank the list out again.
let tasksLoaded = false;

function showTasksConnecting() {
  if (taskList) {
    taskList.innerHTML = '<li class="task-empty task-connecting">Connecting to task server… this can take up to a minute if it has been idle.</li>';
  }
  if (todayTasksEl) {
    todayTasksEl.innerHTML = '<p class="note task-connecting">Connecting to task server…</p>';
  }
}

async function refreshTasks() {
  if (!tasksLoaded) {
    showTasksConnecting();
  }
  try {
    const rawTasks = await fetchJSON('/api/tasks');
    
    // THE FIX: Loop through the tasks and map MongoDB's '_id' to standard 'id'.
    // This instantly fixes your checkboxes, edit buttons, and delete requests!
    currentTasks = (rawTasks || []).map(t => {
      t.id = t._id || t.id;
      return t;
    });
    
    tasksLoaded = true;
    renderTasks(currentTasks);
    renderTodayTasks(currentTasks);
  } catch (err) {
    showToast(err.message || 'Failed to load tasks', 'error');
    if (!tasksLoaded) {
      if (taskList) taskList.innerHTML = '<li class="task-empty">Couldn\u2019t reach the task server. Try again shortly.</li>';
      if (todayTasksEl) todayTasksEl.innerHTML = '<p class="note">Couldn\u2019t reach the task server.</p>';
    }
  }
}

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

function openConfirm(title, message, details, callback) {
  if (!confirmModal) return callback && callback();
  confirmTitle.textContent = title || 'Confirm';
  confirmMessage.textContent = message || '';
  confirmDetails.textContent = details || '';
  confirmCallback = callback;
  confirmModal.classList.remove('hidden');
}

function closeConfirm() {
  if (!confirmModal) return;
  confirmCallback = null;
  confirmModal.classList.add('hidden');
}

confirmYes && confirmYes.addEventListener('click', () => {
  if (typeof confirmCallback === 'function') {
    try { confirmCallback(); } catch (e) { console.error(e); }
  }
  closeConfirm();
});
confirmNo && confirmNo.addEventListener('click', closeConfirm);

function setSystemStatus(message, isError = false) {
  if (!systemStatus) return;
  systemStatus.textContent = message;
  systemStatus.style.color = isError ? '#dc2626' : '#111827';
}

function populateSelects() {
  buildSelectOptions(txType, transactionTypes, false);
  buildSelectOptions(walletMode, walletModeOptions, false);
  buildSelectOptions(txPaymentMode, paymentModes, false);
  buildSelectOptions(txPaymentInstrument, paymentInstruments, false);
}

function resetTransactionForm() {
  if (!transactionForm) return;
  transactionForm.reset();
  if (txDate) txDate.value = formatDate(new Date());
  editingTransactionId = null;
  transactionForm.querySelector('#transaction-form-heading').textContent = 'Create Transaction';
  txSubmit.textContent = 'Save Transaction';
  setFormError('tx-error', '');
  updateTransactionFormFields();
}

function resetWalletForm() {
  if (!walletForm) return;
  walletForm.reset();
  editingWalletId = null;
  walletMode.disabled = false;
  walletModeNote.textContent = '';
  walletForm.querySelector('#wallet-form-heading').textContent = 'Create Wallet';
  walletSubmit.textContent = 'Create Wallet';
  setFormError('wallet-error', '');
  updateWalletFormFields();
}

function openLoanSettlePanel(counterparty) {
  if (!loanSettlePanel) return;
  settleCounterparty.textContent = counterparty;
  settleSource.innerHTML = currentWallets.map((q) => `<option value="${q.id}">${q.name} (${q.scope})</option>`).join('');
  settleAmount.value = '';
  loanSettlePanel.classList.remove('hidden');
}

function hideLoanSettlePanel() {
  if (!loanSettlePanel) return;
  loanSettlePanel.classList.add('hidden');
}

let pendingInterWalletSettlement = null;

function openInterWalletSettlePanel(lenderId, lenderName, borrowerId, borrowerName) {
  if (!interWalletSettlePanel) return;
  pendingInterWalletSettlement = { lenderId, borrowerId };
  interWalletSettleLabel.textContent = `${borrowerName} owes ${lenderName}`;
  interWalletSettleAmount.value = '';
  interWalletSettlePanel.classList.remove('hidden');
}

function hideInterWalletSettlePanel() {
  if (!interWalletSettlePanel) return;
  pendingInterWalletSettlement = null;
  interWalletSettlePanel.classList.add('hidden');
}

fundingOptionsList && fundingOptionsList.addEventListener('click', (event) => {
  const button = event.target.closest('.fund-option');
  if (!button) return;
  if (!pendingTransaction) {
    showToast('No pending transaction to remediate.', 'error');
    return;
  }
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
  fetchJSON('/api/funding-remediation', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      pendingTransaction = null;
      hideFundingOptions();
      refreshData();
      showToast('Funding remediation applied', 'success');
    })
    .catch((err) => showToast(err.message || 'Remediation failed', 'error'));
});

fundingCancel && fundingCancel.addEventListener('click', () => {
  pendingTransaction = null;
  hideFundingOptions();
});

settleCancel && settleCancel.addEventListener('click', hideLoanSettlePanel);
settleSend && settleSend.addEventListener('click', () => {
  const amount = parseFloat(settleAmount.value);
  const source = settleSource.value;
  const counterparty = settleCounterparty.textContent;
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!source) return showToast('Select a source wallet', 'error');
  const payload = {
    type: 'Debit',
    amount,
    source_wallet_id: source,
    category: 'Settle',
    counterparty,
    date: new Date().toISOString().split('T')[0],
    details: 'Loan settlement',
  };
  fetchJSON('/api/transactions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      hideLoanSettlePanel();
      refreshData();
      showToast('Loan settlement recorded', 'success');
    })
    .catch((err) => showToast(err.message || 'Failed to settle loan', 'error'));
});

monthEndButton && monthEndButton.addEventListener('click', () => {
  setSystemStatus('Running month-end sweep...', false);
  fetchJSON('/api/schedule/end', { method: 'POST' })
    .then(() => { setSystemStatus('Month-end sweep completed.'); refreshData(); })
    .catch((err) => setSystemStatus(`Month-end failed: ${err.message}`, true));
});

interWalletSettleCancel && interWalletSettleCancel.addEventListener('click', hideInterWalletSettlePanel);
interWalletSettleSend && interWalletSettleSend.addEventListener('click', () => {
  const amount = parseFloat(interWalletSettleAmount.value);
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!pendingInterWalletSettlement) return showToast('No loan selected to settle', 'error');
  const payload = {
    borrower_wallet_id: pendingInterWalletSettlement.borrowerId,
    lender_wallet_id: pendingInterWalletSettlement.lenderId,
    amount,
  };
  fetchJSON('/api/inter-wallet-loans/settle', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      hideInterWalletSettlePanel();
      refreshData();
      showToast('Inter-Wallet Loan settlement recorded', 'success');
    })
    .catch((err) => showToast(err.message || 'Failed to settle loan', 'error'));
});

document.addEventListener('click', (event) => {
  const target = event.target;
  if (target.classList.contains('delete-category')) {
    const id = target.dataset.id;
    openConfirm('Delete category', 'Delete this category? Transactions will be reassigned to Settled.', '', () => {
      fetchJSON(`/api/categories/${id}`, { method: 'DELETE' })
        .then(() => { refreshData(); showToast('Category deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete category', 'error'));
    });
  }
  if (target.classList.contains('edit-wallet')) {
    openEditWallet(target.dataset.id);
  }
  if (target.classList.contains('delete-wallet')) {
    openConfirm('Delete wallet', 'Delete this wallet permanently? Its remaining balance will be transferred to Savings. This cannot be undone.', '', () => {
      fetchJSON(`/api/wallets/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshData(); showToast('Wallet deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete wallet', 'error'));
    });
  }
  if (target.classList.contains('edit-transaction')) {
    openEditTransaction(target.dataset.id);
  }
  if (target.classList.contains('delete-transaction')) {
    openConfirm('Delete transaction', 'Delete this transaction? This action cannot be undone.', '', () => {
      fetchJSON(`/api/transactions/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshData(); showToast('Transaction deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete transaction', 'error'));
    });
  }
  if (target.classList.contains('task-edit-btn')) {
    editingTaskId = target.dataset.id;
    renderTasks(currentTasks);
  }
  if (target.classList.contains('task-cancel-btn')) {
    editingTaskId = null;
    renderTasks(currentTasks);
  }
  if (target.classList.contains('task-save-btn')) {
    const id = target.dataset.id;
    const li = target.closest('.task-item');
    const titleInput = li && li.querySelector('.task-title-edit');
    const dueInput = li && li.querySelector('.task-due-edit');
    const title = titleInput ? titleInput.value.trim() : '';
    if (!title) {
      showToast('Task title cannot be empty', 'error');
      return;
    }
    const payload = { title, due_date: (dueInput && dueInput.value) || '' };
    fetchJSON(`/api/tasks/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
      .then(() => {
        editingTaskId = null;
        showToast('Task updated', 'success');
        refreshData();
      })
      .catch((err) => showToast(err.message || 'Failed to update task', 'error'));
  }
  if (target.classList.contains('settle-loan')) {
    openLoanSettlePanel(target.dataset.counterparty);
  }
  if (target.classList.contains('settle-interwallet-loan')) {
    openInterWalletSettlePanel(
      target.dataset.lenderId,
      target.dataset.lenderName,
      target.dataset.borrowerId,
      target.dataset.borrowerName,
    );
  }
});

function findWallet(id) {
  return currentWallets.find((q) => String(q.id) === String(id));
}

function findCategory(id) {
  return currentCategories.find((c) => String(c.id) === String(id));
}

function openEditWallet(id) {
  const wallet = findWallet(id);
  if (!wallet) return showToast('Wallet not found', 'error');
  
  editingWalletId = id;
  walletForm.querySelector('#wallet-name').value = wallet.name || '';
  walletForm.querySelector('#wallet-mode').value = wallet.scope || 'Both';
  // Changed to ?? to prevent 0 from being erased
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
  
  editingTransactionId = id;
  transactionForm.querySelector('#tx-type').value = txn.type || 'Debit';
  // Changed to ?? to prevent 0 from being erased
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
      refreshData();
    })
    .catch((err) => {
      if (err.status === 422 && err.body && err.body.options) {
        pendingTransaction = payload;
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
  if (payload.mode === 'Global Only' && payload.monthly_allocation) {
    return setFormError('wallet-error', 'Monthly allocation is not applicable to Global Only wallets');
  }
  const method = editingWalletId ? 'PUT' : 'POST';
  const url = editingWalletId ? `/api/wallets/${editingWalletId}` : '/api/wallets';
  fetchJSON(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      showToast(editingWalletId ? 'Wallet updated' : 'Wallet created', 'success');
      resetWalletForm();
      refreshData();
    })
    .catch((err) => {
      setFormError('wallet-error', err.message || 'Failed to save wallet');
    });
});

categoryForm && categoryForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(categoryForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category created', 'success'); categoryForm.reset(); refreshData(); })
    .catch((err) => showToast(err.message || 'Failed to create category', 'error'));
});

recategorizeForm && recategorizeForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const fd = new FormData(recategorizeForm);
  const payload = Object.fromEntries(fd.entries());
  fetchJSON('/api/categories/recategorize', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => { showToast('Category recategorized', 'success'); recategorizeForm.reset(); refreshData(); })
    .catch((err) => showToast(err.message || 'Failed to recategorize category', 'error'));
});

taskForm && taskForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  setFormError('task-error', '');
  const title = taskTitleInput.value.trim();
  if (!title) {
    setFormError('task-error', 'Task title is required');
    return;
  }
  const payload = { title, due_date: taskDueInput.value || formatDate(new Date()) };
  fetchJSON('/api/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      taskForm.reset();
      if (taskDueInput) taskDueInput.value = formatDate(new Date());
      showToast('Task added', 'success');
      refreshData();
    })
    .catch((err) => setFormError('task-error', err.message || 'Failed to add task'));
});

// Checking the box IS the delete action (there's no separate "mark done"
// state) — strike the title through for 2s so the click reads as
// confirmed, then actually DELETE it. If the DELETE fails, roll the
// checkbox and strikethrough back so the task doesn't look gone when it
// isn't.
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
      .then(() => {
        showToast('Task deleted', 'success');
        refreshData();
      })
      .catch((err) => {
        showToast(err.message || 'Failed to delete task', 'error');
        checkbox.disabled = false;
        checkbox.checked = false;
        li.classList.remove('task-strike');
      });
  }, 2000);
});

deleteAllTasksBtn && deleteAllTasksBtn.addEventListener('click', () => {
  if (!currentTasks.length) {
    showToast('No tasks to delete', 'error');
    return;
  }
  
  openConfirm(
    'Delete all tasks',
    `Delete all ${currentTasks.length} task${currentTasks.length === 1 ? '' : 's'}? This cannot be undone.`,
    '',
    () => {
      // Fires ONE single network request to wipe the collection
      fetchJSON(`/api/tasks`, { method: 'DELETE' })
        .then(() => {
          showToast('All tasks deleted', 'success');
          refreshData();
        })
        .catch((err) => {
          showToast(err.message || 'Failed to delete all tasks', 'error');
          refreshTasks(); 
        });
    },
  );
});

txType && txType.addEventListener('change', updateTransactionFormFields);
walletMode && walletMode.addEventListener('change', updateWalletFormFields);
txCancel && txCancel.addEventListener('click', resetTransactionForm);
walletCancel && walletCancel.addEventListener('click', resetWalletForm);

function refreshWalletsAndCategories() {
  buildSelectOptions(txSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  populateDestinationOptions(txType ? txType.value : '');
  buildSelectOptions(txCategory, currentCategories.map((c) => ({ value: c.name, label: c.name })));
  buildSelectOptions(sourceCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(destCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(settleSource, currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  buildSelectOptions(document.getElementById('wallet-eom-destination'), currentWallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
}

async function refreshData() {
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
    renderTransactions(currentTransactions);
    renderLoanLedger(loanEntries || []);
    renderInterWalletLoanLedger(interWalletLoanEntries || []);
    renderDashboard(walletTotals, categoryTotalsData);
    refreshWalletsAndCategories();
  } catch (err) {
    showToast(err.message || 'Failed to refresh data', 'error');
  }
  // Kept out of the Promise.all above deliberately: refreshTasks() has its
  // own try/catch, so if /api/tasks isn't live on the backend yet, it
  // won't take down the rest of the dashboard refresh with it.
  refreshTasks();
}

if (txDate && !txDate.value) txDate.value = formatDate(new Date());
if (taskDueInput && !taskDueInput.value) taskDueInput.value = formatDate(new Date());
populateSelects();
updateTransactionFormFields();
updateWalletFormFields();
refreshData();