import { calculate, money, parseDate, monthEnd, dueDate, prettyDate, nextNumber, escapeHTML as esc } from './invoice.mjs';

const form = document.querySelector('#editor');
const paper = document.querySelector('#invoice');
const itemList = document.querySelector('#items');
const printButton = document.querySelector('#print');
const status = document.querySelector('#status');
const DRAFT_KEY = 'invoice:draft:v1';
const NUMBERS_KEY = 'invoice:numbers:v1';
const fieldNames = ['number', 'period', 'issueDate', 'terms', 'dueDate', 'sellerName', 'sellerAddress', 'sellerEmail', 'sellerPhone', 'sellerVAT', 'customerName', 'customerAddress', 'customerEmail', 'customerPhone', 'customerVAT', 'currency', 'tax', 'paymentDetails', 'notes'];
const field = name => form.elements.namedItem(name);
const now = new Date();
const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;

function message(text, error = false) {
  status.textContent = text;
  status.classList.toggle('error', error);
}

function registry() {
  const value = JSON.parse(localStorage.getItem(NUMBERS_KEY) || '{}');
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('The saved invoice number list could not be read.');
  return value;
}

function suggestNumber() {
  try { return nextNumber(field('issueDate').value.slice(0, 4) || now.getFullYear(), Object.keys(registry())); }
  catch { return `INV-${now.getFullYear()}-001`; }
}

function addItem(item = { description: '', quantity: '1', price: '' }) {
  if (itemList.children.length >= 50) return message('An invoice can contain up to 50 items.', true);
  const row = document.createElement('div');
  row.className = 'item-editor';
  row.innerHTML = `<div class="item-editor-head"><span class="item-label"></span><button class="remove-item" type="button" aria-label="Remove item">×</button></div>
    <label>Description<textarea data-field="description" rows="2" maxlength="1000" required placeholder="e.g. Monthly software development"></textarea></label>
    <div class="field-grid"><label>Quantity<input data-field="quantity" type="number" min="1" max="999999" step="1" required></label>
    <label>Unit price<input data-field="price" type="text" inputmode="decimal" pattern="[0-9]{1,12}([.,][0-9]{1,2})?" placeholder="0.00" required></label></div>`;
  for (const input of row.querySelectorAll('[data-field]')) input.value = String(item[input.dataset.field] ?? '').slice(0, input.maxLength > 0 ? input.maxLength : 100);
  row.querySelector('button').addEventListener('click', () => {
    if (itemList.children.length === 1) return message('Keep at least one item on the invoice.', true);
    row.remove();
    numberItems();
    render();
  });
  itemList.append(row);
  numberItems();
}

function numberItems() {
  [...itemList.children].forEach((row, index) => {
    row.querySelector('.item-label').textContent = `ITEM ${String(index + 1).padStart(2, '0')}`;
    row.querySelector('button').setAttribute('aria-label', `Remove item ${index + 1}`);
  });
}

function readForm() {
  const data = Object.fromEntries(fieldNames.map(name => [name, field(name).value.trim()]));
  data.items = [...itemList.children].map(row => Object.fromEntries([...row.querySelectorAll('[data-field]')].map(input => [input.dataset.field, input.value.trim()])));
  return data;
}

function applyData(data) {
  if (!data || typeof data !== 'object' || !Array.isArray(data.items) || !data.items.length || data.items.length > 50) throw new Error('This saved draft is not valid.');
  for (const name of fieldNames) {
    const input = field(name);
    input.value = typeof data[name] === 'string' ? data[name].slice(0, input.maxLength > 0 ? input.maxLength : 2000) : '';
  }
  itemList.replaceChildren();
  data.items.forEach(addItem);
  updateDueDate();
  render();
}

function updateDueDate() {
  field('dueDate').readOnly = field('terms').value !== 'custom';
  if (!field('dueDate').readOnly) return;
  try { field('dueDate').value = dueDate(field('issueDate').value, field('terms').value); }
  catch { field('dueDate').value = ''; }
}

const shown = (value, placeholder) => value ? esc(value) : `<span class="placeholder">${esc(placeholder)}</span>`;
const optional = (value, prefix = '') => value ? `<p>${esc(prefix + value)}</p>` : '';

function render() {
  const d = readForm();
  let totals;
  try { totals = calculate(d.items, d.tax); }
  catch { totals = { lines: d.items.map(() => 0n), subtotal: 0n, tax: 0n, total: 0n, rate: 0n }; }
  const amount = value => `${money(value)} ${esc(d.currency)}`;
  const displayDate = value => { try { return prettyDate(value); } catch { return '—'; } };
  paper.innerHTML = `<header><div><p class="seller-name">${shown(d.sellerName, 'Your name or company')}</p><div class="seller-contact"><p class="multiline">${shown(d.sellerAddress, 'Your address')}</p>${optional(d.sellerVAT, 'VAT: ')}${optional(d.sellerPhone)}${optional(d.sellerEmail)}</div></div>
    <div class="right"><h1>INVOICE</h1><p class="number"># ${esc(d.number)}</p><p class="balance-heading">Balance Due</p><p class="balance-amount">${amount(totals.total)}</p></div></header>
    <section class="billing"><div><h2>Bill To</h2><p class="customer-name">${shown(d.customerName, 'Client name or company')}</p><p class="multiline">${shown(d.customerAddress, 'Client address')}</p>${optional(d.customerVAT, 'VAT: ')}${optional(d.customerPhone)}${optional(d.customerEmail)}</div>
    <dl><dt>Invoice Date :</dt><dd>${displayDate(d.issueDate)}</dd><dt>Due Date :</dt><dd>${displayDate(d.dueDate)}</dd><dt>Service Period :</dt><dd>${esc(d.period)}</dd></dl></section>
    <table><colgroup><col style="width:6%"><col style="width:46%"><col style="width:10%"><col style="width:18%"><col style="width:20%"></colgroup><thead><tr><th scope="col">#</th><th scope="col">Item &amp; Description</th><th scope="col" class="numeric">Qty</th><th scope="col" class="numeric">Rate</th><th scope="col" class="numeric">Amount</th></tr></thead><tbody>
    ${d.items.map((item, index) => `<tr><td>${index + 1}</td><td class="multiline">${shown(item.description, 'Your service or product')}</td><td class="numeric">${esc(item.quantity)}</td><td class="numeric">${item.quantity && /^\d+$/.test(item.quantity) && BigInt(item.quantity) > 0n ? money(totals.lines[index] / BigInt(item.quantity)) : '0.00'}</td><td class="numeric">${money(totals.lines[index])}</td></tr>`).join('')}</tbody></table>
    <section class="totals"><p><span>Sub Total</span><span>${amount(totals.subtotal)}</span></p>${totals.rate ? `<p><span>Tax (${money(totals.rate)}%)</span><span>${amount(totals.tax)}</span></p>` : ''}<p class="total"><span>Total</span><span>${amount(totals.total)}</span></p><p class="balance-due"><span>Balance Due</span><span>${amount(totals.total)}</span></p></section>
    <div class="closing"><section class="details"><h2>Notes</h2><p class="multiline">${esc(d.notes || 'Thank you for your business.')}</p></section>${d.paymentDetails ? `<section class="details"><h2>Payment Details</h2><p class="multiline">${esc(d.paymentDetails)}</p></section>` : ''}</div><footer>Payment reference: ${esc(d.number)}</footer>`;
  resizePreview();
}

function resizePreview() {
  const available = document.querySelector('.paper-stage').clientWidth;
  const scale = Math.min(1, available / paper.offsetWidth);
  paper.style.transform = `scale(${scale})`;
  const frame = document.querySelector('#paper-frame');
  frame.style.width = `${paper.offsetWidth * scale}px`;
  frame.style.height = `${paper.offsetHeight * scale}px`;
  document.querySelector('#zoom').textContent = `${Math.round(scale * 100)}%`;
}

form.addEventListener('input', event => {
  if (event.target.name === 'issueDate' || event.target.name === 'terms') updateDueDate();
  message('');
  render();
});
form.addEventListener('change', event => {
  if (event.target.name === 'terms') updateDueDate();
  render();
});
document.querySelector('#add-item').addEventListener('click', () => { addItem(); render(); });
document.querySelector('#next-number').addEventListener('click', () => { field('number').value = suggestNumber(); render(); message('The next available invoice number is ready.'); });
document.querySelector('#month-end').addEventListener('click', () => {
  try { field('issueDate').value = monthEnd(field('period').value); field('number').value = suggestNumber(); updateDueDate(); render(); }
  catch (error) { message(error.message, true); }
});
document.querySelector('#save-draft').addEventListener('click', () => {
  try { localStorage.setItem(DRAFT_KEY, JSON.stringify(readForm())); message('Draft saved in this browser.'); }
  catch { message('Your browser could not save the draft. Check that site storage is allowed.', true); }
});
document.querySelector('#clear-draft').addEventListener('click', () => {
  try { localStorage.removeItem(DRAFT_KEY); message('Saved draft removed. The current form is still here.'); }
  catch { message('Your browser could not remove the saved draft.', true); }
});
document.querySelector('#example').addEventListener('click', () => {
  if ((field('sellerName').value || field('customerName').value) && !confirm('Replace the current form with example details? Saved drafts will stay on this device.')) return;
  applyData({ number: suggestNumber(), issueDate: today, period: today.slice(0, 7), terms: 'end_of_month', currency: 'CHF', tax: '0', sellerName: 'Example Studio', sellerAddress: '12 Studio Lane\nLondon, United Kingdom', sellerEmail: 'hello@example.com', customerName: 'Example Client', customerAddress: '24 Example Street\nGenève, Switzerland', customerEmail: 'billing@example.com', items: [{ description: 'Monthly software development services', quantity: '1', price: '1250.00' }], notes: 'Thank you for your business.' });
  message('Example loaded. Replace these details with your own.');
});

form.addEventListener('submit', async event => {
  event.preventDefault();
  if (!form.reportValidity()) return;
  const d = readForm();
  printButton.disabled = true;
  try {
    for (const name of ['sellerName', 'sellerAddress', 'customerName', 'customerAddress']) if (!d[name]) throw new Error('Enter both the seller and client names and addresses.');
    parseDate(d.issueDate); parseDate(d.dueDate); parseDate(`${d.period}-01`);
    if (d.dueDate < d.issueDate) throw new Error('The due date cannot be earlier than the invoice date.');
    if (d.items.some(item => !item.description)) throw new Error('Give each item a description.');
    const totals = calculate(d.items, d.tax);
    if (totals.total <= 0n) throw new Error('The invoice total must be greater than zero.');
    // Only a fingerprint is stored for printed invoices, never their contents.
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify(d)));
    const fingerprint = [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, '0')).join('');
    const reserve = () => {
      const numbers = registry();
      if (Object.hasOwn(numbers, d.number) && numbers[d.number] !== fingerprint) throw new Error('This invoice number was already used with different details. Click Next to use a new number.');
      numbers[d.number] = fingerprint;
      localStorage.setItem(NUMBERS_KEY, JSON.stringify(numbers));
    };
    if (navigator.locks) await navigator.locks.request('invoice-number-reservation', reserve);
    else reserve();
    render();
    document.title = d.number;
    window.print();
    message('Choose Save as PDF in the print dialog. Keep the downloaded file as your invoice record.');
  } catch (error) {
    message(error instanceof DOMException ? 'Your browser blocked local storage or PDF preparation. Open this site over HTTPS and allow site storage.' : error.message, true);
    status.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  } finally { printButton.disabled = false; }
});

field('issueDate').value = today;
field('period').value = today.slice(0, 7);
field('number').value = suggestNumber();
addItem();
updateDueDate();
try {
  const saved = localStorage.getItem(DRAFT_KEY);
  if (saved) { applyData(JSON.parse(saved)); message('Your saved draft is ready.'); }
} catch { message('A saved draft could not be loaded. You can start a new invoice.', true); }
render();
new ResizeObserver(resizePreview).observe(document.querySelector('.paper-stage'));
printButton.disabled = false;
