// dom.js — All HTML element references

export const views = document.querySelectorAll('.view');
export const navButtons = document.querySelectorAll('nav button');
export const navMenu = document.querySelector('nav');
export const hamburgerToggle = document.getElementById('hamburger-toggle');

// Profile Menu
export const profileToggle = document.getElementById('profile-toggle');
export const profileDropdown = document.getElementById('profile-dropdown');
export const profileUsername = document.getElementById('profile-username');
export const profileLogout = document.getElementById('profile-logout');

// Transaction Form Elements
export const txType = document.getElementById('tx-type');
export const txSource = document.getElementById('tx-source');
export const txDestination = document.getElementById('tx-destination');
export const txCategory = document.getElementById('tx-category');
export const txPaymentMode = document.getElementById('tx-payment-mode');
export const txPaymentInstrument = document.getElementById('tx-payment-instrument');
export const txDate = document.getElementById('tx-date');
export const transactionForm = document.getElementById('transaction-form');
export const txSubmit = document.getElementById('tx-submit');
export const txCancel = document.getElementById('tx-cancel');

// Wallet Form Elements
export const walletMode = document.getElementById('wallet-mode');
export const walletForm = document.getElementById('wallet-form');
export const walletSubmit = document.getElementById('wallet-submit');
export const walletCancel = document.getElementById('wallet-cancel');
export const walletModeNote = document.getElementById('wallet-mode-note');

// Category & Recategorize Forms
export const categoryForm = document.getElementById('category-form');
export const recategorizeForm = document.getElementById('recategorize-form');
export const sourceCategorySelect = document.getElementById('source-category');
export const destCategorySelect = document.getElementById('dest-category');

// Tables
export const walletTableBody = document.querySelector('#wallet-table tbody');
export const transactionTableBody = document.querySelector('#transaction-table tbody');
export const categoryTableBody = document.querySelector('#category-table tbody');

// Dashboards & Ledgers
export const walletBreakdowns = document.getElementById('wallet-breakdowns');
export const categoryTotals = document.getElementById('category-totals');
export const loanLedger = document.getElementById('loan-ledger');
export const interWalletLoanLedger = document.getElementById('interwallet-loan-ledger');

// Loan Settle and Show Elements
export const loanLedger_settle = document.getElementById('loan-ledger-settle');
export const interWalletLoanLedger_settle = document.getElementById('interwallet-loan-ledger-settle');


// Funding & Settlement Panels
export const fundingPanel = document.getElementById('funding-panel');
export const fundingMessage = document.getElementById('funding-message');
export const fundingOptionsList = document.getElementById('funding-options-list');
export const fundingCancel = document.getElementById('funding-cancel');

export const loanSettlePanel = document.getElementById('loan-settle-panel');
export const settleCounterparty = document.getElementById('settle-counterparty');
export const settleAmount = document.getElementById('settle-amount');
export const settleSource = document.getElementById('settle-source');
export const settleSend = document.getElementById('settle-send');
export const settleCancel = document.getElementById('settle-cancel');

export const interWalletSettlePanel = document.getElementById('interwallet-settle-panel');
export const interWalletSettleLabel = document.getElementById('interwallet-settle-label');
export const interWalletSettleAmount = document.getElementById('interwallet-settle-amount');
export const interWalletSettleSend = document.getElementById('interwallet-settle-send');
export const interWalletSettleCancel = document.getElementById('interwallet-settle-cancel');

// System & Task Elements
export const systemStatus = document.getElementById('system-status');
export const monthEndButton = document.getElementById('run-month-end');

export const taskForm = document.getElementById('task-form');
export const taskTitleInput = document.getElementById('task-title');
export const taskDueInput = document.getElementById('task-due');
export const taskList = document.getElementById('task-list');
export const deleteAllTasksBtn = document.getElementById('delete-all-tasks');
export const todayTasksEl = document.getElementById('today-tasks');

// Modal Elements
export const confirmModal = document.getElementById('confirm-modal');
export const confirmTitle = document.getElementById('confirm-title');
export const confirmMessage = document.getElementById('confirm-message');
export const confirmDetails = document.getElementById('confirm-details');
export const confirmYes = document.getElementById('confirm-yes');
export const confirmNo = document.getElementById('confirm-no');