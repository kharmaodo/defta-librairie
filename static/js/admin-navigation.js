(() => {
  'use strict';

  const navigation = document.querySelector('[data-dashboard-nav]');
  const menuToggle = document.querySelector('[data-dashboard-menu-toggle]');

  if (!navigation || !menuToggle) return;

  const groups = [...navigation.querySelectorAll('[data-dashboard-nav-group]')];

  function setGroupExpanded(group, expanded) {
    const button = group.querySelector(':scope > button[aria-controls]');
    if (!button) return;

    const submenu = document.getElementById(button.getAttribute('aria-controls'));
    if (!submenu) return;

    button.setAttribute('aria-expanded', String(expanded));
    submenu.hidden = !expanded;
  }

  function closeMenu({restoreFocus = false} = {}) {
    navigation.removeAttribute('data-open');
    menuToggle.setAttribute('aria-expanded', 'false');
    if (restoreFocus) menuToggle.focus();
  }

  groups.forEach(group => {
    const button = group.querySelector(':scope > button[aria-controls]');
    if (!button) return;

    button.addEventListener('click', () => {
      setGroupExpanded(group, button.getAttribute('aria-expanded') !== 'true');
    });

    button.addEventListener('keydown', event => {
      if (event.key !== 'Escape') return;
      event.stopPropagation();

      if (navigation.hasAttribute('data-open')) {
        closeMenu({restoreFocus: true});
        return;
      }

      setGroupExpanded(group, false);
      button.focus();
    });
  });

  menuToggle.addEventListener('click', () => {
    const opening = !navigation.hasAttribute('data-open');
    navigation.toggleAttribute('data-open', opening);
    menuToggle.setAttribute('aria-expanded', String(opening));
    if (opening) navigation.querySelector('button, a')?.focus();
  });

  navigation.addEventListener('click', event => {
    const link = event.target.closest('[data-dashboard-nav-link]');
    if (!link) return;

    const target = document.querySelector(link.hash);
    if (!target || target.hidden) return;

    event.preventDefault();
    target.scrollIntoView({behavior: 'smooth', block: 'start'});
    target.focus({preventScroll: true});
    closeMenu();
    history.replaceState(null, '', link.hash);
  });

  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && navigation.hasAttribute('data-open')) {
      closeMenu({restoreFocus: true});
    }
  });
})();
