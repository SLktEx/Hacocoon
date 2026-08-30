const cursorKey = 'hacocoon.notifications.cursor.v1';
const seenKey = 'hacocoon.notifications.seen.v1';
const maxSeen = 512;
const pollMs = 1500;

const statusEl = document.getElementById('status');
const listEl = document.getElementById('events');
const enableButton = document.getElementById('enable');
const completedCheckbox = document.getElementById('completed');

let cursor = Number.parseInt(localStorage.getItem(cursorKey) || '0', 10);
if (!Number.isSafeInteger(cursor) || cursor < 0) cursor = 0;
let seen = loadSeen();
let registration;

function loadSeen() {
  try {
    const value = JSON.parse(localStorage.getItem(seenKey) || '[]');
    return Array.isArray(value) ? value.filter((item) => typeof item === 'string').slice(-maxSeen) : [];
  } catch {
    return [];
  }
}

function remember(eventId) {
  if (!eventId || seen.includes(eventId)) return;
  seen.push(eventId);
  if (seen.length > maxSeen) seen = seen.slice(-maxSeen);
  localStorage.setItem(seenKey, JSON.stringify(seen));
}

function notificationText(event) {
  const details = [event.environment, event.capability, event.action].filter(Boolean).join(' · ');
  switch (event.kind) {
    case 'approval-required': return ['Hacocoon approval required', details];
    case 'recovery-required': return ['Hacocoon needs recovery', details || event.code || 'Recovery required'];
    case 'operation-failed': return ['Hacocoon operation failed', details || event.code || 'Operation failed'];
    case 'policy-denied': return ['Hacocoon policy denied', details || event.code || 'Policy denied'];
    case 'approval-denied': return ['Hacocoon approval denied', details || event.code || 'Approval denied'];
    case 'operation-completed': return completedCheckbox.checked ? ['Hacocoon operation completed', details] : null;
    default: return null;
  }
}

async function notify(event) {
  const text = notificationText(event);
  if (!text || Notification.permission !== 'granted' || !registration) return;
  await registration.showNotification(text[0], {
    body: text[1],
    tag: event.event_id,
    renotify: false,
    data: { event_id: event.event_id, request_id: event.request_id }
  });
}

function render(event) {
  const item = document.createElement('li');
  item.textContent = [event.kind, event.environment, event.capability, event.action, event.code].filter(Boolean).join(' · ');
  listEl.prepend(item);
  while (listEl.children.length > 50) listEl.lastElementChild.remove();
}

async function poll() {
  try {
    const response = await fetch(`/api/v1/events?offset=${encodeURIComponent(cursor)}&limit=100`, { cache: 'no-store' });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const batch = await response.json();
    for (const event of batch.events || []) {
      if (!seen.includes(event.event_id)) {
        await notify(event);
        render(event);
        remember(event.event_id);
      }
      if (Number.isSafeInteger(event.next_offset) && event.next_offset >= cursor) {
        cursor = event.next_offset;
        localStorage.setItem(cursorKey, String(cursor));
      }
    }
    if (Number.isSafeInteger(batch.next_offset) && batch.next_offset >= cursor) {
      cursor = batch.next_offset;
      localStorage.setItem(cursorKey, String(cursor));
    }
    if (batch.error) {
      statusEl.textContent = `Paused: ${batch.error.code}`;
      return;
    }
    statusEl.textContent = `Connected · offset ${cursor}`;
  } catch (error) {
    statusEl.textContent = `Disconnected · ${error.message}`;
  }
  window.setTimeout(poll, pollMs);
}

enableButton.addEventListener('click', async () => {
  const permission = await Notification.requestPermission();
  statusEl.textContent = `Notification permission: ${permission}`;
});

(async () => {
  if ('serviceWorker' in navigator) {
    registration = await navigator.serviceWorker.register('/sw.js');
    await navigator.serviceWorker.ready;
  }
  await poll();
})();
