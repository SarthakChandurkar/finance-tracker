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
const quotaMode = document.getElementById('quota-mode');
const quotaForm = document.getElementById('quota-form');
const quotaSubmit = document.getElementById('quota-submit');
const quotaCancel = document.getElementById('quota-cancel');
const categoryForm = document.getElementById('category-form');
const recategorizeForm = document.getElementById('recategorize-form');
const sourceCategorySelect = document.getElementById('source-category');
const destCategorySelect = document.getElementById('dest-category');
const quotaModeNote = document.getElementById('quota-mode-note');
const quotaTableBody = document.querySelector('#quota-table tbody');
const transactionTableBody = document.querySelector('#transaction-table tbody');
const categoryTableBody = document.querySelector('#category-table tbody');
const quotaBreakdowns = document.getElementById('quota-breakdowns');
const categoryTotals = document.getElementById('category-totals');
const loanLedger = document.getElementById('loan-ledger');
const interQuotaLoanLedger = document.getElementById('interquota-loan-ledger');
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
const interQuotaSettlePanel = document.getElementById('interquota-settle-panel');
const interQuotaSettleLabel = document.getElementById('interquota-settle-label');
const interQuotaSettleAmount = document.getElementById('interquota-settle-amount');
const interQuotaSettleSend = document.getElementById('interquota-settle-send');
const interQuotaSettleCancel = document.getElementById('interquota-settle-cancel');
const systemStatus = document.getElementById('system-status');
const monthEndButton = document.getElementById('run-month-end');
const confirmModal = document.getElementById('confirm-modal');
const confirmTitle = document.getElementById('confirm-title');
const confirmMessage = document.getElementById('confirm-message');
const confirmDetails = document.getElementById('confirm-details');
const confirmYes = document.getElementById('confirm-yes');
const confirmNo = document.getElementById('confirm-no');

const transactionTypes = ['Debit', 'Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Quota Loan'];
const quotaModeOptions = ['Both', 'Monthly Only', 'Global Only'];
const paymentModes = ['Self', 'On Behalf of Other'];
const paymentInstruments = ['Cash', 'UPI', 'Card', 'Bank Transfer', 'Cheque'];

let currentQuotas = [];
let currentCategories = [];
let currentTransactions = [];
let pendingTransaction = null;
let editingQuotaId = null;
let editingTransactionId = null;
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
  const res = await fetch(url, options);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch (e) { body = text; }
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
    if (value === '' || value === null || value === undefined) {
      delete payload[field];
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

navButtons.forEach((button) => button.addEventListener('click', () => setActiveView(button.dataset.view)));

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

function updateTransactionFormFields() {
  if (!txType) return;
  const type = txType.value;
  const categoryField = document.querySelector('label[for="tx-category"]').parentElement;
  const destinationField = document.querySelector('label[for="tx-destination"]').parentElement;
  const counterpartyField = document.querySelector('label[for="tx-counterparty"]').parentElement;
  categoryField.style.display = type === 'Debit' ? '' : 'none';
  const destinationRequiredTypes = ['Credit', 'Salary', 'Loan Received', 'Self Transfer', 'Inter-Quota Loan'];
  destinationField.style.display = destinationRequiredTypes.includes(type) ? '' : 'none';
  // Salary is a paycheck, never loan activity — hide Counterparty so it
  // can't accidentally get tagged with a loan counterparty and show up
  // in the Loan Ledger.
  counterpartyField.style.display = type === 'Salary' ? 'none' : '';
  populateDestinationOptions(type);
}

// Credit/Salary/Loan Received can land in either a Global Only quota or a
// Monthly Only quota — both are tracked correctly, so no scope filtering
// is needed here; every active quota is a valid destination.
function populateDestinationOptions(type) {
  if (!txDestination) return;
  const pool = currentQuotas;
  const previousValue = txDestination.value;
  buildSelectOptions(txDestination, pool.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  if (pool.some((q) => q.id === previousValue)) {
    txDestination.value = previousValue;
  }
}

function updateQuotaFormFields() {
  if (!quotaMode) return;
  const mode = quotaMode.value;
  const targetField = document.getElementById('quota-target-amount').parentElement;
  const eomField = document.getElementById('quota-eom-destination').parentElement;
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

// Quota deletion is permanent (the record is removed from storage), so an
// older transaction or an EOM Sweep Destination can end up pointing at an
// ID that no longer resolves to any quota. Rather than leaking that raw
// internal ID into the UI, show a clear "DELETED" marker.
function quotaLabelHTML(quotaId) {
  if (!quotaId) return '';
  const q = findQuota(quotaId);
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
      <td>${quotaLabelHTML(t.source_quota_id)}</td>
      <td>${quotaLabelHTML(t.destination_quota_id)}</td>
      <td>${t.category || ''}</td>
      <td>${t.counterparty || ''}</td>
      <td>${t.details || ''}</td>
      <td class="actions-cell">
        <button class="edit-transaction" data-id="${t.id}">Edit</button>
        <button class="delete-transaction" data-id="${t.id}">Delete</button>
      </td>
    </tr>
  `).join('');
}

function renderQuotas(quotas) {
  if (!quotaTableBody) return;
  quotaTableBody.innerHTML = (quotas || []).map((q) => `
    <tr>
      <td>${q.name}</td>
      <td>${q.scope || ''}</td>
      <td></td>
      <td>${q.target_amount || ''}</td>
      <td>${quotaLabelHTML(q.eom_sweep_destination)}</td>
      <td>
        <button class="edit-quota" data-id="${q.id}">Edit</button>
        ${q.id === 'savings' ? '' : `<button class="delete-quota" data-id="${q.id}">Delete</button>`}
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

function renderInterQuotaLoanLedger(entries) {
  if (!interQuotaLoanLedger) return;
  interQuotaLoanLedger.innerHTML = (entries || []).map((entry) => `
    <div class="loan-entry">
      <span>${entry.lender_quota_name} &rarr; ${entry.borrower_quota_name}: outstanding ${Number(entry.outstanding || 0).toFixed(2)}</span>
      <button class="settle-interquota-loan"
        data-lender-id="${entry.lender_quota_id}"
        data-lender-name="${entry.lender_quota_name}"
        data-borrower-id="${entry.borrower_quota_id}"
        data-borrower-name="${entry.borrower_quota_name}">Settle</button>
    </div>
  `).join('');
}

function renderDashboard(quotaData, categoryData) {
  if (quotaBreakdowns && quotaData) {
    quotaBreakdowns.innerHTML = `
      <div>
        <h4>Monthly Quotas</h4>
        <ul>${(quotaData.monthly || []).map((q) => `<li>${q.quota_name || q.quota_id}: spent ${Number(q.debited).toFixed(2)}, available ${Number(q.available_balance).toFixed(2)}</li>`).join('')}</ul>
      </div>
      <div>
        <h4>Global Quotas</h4>
        <ul>${(quotaData.global || []).map((q) => `<li>${q.quota_name || q.quota_id}: accumulated ${Number(q.accumulated).toFixed(2)}</li>`).join('')}</ul>
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
        <button class="fund-option" data-mechanism="${opt.mechanism}" data-source-quota-id="${opt.source_quota_id}">${opt.label || opt.mechanism}</button>
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
  buildSelectOptions(quotaMode, quotaModeOptions, false);
  buildSelectOptions(txPaymentMode, paymentModes, false);
  buildSelectOptions(txPaymentInstrument, paymentInstruments, false);
}

function resetTransactionForm() {
  if (!transactionForm) return;
  transactionForm.reset();
  editingTransactionId = null;
  transactionForm.querySelector('#transaction-form-heading').textContent = 'Create Transaction';
  txSubmit.textContent = 'Save Transaction';
  setFormError('tx-error', '');
  updateTransactionFormFields();
}

function resetQuotaForm() {
  if (!quotaForm) return;
  quotaForm.reset();
  editingQuotaId = null;
  quotaMode.disabled = false;
  quotaModeNote.textContent = '';
  quotaForm.querySelector('#quota-form-heading').textContent = 'Create Quota';
  quotaSubmit.textContent = 'Create Quota';
  setFormError('quota-error', '');
  updateQuotaFormFields();
}

function openLoanSettlePanel(counterparty) {
  if (!loanSettlePanel) return;
  settleCounterparty.textContent = counterparty;
  settleSource.innerHTML = currentQuotas.map((q) => `<option value="${q.id}">${q.name} (${q.scope})</option>`).join('');
  settleAmount.value = '';
  loanSettlePanel.classList.remove('hidden');
}

function hideLoanSettlePanel() {
  if (!loanSettlePanel) return;
  loanSettlePanel.classList.add('hidden');
}

let pendingInterQuotaSettlement = null;

function openInterQuotaSettlePanel(lenderId, lenderName, borrowerId, borrowerName) {
  if (!interQuotaSettlePanel) return;
  pendingInterQuotaSettlement = { lenderId, borrowerId };
  interQuotaSettleLabel.textContent = `${borrowerName} owes ${lenderName}`;
  interQuotaSettleAmount.value = '';
  interQuotaSettlePanel.classList.remove('hidden');
}

function hideInterQuotaSettlePanel() {
  if (!interQuotaSettlePanel) return;
  pendingInterQuotaSettlement = null;
  interQuotaSettlePanel.classList.add('hidden');
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
    source_quota_id: button.dataset.sourceQuotaId,
    target_quota_id: pendingTransaction.source_quota_id,
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
  if (!source) return showToast('Select a source quota', 'error');
  const payload = {
    type: 'Debit',
    amount,
    source_quota_id: source,
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

interQuotaSettleCancel && interQuotaSettleCancel.addEventListener('click', hideInterQuotaSettlePanel);
interQuotaSettleSend && interQuotaSettleSend.addEventListener('click', () => {
  const amount = parseFloat(interQuotaSettleAmount.value);
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!pendingInterQuotaSettlement) return showToast('No loan selected to settle', 'error');
  const payload = {
    borrower_quota_id: pendingInterQuotaSettlement.borrowerId,
    lender_quota_id: pendingInterQuotaSettlement.lenderId,
    amount,
  };
  fetchJSON('/api/inter-quota-loans/settle', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      hideInterQuotaSettlePanel();
      refreshData();
      showToast('Inter-Quota Loan settlement recorded', 'success');
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
  if (target.classList.contains('edit-quota')) {
    openEditQuota(target.dataset.id);
  }
  if (target.classList.contains('delete-quota')) {
    openConfirm('Delete quota', 'Delete this quota permanently? Its remaining balance will be transferred to Savings. This cannot be undone.', '', () => {
      fetchJSON(`/api/quotas/${target.dataset.id}`, { method: 'DELETE' })
        .then(() => { refreshData(); showToast('Quota deleted', 'success'); })
        .catch((err) => showToast(err.message || 'Failed to delete quota', 'error'));
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
  if (target.classList.contains('settle-loan')) {
    openLoanSettlePanel(target.dataset.counterparty);
  }
  if (target.classList.contains('settle-interquota-loan')) {
    openInterQuotaSettlePanel(
      target.dataset.lenderId,
      target.dataset.lenderName,
      target.dataset.borrowerId,
      target.dataset.borrowerName,
    );
  }
});

function findQuota(id) {
  return currentQuotas.find((q) => String(q.id) === String(id));
}

function findCategory(id) {
  return currentCategories.find((c) => String(c.id) === String(id));
}

function openEditQuota(id) {
  const quota = findQuota(id);
  if (!quota) return showToast('Quota not found', 'error');
  editingQuotaId = id;
  quotaForm.querySelector('#quota-name').value = quota.name || '';
  quotaForm.querySelector('#quota-mode').value = quota.scope || 'Both';
  quotaForm.querySelector('#quota-target-amount').value = quota.target_amount || '';
  quotaForm.querySelector('#quota-eom-destination').value = quota.eom_sweep_destination || '';
  quotaForm.querySelector('#quota-form-heading').textContent = 'Edit Quota';
  quotaSubmit.textContent = 'Save Quota';
  quotaMode.disabled = true;
  quotaModeNote.textContent = 'Quota scope cannot be changed while editing. Create a new quota to change scope.';
  updateQuotaFormFields();
}

function openEditTransaction(id) {
  const txn = currentTransactions.find((t) => String(t.id) === String(id));
  if (!txn) return showToast('Transaction not found', 'error');
  editingTransactionId = id;
  transactionForm.querySelector('#tx-type').value = txn.type || 'Debit';
  transactionForm.querySelector('#tx-amount').value = txn.amount || '';
  transactionForm.querySelector('#tx-source').value = txn.source_quota_id || '';
  transactionForm.querySelector('#tx-destination').value = txn.destination_quota_id || '';
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

quotaForm && quotaForm.addEventListener('submit', (ev) => {
  ev.preventDefault();
  setFormError('quota-error', '');
  const fd = new FormData(quotaForm);
  const payload = Object.fromEntries(fd.entries());
  normalizeFormNumbers(payload, ['target_amount']);
  if (payload.mode === 'Global Only' && payload.monthly_allocation) {
    return setFormError('quota-error', 'Monthly allocation is not applicable to Global Only quotas');
  }
  const method = editingQuotaId ? 'PUT' : 'POST';
  const url = editingQuotaId ? `/api/quotas/${editingQuotaId}` : '/api/quotas';
  fetchJSON(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    .then(() => {
      showToast(editingQuotaId ? 'Quota updated' : 'Quota created', 'success');
      resetQuotaForm();
      refreshData();
    })
    .catch((err) => {
      setFormError('quota-error', err.message || 'Failed to save quota');
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

txType && txType.addEventListener('change', updateTransactionFormFields);
quotaMode && quotaMode.addEventListener('change', updateQuotaFormFields);
txCancel && txCancel.addEventListener('click', resetTransactionForm);
quotaCancel && quotaCancel.addEventListener('click', resetQuotaForm);

function refreshQuotasAndCategories() {
  buildSelectOptions(txSource, currentQuotas.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  populateDestinationOptions(txType ? txType.value : '');
  buildSelectOptions(txCategory, currentCategories.map((c) => ({ value: c.name, label: c.name })));
  buildSelectOptions(sourceCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(destCategorySelect, currentCategories.map((c) => ({ value: c.id, label: c.name })));
  buildSelectOptions(settleSource, currentQuotas.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  buildSelectOptions(document.getElementById('quota-eom-destination'), currentQuotas.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
}

async function refreshData() {
  try {
    const [quotas, categories, transactions, quotaTotals, categoryTotalsData, loanEntries, interQuotaLoanEntries] = await Promise.all([
      fetchJSON('/api/quotas'),
      fetchJSON('/api/categories'),
      fetchJSON('/api/transactions'),
      fetchJSON('/api/quotas/breakdowns'),
      fetchJSON('/api/categories/totals'),
      fetchJSON('/api/loan-ledger'),
      fetchJSON('/api/inter-quota-loans'),
    ]);
    currentQuotas = quotas || [];
    currentCategories = categories || [];
    currentTransactions = transactions || [];
    renderQuotas(currentQuotas);
    renderCategories(currentCategories);
    renderTransactions(currentTransactions);
    renderLoanLedger(loanEntries || []);
    renderInterQuotaLoanLedger(interQuotaLoanEntries || []);
    renderDashboard(quotaTotals, categoryTotalsData);
    refreshQuotasAndCategories();
  } catch (err) {
    showToast(err.message || 'Failed to refresh data', 'error');
  }
}

if (txDate && !txDate.value) txDate.value = formatDate(new Date());
populateSelects();
updateTransactionFormFields();
updateQuotaFormFields();
refreshData();
EOF