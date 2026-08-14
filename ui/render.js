// render.js — UI rendering and HTML templates

import { escapeHTML, toDateOnly, formatDate } from './utils.js';
import {
  walletTableBody, transactionTableBody, categoryTableBody,
  loanLedger, interWalletLoanLedger, walletBreakdowns, categoryTotals,
  taskList, todayTasksEl, txDestination , loanLedger_settle, interWalletLoanLedger_settle
} from './dom.js';

export function buildSelectOptions(select, values, includeEmpty = true) {
  if (!select) return;
  const items = includeEmpty ? [{ value: '', label: '-- Select --' }, ...values] : values;
  select.innerHTML = items.map((value) => {
    if (typeof value === 'object') {
      return `<option value="${value.value}">${value.label}</option>`;
    }
    return `<option value="${value}">${value}</option>`;
  }).join('');
}

// Generates the HTML Badge for the wallet (visual output)
export function walletLabelHTML(walletId, wallets) {
  if (!walletId) return '';
  const q = wallets.find((w) => String(w.id) === String(walletId));
  if (q) {
    const scopeClass = q.scope === 'Monthly' ? 'monthly' : (q.scope === 'Global' ? 'global' : 'accumulated');
    return `${escapeHTML(q.name)} <span class="stat-badge ${scopeClass}">${escapeHTML(q.scope)}</span>`;
  }
  return '<span style="color:#dc2626;font-weight:600;">DELETED</span>';
}

export function renderTransactions(transactions, wallets) {
  if (!transactionTableBody) return;
  transactionTableBody.innerHTML = (transactions || []).map((t) => {
    let typeClass = 'accumulated'; 
    if (t.type === 'Debit') typeClass = 'spent';
    else if (t.type === 'Credit' || t.type === 'Salary') typeClass = 'available';
    else if (t.type === 'Inter-Wallet Loan') typeClass = 'iw-loan';
    else if (t.type === 'Loan Received') typeClass = 'loan-received';
    
    return `
    <tr>
      <td>${t.date || ''}</td>
      <td><span class="stat-badge ${typeClass}">${t.type || ''}</span></td>
      <td class="font-mono">${typeof t.amount === 'number' ? t.amount.toFixed(2) : t.amount || ''}</td>
      <td>${walletLabelHTML(t.source_wallet_id, wallets)}</td>
      <td>${walletLabelHTML(t.destination_wallet_id, wallets)}</td>
      <td>${escapeHTML(t.category || '')}</td>
      <td>${escapeHTML(t.counterparty || '')}</td>
      <td>${escapeHTML(t.details || '')}</td>
      <td class="actions-cell">
        <button class="btn-small edit-transaction" data-id="${t.id}">Edit</button>
        <button class="btn-small danger-btn delete-transaction" data-id="${t.id}" style="margin-left: 0.5rem">Delete</button>
      </td>
    </tr>
  `}).join('');
}

export function renderWallets(wallets) {
  if (!walletTableBody) return;
  walletTableBody.innerHTML = (wallets || []).map((q) => {
    const scopeClass = q.scope === 'Monthly' ? 'monthly' : (q.scope === 'Global' ? 'global' : 'accumulated');
    return `
    <tr>
      <td><strong>${escapeHTML(q.name)}</strong></td>
      <td><span class="stat-badge ${scopeClass}">${escapeHTML(q.scope || '')}</span></td>
      <td class="font-mono">${q.target_amount || ''}</td>
      <td>${walletLabelHTML(q.eom_sweep_destination, wallets)}</td>
      <td class="actions-cell">
        <button class="btn-small edit-wallet" data-id="${q.id}">Edit</button>
        ${q.id === 'savings' ? '' : `<button class="btn-small danger-btn delete-wallet" data-id="${q.id}" style="margin-left: 0.5rem">Delete</button>`}
      </td>
    </tr>
  `}).join('');
}

export function renderSortIndicators(txConfig, walletConfig) {
  document.querySelectorAll('#transaction-table th.sortable').forEach(th => {
    th.classList.remove('sort-asc', 'sort-desc');
    if (th.dataset.sort === txConfig.key) th.classList.add(txConfig.dir === 'asc' ? 'sort-asc' : 'sort-desc');
  });
  document.querySelectorAll('#wallet-table th.sortable').forEach(th => {
    th.classList.remove('sort-asc', 'sort-desc');
    if (th.dataset.sort === walletConfig.key) th.classList.add(walletConfig.dir === 'asc' ? 'sort-asc' : 'sort-desc');
  });
}

export function renderCategories(categories) {
  if (!categoryTableBody) return;
  categoryTableBody.innerHTML = (categories || []).map((c) => `
    <tr>
      <td><strong>${escapeHTML(c.name)}</strong></td>
      <td class="actions-cell">
        <button class="btn-small danger-btn delete-category" data-id="${c.id}">Delete</button>
      </td>
    </tr>
  `).join('');
}

export function renderLoanLedger(entries) {
  if (!loanLedger) return;
  loanLedger.innerHTML = `<ul class="modern-list">` + (entries || []).map((entry) => `
    <li class="modern-list-item">
      <div class="item-info">
        <span class="item-name">${escapeHTML(entry.counterparty)}</span>
        <span class="item-value negative font-mono">Outstanding: ${Number(entry.outstanding_external_loan || 0).toFixed(2)}</span>
      </div>
    </li>
  `).join('') + `</ul>`;
}

// Helpers for the ledger to output HTML visually
const getDispHTML = (id, name, wallets) => {
   const w = wallets.find(x => String(x.id) === String(id));
   if (w) {
     const scopeClass = w.scope === 'Monthly' ? 'monthly' : (w.scope === 'Global' ? 'global' : 'accumulated');
     return `${escapeHTML(name)} <span class="stat-badge ${scopeClass}">${escapeHTML(w.scope)}</span>`;
   }
   return escapeHTML(name);
};

export function renderInterWalletLoanLedger(entries, wallets = []) {
  if (!interWalletLoanLedger) return;
  interWalletLoanLedger.innerHTML = `<ul class="modern-list">` + (entries || []).map((entry) => `
    <li class="modern-list-item">
      <div class="item-info">
        <span class="item-name">${getDispHTML(entry.lender_wallet_id, entry.lender_wallet_name, wallets)} &rarr; ${getDispHTML(entry.borrower_wallet_id, entry.borrower_wallet_name, wallets)}</span>
        <span class="item-value negative font-mono">Outstanding: ${Number(entry.outstanding || 0).toFixed(2)}</span>
      </div>
    </li>
  `).join('') + `</ul>`;
}

export function renderLoanLedger_settle(entries) {
  if (!loanLedger_settle) return;
  loanLedger_settle.innerHTML = `<ul class="modern-list">` + (entries || []).map((entry) => `
    <li class="modern-list-item flex-between">
      <div class="item-info">
        <span class="item-name">${escapeHTML(entry.counterparty)}</span>
        <span class="item-value negative font-mono">Outstanding: ${Number(entry.outstanding_external_loan || 0).toFixed(2)}</span>
      </div>
      <button class="settle-loan btn-small" data-counterparty="${escapeHTML(entry.counterparty)}">Settle</button>
    </li>
  `).join('') + `</ul>`;
}

export function renderInterWalletLoanLedger_settle(entries, wallets = []) {
  if (!interWalletLoanLedger_settle) return;
  
  interWalletLoanLedger_settle.innerHTML = `<ul class="modern-list">` + (entries || []).map((entry) => {
    const lenderHTML = getDispHTML(entry.lender_wallet_id, entry.lender_wallet_name, wallets);
    const borrowerHTML = getDispHTML(entry.borrower_wallet_id, entry.borrower_wallet_name, wallets);
    
    return `
    <li class="modern-list-item flex-between">
      <div class="item-info">
        <span class="item-name">${lenderHTML} &rarr; ${borrowerHTML}</span>
        <span class="item-value negative font-mono">Outstanding: ${Number(entry.outstanding || 0).toFixed(2)}</span>
      </div>
      <button class="settle-interwallet-loan btn-small"
        data-lender-id="${escapeHTML(entry.lender_wallet_id)}"
        data-lender-name="${escapeHTML(entry.lender_wallet_name)}"
        data-borrower-id="${escapeHTML(entry.borrower_wallet_id)}"
        data-borrower-name="${escapeHTML(entry.borrower_wallet_name)}">Settle</button>
    </li>
  `}).join('') + `</ul>`;
}

export function renderDashboard(walletData, categoryData, wallets = []) {
  if (walletBreakdowns && walletData) {
    walletBreakdowns.innerHTML = `
      <div class="dashboard-section">
        <h4 class="section-title">Monthly Wallets</h4>
        <ul class="modern-list">${(walletData.monthly || []).map((q) => `
          <li class="modern-list-item">
            <div class="item-info">
              <span class="item-name">${escapeHTML(q.wallet_name || q.wallet_id)}</span>
              <div class="item-stats">
                <span class="stat-badge spent font-mono">Spent: ${Number(q.debited).toFixed(2)}</span>
                <span class="stat-badge available font-mono">Avail: ${Number(q.available_balance).toFixed(2)}</span>
              </div>
            </div>
          </li>`).join('')}
        </ul>
      </div>
      <div class="dashboard-section mt-3">
        <h4 class="section-title">Global Wallets</h4>
        <ul class="modern-list">${(walletData.global || []).map((q) => {
          const w = wallets.find(x => String(x.id) === String(q.wallet_id));
          let compBadge = '';
          if (w && Number(w.target_amount) > 0) {
            const comp = (Number(q.accumulated) / Number(w.target_amount)) * 100;
            compBadge = `<span class="stat-badge completion font-mono">Completion : ${comp.toFixed(1)}%</span>`;
          }
          return `
          <li class="modern-list-item">
            <div class="item-info">
              <span class="item-name">${escapeHTML(q.wallet_name || q.wallet_id)}</span>
              <div class="item-stats">
                <span class="stat-badge accumulated font-mono">Accumulated: ${Number(q.accumulated).toFixed(2)}</span>
                ${compBadge}
              </div>
            </div>
          </li>`
        }).join('')}
        </ul>
      </div>
    `;
  }
  if (categoryTotals && categoryData) {
    categoryTotals.innerHTML = `
      <div class="dashboard-section">
        <h4 class="section-title">Monthly Totals</h4>
        <ul class="modern-list">${(categoryData.monthly || []).map((c) => `
          <li class="modern-list-item flex-between">
            <span class="item-name">${escapeHTML(c.category_name)}</span>
            <span class="item-value font-mono">${Number(c.total).toFixed(2)}</span>
          </li>`).join('')}
        </ul>
      </div>
      <div class="dashboard-section mt-3">
        <h4 class="section-title">Global Totals</h4>
        <ul class="modern-list">${(categoryData.global || []).map((c) => `
          <li class="modern-list-item flex-between">
            <span class="item-name">${escapeHTML(c.category_name)}</span>
            <span class="item-value font-mono">${Number(c.total).toFixed(2)}</span>
          </li>`).join('')}
        </ul>
      </div>
    `;
  }
}

export function renderTasks(tasks, editingTaskId) {
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

export function renderTodayTasks(tasks) {
  if (!todayTasksEl) return;
  const today = formatDate(new Date());
  const todays = (tasks || []).filter((t) => toDateOnly(t.due_date) === today);
  if (!todays.length) {
    todayTasksEl.innerHTML = '<p class="note" style="margin-top:0;">No tasks due today.</p>';
    return;
  }
  todayTasksEl.innerHTML = `<ul class="today-task-list">${todays.map((t) => `<li><strong>${escapeHTML(t.title || '')}</strong></li>`).join('')}</ul>`;
}

export function showTasksConnecting() {
  if (taskList) {
    taskList.innerHTML = '<li class="task-empty task-connecting">Connecting to task server… this can take up to a minute if it has been idle.</li>';
  }
  if (todayTasksEl) {
    todayTasksEl.innerHTML = '<p class="note task-connecting" style="margin-top:0;">Connecting to task server…</p>';
  }
}

export function populateDestinationOptions(wallets) {
  if (!txDestination) return;
  const previousValue = txDestination.value;
  buildSelectOptions(txDestination, wallets.map((q) => ({ value: q.id, label: `${q.name} (${q.scope})` })));
  if (wallets.some((q) => q.id === previousValue)) {
    txDestination.value = previousValue;
  }
}