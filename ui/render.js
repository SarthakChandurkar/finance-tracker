// render.js — UI rendering and HTML templates

import { escapeHTML, toDateOnly, formatDate } from './utils.js';
import {
  walletTableBody, transactionTableBody, categoryTableBody,
  loanLedger, interWalletLoanLedger, walletBreakdowns, categoryTotals,
  taskList, todayTasksEl, txDestination
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

export function walletLabelHTML(walletId, wallets) {
  if (!walletId) return '';
  const q = wallets.find((w) => String(w.id) === String(walletId));
  if (q) return q.name;
  return '<span style="color:#dc2626;font-weight:600;">DELETED</span>';
}

export function renderTransactions(transactions, wallets) {
  if (!transactionTableBody) return;
  transactionTableBody.innerHTML = (transactions || []).map((t) => `
    <tr>
      <td>${t.date || ''}</td>
      <td>${t.type || ''}</td>
      <td>${typeof t.amount === 'number' ? t.amount.toFixed(2) : t.amount || ''}</td>
      <td>${walletLabelHTML(t.source_wallet_id, wallets)}</td>
      <td>${walletLabelHTML(t.destination_wallet_id, wallets)}</td>
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

export function renderWallets(wallets) {
  if (!walletTableBody) return;
  walletTableBody.innerHTML = (wallets || []).map((q) => `
    <tr>
      <td>${q.name}</td>
      <td>${q.scope || ''}</td>
      <td></td>
      <td>${q.target_amount || ''}</td>
      <td>${walletLabelHTML(q.eom_sweep_destination, wallets)}</td>
      <td>
        <button class="edit-wallet" data-id="${q.id}">Edit</button>
        ${q.id === 'savings' ? '' : `<button class="delete-wallet" data-id="${q.id}">Delete</button>`}
      </td>
    </tr>
  `).join('');
}

export function renderCategories(categories) {
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

export function renderLoanLedger(entries) {
  if (!loanLedger) return;
  loanLedger.innerHTML = (entries || []).map((entry) => `
    <div class="loan-entry">
      <span>${entry.counterparty}: outstanding ${Number(entry.outstanding_external_loan || 0).toFixed(2)}</span>
      <button class="settle-loan" data-counterparty="${entry.counterparty}">Settle</button>
    </div>
  `).join('');
}

export function renderInterWalletLoanLedger(entries) {
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

export function renderDashboard(walletData, categoryData) {
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
    todayTasksEl.innerHTML = '<p class="note">No tasks due today.</p>';
    return;
  }
  todayTasksEl.innerHTML = `<ul class="today-task-list">${todays.map((t) => `<li>${escapeHTML(t.title || '')}</li>`).join('')}</ul>`;
}

export function showTasksConnecting() {
  if (taskList) {
    taskList.innerHTML = '<li class="task-empty task-connecting">Connecting to task server… this can take up to a minute if it has been idle.</li>';
  }
  if (todayTasksEl) {
    todayTasksEl.innerHTML = '<p class="note task-connecting">Connecting to task server…</p>';
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