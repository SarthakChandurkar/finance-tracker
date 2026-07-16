# Personal Finance Tracker — Requirements Document (Release 1)

This document is split into two clearly separated parts:

- **Part A — Backend / Domain Requirements:** what the system must compute, store, and enforce. This is stack-independent — it doesn't care whether the frontend ends up being an MPA, htmx, or an SPA.
- **Part B — Frontend / UI Requirements:** how the above gets shown and interacted with on screen. This part *does* depend on the eventual frontend decision, but the requirements below describe the necessary behavior regardless of which technology implements it.

---
---

# PART A — Backend / Domain Requirements

## A1. Transaction Management

> **Single Schema Principle:** There is exactly one transaction schema in this app (defined below). Every other domain module — Quotas, Per-Counterparty Ledger, External Loans — is built entirely by filtering and aggregating this same transaction list. No module has its own separate data structure.

### A1.1 Core Transaction Types

Every transaction has a Type, and every Type belongs to one of two groups via a **Balance Effect** field (auto-derived from Type, not separately chosen):

| Balance Effect | Types included | What it means |
|---|---|---|
| **Balance-Changing** | Credit, Debit, Salary, Loan Received | Actually increases or decreases your total money |
| **Non-Balance-Changing** | Self Transfer, Inter-Quota Loan | Money moves between quotas only — total balance stays the same |

- **Debit** — money spent/paid out *(Balance-Changing)*. Deducted from a Monthly Only quota entity, or directly from a Global Only quota entity.
- **Credit** — generic money received (reimbursements, refunds, gifts, etc.) *(Balance-Changing)*
- **Salary** — regular income credit *(Balance-Changing)*
- **Loan Received** — money borrowed from a person/institution *(Balance-Changing)*
- **Self Transfer** — moving money between quota entities based on strict directional rules (see A2.5) *(Non-Balance-Changing)*
- **Inter-Quota Loan** — borrowing money between quota entities with an obligation to repay (see A2.6) *(Non-Balance-Changing)*

### A1.2 Transaction Fields & Validation Rules

Each transaction record captures the following. (Input widget/control for each field — dropdown, calendar picker, etc. — is specified in Part B, Section B1.)

| Field | Description | Required? | Default |
|---|---|---|---|
| Type | Debit / Credit / Salary / Loan Received / Self Transfer / Inter-Quota Loan | Required | Debit |
| Balance Effect | Balance-Changing / Non-Balance-Changing — auto-derived from Type | Automatic | — |
| Amount | Numeric value | Compulsory | — |
| Source Quota | The quota entity the money is drawn from | Compulsory for Debit, Self Transfer, and Inter-Quota Loan. Not required for inflows (Salary, Credit, Loan Received) | — |
| Destination Quota | The quota entity receiving the money | Required for Self Transfer / Inter-Quota Loan. Optional for inflows if directing them into a specific quota immediately | — |
| Category | Expenditure category — only applicable/required for Debit transactions | Compulsory for Debit only | Miscellaneous (Debit only) |
| Counterparty | For Debit: destination/payee. For Credit/Salary/Loan Received: source/payer | Optional | — |
| Payment Mode | Self / On behalf of others (see A3.1) | Optional | Self |
| Payment Instrument | Cash / UPI / Card / Bank Transfer / Cheque | Optional | — |
| Date | Transaction date | Required | Today's date |
| Details | Free-text notes about the transaction | Optional | — |

### A1.3 Editing & Deletion
- All transactions are editable after creation.
- Edited/deleted transactions automatically recalculate totals, Monthly balances, and Global accumulations — since everything downstream is derived, not separately stored.

### A1.4 Non-Negative Balance Rule (Global Validation)
- No balance may go below zero.
- **Debit Constraint:** Debits primarily happen from a Monthly Only quota entity. Direct Debits are also permitted from Global Only entities (e.g., spending from a completed Vacation Fund). If a Monthly or Global Only balance is 0, Debits are blocked unless funded via:
  - **Monthly shortfalls:** a Self Transfer (from a Global entity or another Monthly entity, per A2.5) or an Inter-Quota Loan (per A2.6)
  - **Global Only shortfalls:** a Self Transfer only — from another Global Only entity, or from another quota's Monthly entity (Reverse Cross-Quota Transfer, per A2.5); Inter-Quota Loan does not apply here
- Validated at transaction save time — if the resulting balance would be negative, the backend rejects the transaction and returns the set of eligible funding options (mechanism + source quotas) for the frontend to present (see B2.3 for the prompt UI).

---

## A2. Quota (Budget) Management — Domain Rules

> **Entity Model Note:** A quota is **always a single, independent entity** with exactly one Scope — **Monthly Only** or **Global Only**. There is no quota entity that itself "has" both. When a user creates a quota via the **"Both"** creation mode, the backend generates **two separate, linked quota entities** (e.g., "Food (Monthly)" and "Food (Global)"), connected by a **Linked Quota** reference, so the frontend can treat them as one conceptual budget while their balances, IDs, and transaction history stay fully separate.

### A2.1 The Major Views (conceptual, computed from transactions)

**Monthly (Active Operating):**
- At the start of each month, the designated allocation amount is credited to each **Monthly Only** quota entity (funded from the overall inflow/salary pool).
- The allocation is the absolute cap of expenditure for that entity for the month.
- Primary source of day-to-day Debits.

**Global (Accumulation & Savings):**
- At the end of every month, the remaining unspent balance of each Monthly Only quota entity is automatically swept into its **linked Global Only entity** (if paired), or into its explicitly set EOM Sweep Destination otherwise.
- Any Global Only entity acts as historical savings/reserve, and can be directly debited.

### A2.2 Quota Metadata Schema

Since all financial balances are derived dynamically from transactions, each Quota entity's metadata purely governs how the rules apply to it.

**System Default Quota:** one mandatory, non-deletable quota entity named **"Savings"**. Scope = Global Only. Acts as the ultimate fallback for system actions (deletions, untargeted sweeps).

**Quota Fields (per entity):**

| Field | Description | Rules / Applicability |
|---|---|---|
| ID | Unique internal identifier | Required |
| Name | Display name (linked-pair entities share the same base name) | Required |
| Scope | **Monthly Only** or **Global Only** | Required |
| Linked Quota ID | Reference to this entity's paired counterpart | Optional — present only if created via "Both" |
| Monthly Allocation | Amount auto-credited at the start of every month | Required if Scope = Monthly Only. N/A if Global Only |
| Target Amount (Goal) | Optional monetary target | Applicable only if Scope = Global Only. If empty/null/0 → unlimited bucket; if set → tracked Goal |
| EOM Sweep Destination | Which Global Only entity unspent money sweeps into at month-end | Applicable only if Scope = Monthly Only. Defaults to its Linked Quota (if paired); otherwise must be set explicitly, defaulting to "Savings" if unset |

**Creation Modes:**
- **"Both":** generates two linked entities — `{Name} (Monthly Only)` and `{Name} (Global Only)` — Monthly entity's EOM Sweep Destination auto-defaults to its linked Global entity.
- **"Monthly Only":** generates a single, unlinked Monthly Only entity. EOM Sweep Destination must be set explicitly (defaults to "Savings" if left unset).
- **"Global Only":** generates a single, standalone Global Only entity.

**Scope Behaviors:**
- **Monthly Only, linked:** Gets a Monthly Allocation. At month-end, sweeps into its Linked Quota's Global entity.
- **Monthly Only, unlinked:** Gets a Monthly Allocation. At month-end, sweeps into its EOM Sweep Destination (defaults to "Savings").
- **Global Only:** No Monthly Allocation. Accumulates via EOM sweeps or direct income allocation. Directly debitable.

### A2.3 Monthly Entity Computation

For every quota entity with Scope = Monthly Only, derived from transactions:
- **Allocated:** Starting amount credited this month.
- **Debited:** Total debited from this entity this month.
- **Available Balance:** Allocated + Self Transfers In + Loans In − Debited − Loans Out − Self Transfers Out.
- **Debit percentage:** % of available balance spent.

### A2.4 Global Entity Computation

For every quota entity with Scope = Global Only:
- Lifetime accumulated, unspent funds (historical swept balances + any direct income allocation).
- Directly debitable.
- **Target Amount governs interpretation:** if null/0, treated as an unlimited bucket; if set, treated as a tracked Goal with a computed % complete (accumulated ÷ Target). Reaching 100% does not block further accumulation — it's simply over-funded. (Display formatting of this in Part B, Section B2.2.)

### A2.5 Self Transfer — Directional Rules

Self Transfers do not carry an obligation to repay. Bound strictly by:

- **Paired-Quota Replenishment (Linked Global X → Linked Monthly X):** For a "Both"-created quota, money can move from its Global entity into its own linked Monthly entity.
- **Cross-Quota Transfer (Global Y ↔ Monthly X):** Bidirectional, between any Global Only entity (Y) and any Monthly Only entity (X) not already linked to each other.
  - Forward (Y → X): funding a Monthly expense from Global savings.
  - Reverse (X → Y): moving unspent Monthly funds into a Global Only entity manually, ahead of the automatic EOM sweep.
- **Global-to-Global Transfer (W ↔ Z):** Bidirectional, between any two Global Only entities.

### A2.6 Inter-Quota Loans — Directional Rules & Repayment

Inter-Quota Loans carry a strict obligation to repay.

**Directional Rules:**
- **Global-to-Monthly (Y → X):** Any Global Only entity lends to any Monthly Only entity, to cover a shortfall for the current month.
- **Monthly-to-Monthly (Z → X):** An active Monthly Only entity lends its unspent monthly balance to another Monthly Only entity.

**Insufficient Funds — Eligibility Computation (backend logic; prompt UI is Part B, Section B2.3):**
- On a Debit attempt with insufficient balance, the backend determines which mechanism(s) are valid for the target entity:
  - **Monthly target:** both Self Transfer and Inter-Quota Loan are valid.
  - **Global Only target:** only Self Transfer is valid (Global-to-Global, or Reverse Cross-Quota from a Monthly entity). Inter-Quota Loan does not apply, since loans only lend *into* Monthly entities.
- The backend computes and returns the full list of **eligible source quotas with sufficient balance**, filtered to whichever mechanism(s) apply.
- On selection, the backend generates the funding transaction (Self Transfer or Inter-Quota Loan) followed by the original Debit.

**Repayment Rule:** When the borrowing Monthly entity next receives its monthly allocation, that allocation is first used to auto-repay the lending entity, before any remainder becomes available for spending.

### A2.7 Quota Deletion Rule

A quota entity cannot be hard-deleted without losing financial history.

**Deletion Action:** for each entity involved (one if unlinked, two if a linked "Both" pair):
1. Auto-generates a transfer moving that entity's remaining balance into the mandatory "Savings" Global Quota.
2. Marks that entity as **Archived** — excluded from active queries/dropdowns, but historical transactions remain linked for accurate past reporting.

If only one half of a linked pair is deleted, the surviving half's Linked Quota ID reference is cleared.

---

## A3. Per-Counterparty Ledger

Derived mechanism: net Debit/Loan Received minus Credit, grouped per Counterparty.

### A3.1 Self vs. On-Behalf-of-Others Payments
- Marked **On Behalf of Other(s)** via Payment Mode; Counterparty = the person.
- Feeds the ledger as a receivable.
- **Settle:** auto-generates a Credit transaction for the outstanding amount.

### A3.2 Lending & Borrowing (from Others/Family)
- Money lent → Debit, creates receivable. **Settle** (they repay you): auto-generates a Credit, Counterparty = that person — same mechanism as A3.1.
- Money borrowed → **Loan Received**, creates payable — this is an **External Loan**, tracked per A3.3.

### A3.3 External Loans — Tracking & Settlement

External Loans (Type = Loan Received) are computed and stored **separately from Inter-Quota Loans** (A2.6), since they behave fundamentally differently:

| | External Loan | Inter-Quota Loan (A2.6) |
|---|---|---|
| Money involved | Real, external — enters/leaves total balance | Internal only — never leaves total balance |
| Balance Effect | Balance-Changing | Non-Balance-Changing |
| Repayment | Manual — user chooses when and from which quota | Automatic — from next allocation |

- Tracked **per Counterparty** (one combined running balance per lender, not per individual loan transaction).
- **Outstanding balance** = sum of all Loan Received amounts for that Counterparty − sum of all repayment Debits tagged to that Counterparty. Multiple loans from the same lender combine into one balance.
- **Settlement:** repayment is an ordinary **Debit** transaction (Counterparty = the lender, Source Quota = any quota the user selects — Monthly Only or Global Only). Balance-Changing.
- **Partial repayment supported:** any amount up to the outstanding balance; reduces the running balance without requiring full closure.

---

## A4. Expenditure Categories — Domain Rules

### A4.1 Category Management
- Add, edit, remove categories. Each category is a single record — no Scope, no linked entities (unlike quotas).

### A4.2 Category Deletion Rule
- **"Settled"** is a mandatory, non-deletable category — the fallback for orphaned transactions.
- Deleted categories have their transactions reassigned to "Settled" automatically.

### A4.3 Monthly / Global Filtering (query logic only)
- Total spend per category is computed two ways, both from the same underlying query (sum of Debits tagged to this Category), just with a different date-range filter:
  - **Monthly:** filtered to the current calendar month.
  - **Global:** no date restriction — all history.
- No separate stored entity, no sweep logic, no linking — this is a pure filter, unlike the quota model in A2.

### A4.4 Category-to-Category Transfer
- Bulk recategorization: reassigns the Category label on a set of past transactions from a source category to a destination category.

---

## A5. Data Storage & Portability (Release 1 architecture)
- Local on-device storage via a local JSON file (Go `encoding/json` + `os`), on the server's disk.
- Atomic writes (temp file → rename) to avoid corruption on crash.
- Export/Import capabilities for manual device-to-device transfer.
- No cloud sync or external database in Release 1.

---

## A6. Summary Table — Backend Modules

| # | Module | Key Capability |
|---|---|---|
| 1 | Transactions | Single schema. Debits target Monthly and Global Only entities. Self Transfers & Loans follow strict directional rules. Non-negative balance enforced at save time. |
| 2 | Quota Domain | Each quota is a single entity (Monthly Only or Global Only); "Both" creation generates two linked entities. EOM sweep logic. Insufficient-balance eligibility computation (mechanism + source quota list). |
| 3 | Counterparty Ledger | Unified derived tracking for on-behalf payables/receivables and lending/borrowing. External Loans tracked per-Counterparty, settled via Balance-Changing Debit, partial repayment supported. |
| 4 | Categories | CRUD; mandatory "Settled" fallback; Monthly/Global as a pure date-filtered query; bulk recategorization. |
| 5 | Storage | Local JSON file persistence (server-side), export/import. |

---
---

# PART B — Frontend / UI Requirements

> These describe *what* the interface must let the user do and see. *How* they're implemented (full-page MPA reloads, htmx partial updates, or a full SPA) is a separate, still-open decision — none of the requirements below assume one approach over another.

## B1. Transaction Entry Form
- **Type:** dropdown (Debit / Credit / Salary / Loan Received / Self Transfer / Inter-Quota Loan), defaults to Debit.
- **Amount:** numeric input, compulsory.
- **Source Quota / Destination Quota:** dropdown(s), populated from active (non-Archived) quota entities; Destination Quota only shown/required when Type is Self Transfer or Inter-Quota Loan.
- **Category:** dropdown, only shown/required when Type is Debit; defaults to "Miscellaneous."
- **Counterparty:** free-text input.
- **Payment Mode:** dropdown (Self / On Behalf of Other), defaults to Self.
- **Payment Instrument:** dropdown (Cash / UPI / Card / Bank Transfer / Cheque), optional.
- **Date:** **calendar/date-picker widget** (no manual typing), defaults to today.
- **Details:** free-text area, optional.
- Inline validation feedback for missing compulsory fields before submission.

## B2. Quota Dashboard

### B2.1 Per-Quota Breakdown View
- Overall balance summary (all-time, this month, today, custom range) — aggregated across all quota entities.
- Per-quota cards/rows showing Credited, Debited, Net, Debit % — filterable by Today / This Month / Custom range.
- Quota creation form: mode selector (Both / Monthly Only / Global Only), Name, Monthly Allocation (if applicable), Target Amount (if Global Only), EOM Sweep Destination (if Monthly Only and unlinked).

### B2.2 Goal / Target Display
- For a Global Only entity with a Target Amount set: progress display (e.g., "₹25,000 / ₹50,000 — 50% Complete").
- For a Global Only entity with no Target: plain accumulated total (e.g., "Total Balance: ₹50,000").

### B2.3 Insufficient Funds Prompt
- Triggered when a Debit submission is rejected by the backend (A1.4) for insufficient balance.
- Presents: a choice of mechanism (Self Transfer / Inter-Quota Loan, filtered to what's valid per A2.6), and a list of eligible source quotas with sufficient balance for the chosen mechanism.
- **Note from prior discussion:** if the mechanism choice needs to instantly re-filter the eligible-quota list without a full page reload, that's a candidate for a small htmx-driven partial update rather than a full MPA reload — worth deciding once the frontend approach is locked in.
- On confirmation, submits both the funding transaction and the original Debit as one user action.

## B3. Loans View
- Dedicated **External Loans** dashboard, visually separate from any Inter-Quota Loan display — per-Counterparty cards showing lender, total borrowed, repaid so far, outstanding balance.
- Settlement form: amount input (supports partial, up to outstanding balance), Source Quota dropdown (any quota).

## B4. Category Views
- Category list/management screen: add / edit / remove, with "Settled" shown as non-removable.
- **Monthly / Global tab toggle** per category — switches the same underlying total between "this month" and "all-time" (pure filter, no extra loading complexity expected).
- Category dropdown (used in B1) reflects current active categories.
- Bulk recategorization UI: pick a source category and a destination category, apply to matching past transactions.

## B5. Export / Import
- Export: a download action producing the current JSON dataset.
- Import: a file upload action; on confirmation, fully replaces existing data (no merge, per Release 1 scope).

## B6. Open Architecture Decision

The following is **not yet decided** and doesn't block backend development:
- **MPA** (`html/template`, full reload per action) vs. **MPA + htmx** (partial updates on specific interactions like B2.3) vs. **full SPA** (React/Redux, requires the backend to expose JSON instead of HTML — see Part A's handler layer design).
- This choice affects only the **handler/rendering layer** of the backend (thin HTTP handlers), not any of the domain logic in Part A — safe to defer.
