// Money stays in integer cents, including tax rounding.
export function cents(value) {
  const text = String(value).trim().replace(',', '.');
  if (!/^\d{1,12}(\.\d{1,2})?$/.test(text)) throw new Error('Enter an amount with up to two decimal places.');
  const [whole, fraction = ''] = text.split('.');
  return BigInt(whole) * 100n + BigInt(fraction.padEnd(2, '0'));
}

export function calculate(items, tax) {
  const rate = cents(tax);
  if (rate > 10000n) throw new Error('Tax must be between 0 and 100%.');
  const lines = items.map(item => {
    if (!/^[1-9]\d{0,5}$/.test(String(item.quantity))) throw new Error('Quantity must be a whole number between 1 and 999999.');
    return cents(item.price) * BigInt(item.quantity);
  });
  const subtotal = lines.reduce((sum, line) => sum + line, 0n);
  const taxAmount = (subtotal * rate + 5000n) / 10000n;
  return { lines, subtotal, tax: taxAmount, total: subtotal + taxAmount, rate };
}

export function money(value) {
  return `${new Intl.NumberFormat('en-US').format(value / 100n)}.${String(value % 100n).padStart(2, '0')}`;
}

export function parseDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) throw new Error('Enter a valid date.');
  const date = new Date(`${value}T12:00:00Z`);
  if (Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== value || date.getUTCFullYear() < 1900) throw new Error('Enter a valid date from year 1900 onward.');
  return date;
}

export function monthEnd(period) {
  const date = parseDate(`${period}-01`);
  date.setUTCMonth(date.getUTCMonth() + 1, 0);
  return date.toISOString().slice(0, 10);
}

export function dueDate(issue, terms) {
  const date = parseDate(issue);
  if (terms === 'end_of_month') return monthEnd(issue.slice(0, 7));
  if (terms === '30_days') date.setUTCDate(date.getUTCDate() + 30);
  const result = date.toISOString().slice(0, 10);
  parseDate(result);
  return result;
}

export function prettyDate(value) {
  if (!value) return '—';
  return new Intl.DateTimeFormat('en-GB', { day: '2-digit', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(parseDate(value));
}

export function nextNumber(year, numbers) {
  const prefix = `INV-${year}-`;
  const sequences = numbers.filter(number => number.startsWith(prefix) && /^\d{1,9}$/.test(number.slice(prefix.length))).map(number => Number(number.slice(prefix.length)));
  return prefix + String(Math.max(0, ...sequences) + 1).padStart(3, '0');
}

export function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
}
