const fragment = new URLSearchParams(location.hash.slice(1));
if (fragment.has('token')) sessionStorage.setItem('local-bots-token', fragment.get('token'));
history.replaceState(null, '', location.pathname);

document.addEventListener('htmx:configRequest', event => {
  event.detail.headers.Authorization = 'Bearer ' + (sessionStorage.getItem('local-bots-token') || '');
});

document.addEventListener('htmx:beforeSwap', event => {
  if (event.detail.xhr.status >= 400) {
    event.detail.shouldSwap = false;
    const notice = document.getElementById('notice');
    if (notice) notice.textContent = event.detail.xhr.status === 401
      ? 'Open the dashboard link printed by bots serve to connect.'
      : event.detail.xhr.responseText;
  }
});

document.addEventListener('htmx:sendError', () => {
  document.getElementById('notice').textContent = 'Connection lost. Check that the local service is running.';
});
