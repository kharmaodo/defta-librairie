(() => {
  'use strict';

  const navigation = document.querySelector('[data-dashboard-nav]');
  const menuToggle = document.querySelector('[data-dashboard-menu-toggle]');
  const backdrop = document.querySelector('[data-dashboard-nav-backdrop]');

  if (!navigation || !menuToggle || !backdrop) return;

  const groups = [...navigation.querySelectorAll('[data-dashboard-nav-group]')];
  const links = [...navigation.querySelectorAll('[data-dashboard-nav-link]')];
  const mobileQuery = window.matchMedia('(max-width: 1100px)');

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
    document.body.removeAttribute('data-navigation-open');
    menuToggle.setAttribute('aria-expanded', 'false');
    backdrop.hidden = true;
    if (restoreFocus) menuToggle.focus();
  }

  function setActiveLink(activeLink) {
    links.forEach(link => {
      link.classList.toggle('is-active', link === activeLink);
      if (link === activeLink) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    });

    const activeGroup = activeLink?.closest('[data-dashboard-nav-group]');
    if (activeGroup) setGroupExpanded(activeGroup, true);
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
    document.body.toggleAttribute('data-navigation-open', opening);
    menuToggle.setAttribute('aria-expanded', String(opening));
    backdrop.hidden = !opening;
    if (opening) navigation.querySelector('[aria-current="location"], button, a')?.focus();
  });

  backdrop.addEventListener('click', () => closeMenu({restoreFocus: true}));

  navigation.addEventListener('click', event => {
    const link = event.target.closest('[data-dashboard-nav-link]');
    if (!link) return;

    const target = document.querySelector(link.hash);
    if (!target || target.hidden) return;

    event.preventDefault();
    setActiveLink(link);
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

  mobileQuery.addEventListener('change', event => {
    if (!event.matches) closeMenu();
  });

  const visibleTargets = links
    .map(link => ({link, target: document.querySelector(link.hash)}))
    .filter(item => item.target);

  if ('IntersectionObserver' in window) {
    const observer = new IntersectionObserver(entries => {
      const visible = entries
        .filter(entry => entry.isIntersecting && !entry.target.hidden)
        .sort((left, right) => right.intersectionRatio - left.intersectionRatio)[0];
      if (!visible) return;
      setActiveLink(visibleTargets.find(item => item.target === visible.target)?.link);
    }, {rootMargin: '-15% 0px -70% 0px', threshold: [0, .25, .75]});

    visibleTargets.forEach(item => observer.observe(item.target));
  }

  const initialLink = links.find(link => link.hash === window.location.hash)
    || links.find(link => link.hash === '#dashboard-overview');
  setActiveLink(initialLink);
})();
