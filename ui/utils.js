// utils.js — Pure helpers and network utilities

export function showToast(message, type = 'success', ms = 4000) {
  const toasts = document.getElementById('toasts');
  if (!toasts) return;
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = message;
  toasts.appendChild(el);
  setTimeout(() => el.remove(), ms);
}

export function setFormError(id, msg) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = msg || '';
}

export async function fetchJSON(url, options = {}) {
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

export function normalizeFormNumbers(payload, fields) {
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

export function formatDate(dateString) {
  const d = new Date(dateString);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// Used any time task/user-entered text gets dropped into innerHTML
export function escapeHTML(str) {
  const div = document.createElement('div');
  div.textContent = str == null ? '' : String(str);
  return div.innerHTML;
}

export function toDateOnly(dateString) {
  if (!dateString) return '';
  return String(dateString).slice(0, 10);
}