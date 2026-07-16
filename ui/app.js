const views = document.querySelectorAll('.view');
const navButtons = document.querySelectorAll('nav button');
const quotaMode = document.getElementById('quota-mode');
const txType = document.getElementById('tx-type');
const txSource = document.getElementById('tx-source');
const txDestination = document.getElementById('tx-destination');
const txCategory = document.getElementById('tx-category');
const txPaymentMode = document.getElementById('tx-payment-mode');
const txPaymentInstrument = document.getElementById('tx-payment-instrument');
const txDate = document.getElementById('tx-date');
const quotaTable = document.querySelector('#quota-table tbody');
const categoryTable = document.querySelector('#category-table tbody');
const transactionTable = document.querySelector('#transaction-table tbody');
const quotaBreakdowns = document.getElementById('quota-breakdowns');
const categoryTotals = document.getElementById('category-totals');
const loanLedger = document.getElementById('loan-ledger');
const sourceCategory = document.getElementById('source-category');
const destCategory = document.getElementById('dest-category');
const quotaEomDestination = document.getElementById('quota-eom-destination');

const transactionForm = document.getElementById('transaction-form');
const quotaForm = document.getElementById('quota-form');
const categoryForm = document.getElementById('category-form');
const recategorizeForm = document.getElementById('recategorize-form');
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
const txSubmit = document.getElementById('tx-submit');
const txCancel = document.getElementById('tx-cancel');
const quotaSubmit = document.getElementById('quota-submit');
const quotaCancel = document.getElementById('quota-cancel');
const txHeading = document.getElementById('transaction-form-heading');
const quotaHeading = document.getElementById('quota-form-heading');
const confirmModal = document.getElementById('confirm-modal');
const confirmTitle = document.getElementById('confirm-title');
const confirmMessage = document.getElementById('confirm-message');
const confirmDetails = document.getElementById('confirm-details');
const confirmYes = document.getElementById('confirm-yes');
const confirmNo = document.getElementById('confirm-no');
let confirmCallback = null;

function openConfirm(title, message, details, onConfirm) {
  confirmTitle.textContent = title || 'Confirm';
  confirmMessage.textContent = message || '';
  confirmDetails.textContent = details || '';
  confirmCallback = onConfirm;
  confirmModal.classList.remove('hidden');
}

function closeConfirm() {
  confirmCallback = null;
  confirmModal.classList.add('hidden');
}

confirmYes.addEventListener('click', () => {
  if (typeof confirmCallback === 'function') {
    try { confirmCallback(); } catch (e) { console.error(e); }
  }
  closeConfirm();
});
confirmNo.addEventListener('click', () => closeConfirm());

const transactionTypes = [
  'Debit',
  'Credit',
  'Salary',
  'Loan Received',
  'Self Transfer',
  'Inter-Quota Loan',
];
const quotaModeOptions = ['Both', 'Monthly Only', 'Global Only'];
const paymentModes = ['Self', 'On Behalf of Other'];
const paymentInstruments = ['Cash', 'UPI', 'Card', 'Bank Transfer', 'Cheque'];

function setActiveView(viewId) {
  views.forEach((view) => view.classList.toggle('active', view.id === viewId));
  navButtons.forEach((button) => button.classList.toggle('active', button.dataset.view === viewId));
}

navButtons.forEach((button) => {
  button.addEventListener('click', () => setActiveView(button.dataset.view));
});

function formatDate(date) {
  const d = new Date(date);
  return d.toISOString().split('T')[0];
}

function buildSelectOptions(select, values) {
  select.innerHTML = values.map((value) => `<option value="${value}">${value}</option>`).join('');
}

function updateTransactionFormFields() {
  const type = txType.value;
  const categoryField = document.querySelector('label[for="tx-category"]').parentElement;
  const destinationField = document.querySelector('label[for="tx-destination"]').parentElement;
  categoryField.style.display = type === 'Debit' ? '' : 'none';
  destinationField.style.display = type === 'Self Transfer' || type === 'Inter-Quota Loan' ? '' : 'none';
}

function updateQuotaFormFields() {
  const mode = quotaMode.value;
  const monthlyField = document.getElementById('quota-monthly-allocation').parentElement;
  const targetField = document.getElementById('quota-target-amount').parentElement;
  const eomField = document.getElementById('quota-eom-destination').parentElement;
  if (mode === 'Monthly Only') {
    monthlyField.style.display = '';
    eomField.style.display = '';
    targetField.style.display = 'none';
  } else if (mode === 'Global Only') {
    monthlyField.style.display = 'none';
    eomField.style.display = 'none';
    targetField.style.display = '';
  } else { // Both (creation only)
    monthlyField.style.display = '';
    eomField.style.display = '';
    targetField.style.display = '';
  }
}

function fetchJSON(url, options = {}) {
  return fetch(url, options).then(async (res) => {
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_) {
      body = text;
    }
    if (!res.ok) {
      const err = new Error((body && body.error) || text || res.statusText);
      err.status = res.status;
      err.body = body;
      throw err;
    }
    return body;
  });
}

function loadQuotas() {
  return fetchJSON('/api/quotas');
}

function loadCategories() {
  return fetchJSON('/api/categories');
}

function loadTransactions() {
  return fetchJSON('/api/transactions');
}

function loadLoanLedger() {
  return fetchJSON('/api/loan-ledger');
}

function renderQuotaTable(quotas) {
  quotaTable.innerHTML = quotas.map((quota) => `
    <tr>
      <td>${quota.name}</td>
      <td>${quota.scope}</td>
      <td>${quota.monthly_allocation || '-'}</td>
      <td>${quota.target_amount || '-'}</td>
      <td>${quota.eom_sweep_destination || '-'}</td>
      <td>${quota.archived ? 'Yes' : 'No'}</td>
      <td>
        <button data-id="${quota.id}" class="edit-quota">Edit</button>
        <button data-id="${quota.id}" class="archive-quota">Archive</button>
      </td>
    </tr>
  `).join('');
}

function renderCategoryTable(categories) {
  categoryTable.innerHTML = categories.map((category) => `
    <tr>
      <td>${category.name}</td>
      <td>
        <button data-id="${category.id}" class="delete-category">Delete</button>
      </td>
    </tr>
  `).join('');
}

function renderTransactionTable(transactions) {
  transactionTable.innerHTML = transactions.map((tx) => `
    <tr>
      <td>${formatDate(tx.date)}</td>
      <td>${tx.type}</td>
      <td>${tx.amount.toFixed(2)}</td>
      <td>${tx.source_quota_id || '-'}</td>
      <td>${tx.destination_quota_id || '-'}</td>
      <td>${tx.category || '-'}</td>
      <td>${tx.counterparty || '-'}</td>
      <td>${tx.details || '-'}</td>
      <td>
        <button data-id="${tx.id}" class="edit-transaction">Edit</button>
        <button data-id="${tx.id}" class="delete-transaction">Delete</button>
      </td>
    </tr>
  `).join('');
}

function findQuotaName(quotas, id) {
  const quota = quotas.find((q) => q.id === id);
  return quota ? quota.name : id;
}

function renderDashboard(monthly, globalTotals, quotas) {
  quotaBreakdowns.innerHTML = `
    <div>
      <h4>Monthly Quotas</h4>
      <ul>${monthly.map((q) => `<li>${findQuotaName(quotas, q.quota_id)}: spent ${q.debited.toFixed(2)}, available ${q.available_balance.toFixed(2)}</li>`).join('')}</ul>
    </div>
    <div>
      <h4>Global Quotas</h4>
      <ul>${globalTotals.map((q) => `<li>${findQuotaName(quotas, q.quota_id)}: accumulated ${q.accumulated.toFixed(2)}</li>`).join('')}</ul>
    </div>
  `;
}

function renderCategoryTotals(monthly, globalTotals) {
  categoryTotals.innerHTML = `
    <div>
      <h4>Monthly Totals</h4>
      <ul>${monthly.map((c) => `<li>${c.category_name}: ${c.total.toFixed(2)}</li>`).join('')}</ul>
    </div>
    <div>
      <h4>Global Totals</h4>
      <ul>${globalTotals.map((c) => `<li>${c.category_name}: ${c.total.toFixed(2)}</li>`).join('')}</ul>
    </div>
  `;
}

function renderLoanLedger(entries) {
  loanLedger.innerHTML = `
    <ul>${entries.map((entry) => `<li>${entry.counterparty}: outstanding ${entry.outstanding_external_loan.toFixed(2)}</li>`).join('')}</ul>
  `;
}

function showFundingOptions(errorBody) {
  const options = errorBody?.options || [];
  if (!options.length) {
    fundingMessage.textContent = 'No funding options are available for this shortfall.';
    fundingOptionsList.innerHTML = '';
  } else {
    fundingMessage.textContent = 'Choose a funding source to cover the shortfall and retry:';
    fundingOptionsList.innerHTML = options.map((option, index) => `
      <li>
        <strong>${option.source_quota_name}</strong> (${option.mechanism}) — available ${option.available_balance.toFixed(2)}
        <button type="button" data-index="${index}" data-mechanism="${option.mechanism}" data-source-quota-id="${option.source_quota_id}" class="fund-option">Use this</button>
      </li>
    `).join('');
  }
  fundingPanel.classList.remove('hidden');
}

function hideFundingOptions() {
  fundingPanel.classList.add('hidden');
  fundingMessage.textContent = '';
  fundingOptionsList.innerHTML = '';
}

let pendingTransaction = null;
let editingQuotaId = null;
let editingTransactionId = null;
let currentQuotas = [];
let currentCategories = [];

function populateSelects(quotas, categories) {
  txSource.innerHTML = `<option value="">-</option>${quotas.map((q) => `<option value="${q.id}">${q.name}</option>`).join('')}`;
  txDestination.innerHTML = `<option value="">-</option>${quotas.map((q) => `<option value="${q.id}">${q.name}</option>`).join('')}`;
  quotaEomDestination.innerHTML = `<option value="">-</option>${quotas.filter((q) => q.scope === 'Global Only').map((q) => `<option value="${q.id}">${q.name}</option>`).join('')}`;
  txCategory.innerHTML = `<option value="">-</option>${categories.map((c) => `<option value="${c.name}">${c.name}</option>`).join('')}`;
  sourceCategory.innerHTML = `<option value="">-</option>${categories.map((c) => `<option value="${c.id}">${c.name}</option>`).join('')}`;
  destCategory.innerHTML = `<option value="">-</option>${categories.map((c) => `<option value="${c.id}">${c.name}</option>`).join('')}`;
  currentQuotas = quotas;
  currentCategories = categories;
}

quotaMode.addEventListener('change', updateQuotaFormFields);

function refreshData() {
  hideFundingOptions();
  return Promise.all([loadQuotas(), loadCategories(), loadTransactions()])
    .then(([quotas, categories, transactions]) => {
      renderQuotaTable(quotas);
      renderCategoryTable(categories);
      renderTransactionTable(transactions);
      populateSelects(quotas, categories);
      return Promise.all([
        Promise.resolve(quotas),
        fetchJSON('/api/quotas/breakdowns?scope=all'),
        fetchJSON('/api/categories/totals?scope=all'),
        loadLoanLedger(),
      ]).then(([resolvedQuotas, quotaData, categoryData, ledgerData]) => {
        renderDashboard(quotaData.monthly, quotaData.global, resolvedQuotas);
        renderCategoryTotals(categoryData.monthly, categoryData.global);
        renderLoanLedger(ledgerData);
      });
    })
    .catch((err) => console.error('refresh error', err));
}

function openEditQuota(id) {
  const q = currentQuotas.find((qq) => qq.id === id);
  if (!q) return;
  editingQuotaId = id;
  quotaMode.value = q.scope || 'Monthly Only';
  document.getElementById('quota-name').value = q.name;
  document.getElementById('quota-monthly-allocation').value = q.monthly_allocation || '';
  document.getElementById('quota-target-amount').value = q.target_amount || '';
  document.getElementById('quota-eom-destination').value = q.eom_sweep_destination || '';
  quotaHeading.textContent = 'Edit Quota';
  quotaSubmit.textContent = 'Save Changes';
  // Lock scope when editing since backend does not support scope change via PUT
  quotaMode.disabled = true;
  const note = document.getElementById('quota-mode-note');
  note.textContent = 'Scope cannot be changed for existing quotas. Create a new quota if you need a different scope.';
  updateQuotaFormFields();
}

function clearQuotaEditState() {
  editingQuotaId = null;
  quotaForm.reset();
  quotaHeading.textContent = 'Create Quota';
  quotaSubmit.textContent = 'Create Quota';
  quotaMode.disabled = false;
  document.getElementById('quota-mode-note').textContent = '';
  updateQuotaFormFields();
}

function openEditTransaction(id) {
  const txs = document.querySelectorAll('#transaction-table tbody tr');
  // Find transaction from last-loaded transactions by fetching list again
  loadTransactions().then((transactions) => {
    const tx = transactions.find((t) => t.id === id);
    if (!tx) return;
    editingTransactionId = id;
    txType.value = tx.type;
    document.getElementById('tx-amount').value = tx.amount;
    txSource.value = tx.source_quota_id || '';
    txDestination.value = tx.destination_quota_id || '';
    txCategory.value = tx.category || '';
    document.getElementById('tx-counterparty').value = tx.counterparty || '';
    txPaymentMode.value = tx.payment_mode || '';
    txPaymentInstrument.value = tx.payment_instrument || '';
    txDate.value = new Date(tx.date).toISOString().split('T')[0];
    document.getElementById('tx-details').value = tx.details || '';
    updateTransactionFormFields();
    txHeading.textContent = 'Edit Transaction';
    txSubmit.textContent = 'Save Changes';
  });
}

function clearTransactionEditState() {
  editingTransactionId = null;
  transactionForm.reset();
  updateTransactionFormFields();
  txHeading.textContent = 'Create Transaction';
  txSubmit.textContent = 'Save Transaction';
}

txCancel.addEventListener('click', () => {
  clearTransactionEditState();
});

quotaCancel.addEventListener('click', () => {
  clearQuotaEditState();
});

transactionForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const formData = new FormData(transactionForm);
  const payload = {
    type: formData.get('type'),
    amount: parseFloat(formData.get('amount')),
    source_quota_id: formData.get('source_quota_id') || undefined,
    destination_quota_id: formData.get('destination_quota_id') || undefined,
    category: formData.get('category') || undefined,
    counterparty: formData.get('counterparty') || undefined,
    payment_mode: formData.get('payment_mode') || undefined,
    payment_instrument: formData.get('payment_instrument') || undefined,
    date: formData.get('date'),
    details: formData.get('details') || undefined,
  };

  pendingTransaction = payload;
  const method = editingTransactionId ? 'PUT' : 'POST';
  const url = editingTransactionId ? `/api/transactions/${editingTransactionId}` : '/api/transactions';
  fetchJSON(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      pendingTransaction = null;
      hideFundingOptions();
      clearTransactionEditState();
      refreshData();
    })
    .catch((err) => {
      if (err.status === 422 && err.body && err.body.options) {
        showFundingOptions(err.body);
      } else {
        showToast(err.message || 'Request failed', 'error');
      }
    });
});

quotaForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const formData = new FormData(quotaForm);
  const payload = {
    mode: formData.get('mode'),
    name: formData.get('name'),
    monthly_allocation: parseFloat(formData.get('monthly_allocation')) || 0,
    target_amount: parseFloat(formData.get('target_amount')) || 0,
    eom_sweep_destination: formData.get('eom_sweep_destination') || undefined,
  };

  // Client-side validation consistent with backend rules
  const mode = formData.get('mode');
  if (editingQuotaId) {
    // scope is locked during edit; determine actual scope from quotaMode
    const scope = quotaMode.value;
    if (scope === 'Monthly Only' && (!payload.monthly_allocation || payload.monthly_allocation <= 0)) {
      setFormError('quota-error', 'Monthly allocation is required and must be > 0 for Monthly quotas');
      return;
    }
    if (scope === 'Global Only' && payload.monthly_allocation && payload.monthly_allocation !== 0) {
      setFormError('quota-error', 'Monthly allocation is not applicable to Global Only quotas');
      return;
    }
    // submit PUT
    fetchJSON(`/api/quotas/${editingQuotaId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: payload.name,
        monthly_allocation: payload.monthly_allocation,
        target_amount: payload.target_amount,
        eom_sweep_destination: payload.eom_sweep_destination,
      }),
    })
      .then(() => {
        clearQuotaEditState();
        refreshData();
      })
      .catch((err) => showToast(err.message || 'Request failed', 'error'));
  } else {
    // creation path: validate according to chosen mode
    if (mode === 'Monthly Only' && (!payload.monthly_allocation || payload.monthly_allocation <= 0)) {
      setFormError('quota-error', 'Monthly allocation is required and must be > 0 for Monthly quotas');
      return;
    }
    fetchJSON('/api/quotas', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
      .then(() => refreshData())
      .catch((err) => showToast(err.message || 'Request failed', 'error'));
  }
});

categoryForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const formData = new FormData(categoryForm);
  fetchJSON('/api/categories', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: formData.get('name') }),
  })
    .then(() => refreshData())
    .catch((err) => showToast(err.message || 'Request failed', 'error'));
});

recategorizeForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const formData = new FormData(recategorizeForm);
  fetchJSON('/api/categories/recategorize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      source_category_id: formData.get('source_category_id'),
      dest_category_id: formData.get('dest_category_id'),
    }),
  })
    .then(() => refreshData())
    .catch((err) => showToast(err.message || 'Request failed', 'error'));
});

fundingOptionsList.addEventListener('click', (event) => {
  const button = event.target.closest('.fund-option');
  if (!button) {
    return;
  }
  if (!pendingTransaction) {
    showToast('No pending transaction to remediate. Please resubmit the transaction.', 'error');
    return;
  }
  const selected = {
    mechanism: button.dataset.mechanism,
    source_quota_id: button.dataset.sourceQuotaId,
  };
  submitFundingRemediation(selected);
});

fundingCancel.addEventListener('click', hideFundingOptions);

function submitFundingRemediation(option) {
  const payload = {
    mechanism: option.mechanism,
    source_quota_id: option.source_quota_id,
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
    })
    .catch((err) => showToast(err.message || 'Request failed', 'error'));
});

const monthStartButton = document.getElementById('run-month-start');
const monthEndButton = document.getElementById('run-month-end');
const systemStatus = document.getElementById('system-status');

function setSystemStatus(message, isError = false) {
  systemStatus.textContent = message;
  systemStatus.style.color = isError ? '#dc2626' : '#111827';
}

// Toasts and inline form errors
const toasts = document.getElementById('toasts');
function showToast(message, type = 'success', ms = 4000) {
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = message;
  toasts.appendChild(el);
  setTimeout(() => {
    el.remove();
  }, ms);
}

function setFormError(id, msg) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = msg || '';
}

function openLoanSettlePanel(counterparty) {
  settleCounterparty.textContent = counterparty;
  // populate source quotas
  settleSource.innerHTML = currentQuotas.map((q) => `<option value="${q.id}">${q.name} (${q.scope})</option>`).join('');
  settleAmount.value = '';
  loanSettlePanel.classList.remove('hidden');
}

function hideLoanSettlePanel() {
  loanSettlePanel.classList.add('hidden');
}

settleCancel.addEventListener('click', hideLoanSettlePanel);

settleSend.addEventListener('click', () => {
  const amount = parseFloat(settleAmount.value);
  const source = settleSource.value;
  const counterparty = settleCounterparty.textContent;
  if (!amount || amount <= 0) return showToast('Enter a positive amount', 'error');
  if (!source) return showToast('Select a source quota', 'error');
  const payload = {
    type: 'Debit',
    amount,
    source_quota_id: source,
    category: '',
    counterparty,
    date: new Date().toISOString(),
    details: 'settle external loan',
  };
  fetchJSON('/api/transactions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then(() => {
      hideLoanSettlePanel();
      refreshData();
    })
    .catch((err) => showToast(err.message || 'Request failed', 'error'));
});

monthStartButton.addEventListener('click', () => {
  setSystemStatus('Running month-start...', false);
  fetchJSON('/api/schedule/start', { method: 'POST' })
    .then((result) => {
      setSystemStatus('Month start completed.');
      refreshData();
    })
    .catch((err) => {
      setSystemStatus(`Month start failed: ${err.message}`, true);
    });
});

monthEndButton.addEventListener('click', () => {
  setSystemStatus('Running month-end sweep...', false);
  fetchJSON('/api/schedule/end', { method: 'POST' })
    .then(() => {
      setSystemStatus('Month-end sweep completed.');
      refreshData();
    })
    .catch((err) => {
      setSystemStatus(`Month-end failed: ${err.message}`, true);
    });
});

document.addEventListener('click', (event) => {
  const target = event.target;
  if (target.classList.contains('delete-category')) {
    const id = target.dataset.id;
    fetchJSON(`/api/categories/${id}`, { method: 'DELETE' })
      .then(() => refreshData())
      .catch((err) => showToast(err.message || 'Request failed', 'error'));
  }
  // Quota actions
  if (target.classList.contains('edit-quota')) {
    const id = target.dataset.id;
    openEditQuota(id);
  }
  if (target.classList.contains('archive-quota')) {
    const id = target.dataset.id;
    openConfirm('Archive quota', 'Archive this quota? This cannot be undone via the UI.', '', () => {
      fetchJSON(`/api/quotas/${id}`, { method: 'DELETE' })
        .then(() => refreshData())
        .catch((err) => showToast(err.message || 'Request failed', 'error'));
    });
  }
  // Transaction actions
  if (target.classList.contains('edit-transaction')) {
    const id = target.dataset.id;
    openEditTransaction(id);
  }
  if (target.classList.contains('delete-transaction')) {
    const id = target.dataset.id;
    openConfirm('Delete transaction', 'Delete this transaction? This action cannot be undone.', '', () => {
      fetchJSON(`/api/transactions/${id}`, { method: 'DELETE' })
        .then(() => refreshData())
        .catch((err) => showToast(err.message || 'Request failed', 'error'));
    });
  }
  // Loan settle opener (from loan ledger)
  if (target.classList.contains('settle-loan')) {
    const counterparty = target.dataset.counterparty;
    openLoanSettlePanel(counterparty);
  }
});

buildSelectOptions(txType, transactionTypes);
buildSelectOptions(quotaMode, quotaModeOptions);
buildSelectOptions(txPaymentMode, paymentModes);
buildSelectOptions(txPaymentInstrument, paymentInstruments);
updateTransactionFormFields();
txType.addEventListener('change', updateTransactionFormFields);

const today = new Date().toISOString().split('T')[0];
txDate.value = today;

refreshData();
