import { initFlowbite } from "flowbite";

initFlowbite();

const toggle = document.querySelector('#menu-toggle');
const sidebar = document.querySelector('#sidebar');
const backdrop = document.querySelector('#sidebar-backdrop');

if (toggle && sidebar && backdrop) {
	const desktop = window.matchMedia('(min-width: 1024px)');
	const setOpen = (open) => {
		sidebar.classList.toggle('-translate-x-full', !open);
		sidebar.inert = !open;
		backdrop.classList.toggle('hidden', !open || desktop.matches);
		document.body.classList.toggle('overflow-hidden', open && !desktop.matches);
		toggle.setAttribute('aria-expanded', String(open));
	};

	setOpen(desktop.matches);
	desktop.addEventListener('change', (event) => setOpen(event.matches));
	toggle.addEventListener('click', () => setOpen(toggle.getAttribute('aria-expanded') !== 'true'));
	backdrop.addEventListener('click', () => setOpen(false));
	document.addEventListener('keydown', (event) => {
		if (event.key === 'Escape' && !desktop.matches && toggle.getAttribute('aria-expanded') === 'true') {
			setOpen(false);
			toggle.focus();
		}
	});
}

document.querySelectorAll('form[data-confirm]').forEach((form) => {
	form.addEventListener('submit', (event) => {
		if (!window.confirm(form.dataset.confirm)) event.preventDefault();
	});
});
