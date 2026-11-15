const toggle = document.querySelector('#menu-toggle');
const sidebar = document.querySelector('#sidebar');
if (toggle && sidebar) {
  const desktop = window.matchMedia('(min-width: 1024px)');
  const setOpen = (open) => {
    document.body.classList.toggle('sidebar-open', open);
    toggle.setAttribute('aria-expanded', String(open));
    sidebar.inert = !open;
  };
  setOpen(desktop.matches);
  desktop.addEventListener('change', (event) => setOpen(event.matches));
  toggle.addEventListener('click', () => setOpen(!document.body.classList.contains('sidebar-open')));
  document.addEventListener('pointerdown', (event) => {
    if (!desktop.matches && !sidebar.contains(event.target) && !toggle.contains(event.target)) setOpen(false);
  });
  document.addEventListener('keydown', (event) => { if (event.key === 'Escape') { setOpen(false); toggle.focus(); } });
}
document.querySelectorAll('form[data-confirm]').forEach((form) => {
  form.addEventListener('submit', (event) => { if (!window.confirm(form.dataset.confirm)) event.preventDefault(); });
});
