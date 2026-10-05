(() => {
  'use strict';

  const navigation = document.querySelector('[data-dashboard-nav]');
  const menuToggle = document.querySelector('[data-dashboard-menu-toggle]');
  const menuClose = document.querySelector('[data-dashboard-menu-close]');
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

  const backgroundState = new Map();

  function isolateBackground(isolated) {
    if (isolated) {
      for (const element of document.body.children) {
        if (element === navigation || element === backdrop) continue;
        if (!backgroundState.has(element)) backgroundState.set(element, element.inert);
        element.inert = true;
      }
    } else {
      for (const [element, previous] of backgroundState) element.inert = previous;
      backgroundState.clear();
    }
  }

  function closeMenu({restoreFocus = false} = {}) {
    navigation.removeAttribute('data-open');
    document.body.removeAttribute('data-navigation-open');
    menuToggle.setAttribute('aria-expanded', 'false');
    backdrop.hidden = true;
    isolateBackground(false);
    navigation.inert = mobileQuery.matches;
    navigation.removeAttribute('role');
    navigation.removeAttribute('aria-modal');
    if (restoreFocus) menuToggle.focus();
  }

  function setActiveLink(activeLink) {
    links.forEach(link => {
      link.classList.toggle('is-active', link === activeLink);
      if (link === activeLink) link.setAttribute('aria-current', "location");
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
    navigation.inert = false;
    if (opening && mobileQuery.matches) {
      isolateBackground(true);
      navigation.setAttribute('role', 'dialog');
      navigation.setAttribute('aria-modal', 'true');
      menuClose?.focus();
    } else if (!opening) closeMenu({restoreFocus: true});
  });

  menuClose?.addEventListener('click', () => closeMenu({restoreFocus: true}));

  backdrop.addEventListener('click', () => closeMenu({restoreFocus: true}));

  navigation.addEventListener('click', event => {
    const link = event.target.closest('[data-dashboard-nav-link]');
    if (!link || event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;

    const target = document.querySelector(link.hash);
    if (!target || target.hidden) return;

    event.preventDefault();
    setActiveLink(link);
    closeMenu();
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    target.scrollIntoView({behavior: reducedMotion ? 'auto' : 'smooth', block: 'start'});
    target.focus({preventScroll: true});
    if (location.hash !== link.hash) history.pushState(history.state, '', link.hash);
  });

  document.addEventListener('keydown', event => {
    if (event.key === 'Tab' && mobileQuery.matches && navigation.hasAttribute('data-open')) {
      const focusable = [...navigation.querySelectorAll('button, a[href], [tabindex="0"]')]
        .filter(element => !element.disabled && element.getClientRects().length && !element.closest('[hidden]'));
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault(); last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault(); first?.focus();
      }
    }
    if (event.key === 'Escape' && navigation.hasAttribute('data-open')) {
      closeMenu({restoreFocus: true});
    }
  });

  mobileQuery.addEventListener('change', event => {
    const focused = navigation.contains(document.activeElement);
    closeMenu({restoreFocus: event.matches && focused});
    if (!event.matches && document.activeElement === menuClose) {
      navigation.querySelector('[data-dashboard-nav-group] > button')?.focus();
    }
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

  window.addEventListener('popstate', () => {
    const link = links.find(item => item.hash === location.hash);
    const target = link && document.querySelector(link.hash);
    if (!target || target.hidden) return;
    closeMenu(); setActiveLink(link);
    target.scrollIntoView({block:'start'}); target.focus({preventScroll:true});
  });

  const initialLink = links.find(link => link.hash === window.location.hash)
    || links.find(link => link.hash === '#dashboard-overview');
  setActiveLink(initialLink);
  closeMenu();
})();

// RESP-04 shared table presentation; bundled to preserve the script-request budget.
(() => {
  'use strict';
  const states = new Map();
  let pending = false;
  let counter = 0;
  const resize = 'ResizeObserver' in window ? new ResizeObserver(schedule) : null;

  function update(wrapper, state) {
    const table = wrapper.querySelector('table');
    if (!table) return;
    if (table !== state.table) {
      if (state.table) resize?.unobserve(state.table);
      state.table = table;
      resize?.observe(table);
    }
    table.querySelectorAll('td.empty').forEach(cell => {
      if (cell.querySelector('.table-empty-message')) return;
      const message = document.createElement('span');
      message.className = 'table-empty-message';
      while (cell.firstChild) message.append(cell.firstChild);
      cell.append(message);
    });
    const overflow = wrapper.scrollWidth > wrapper.clientWidth + 1;
    state.hint.hidden = !overflow;
    wrapper.classList.toggle('table-overflows', overflow);
    if (overflow) {
      wrapper.setAttribute('tabindex', '0');
      wrapper.setAttribute('aria-describedby', [state.description, state.hint.id].filter(Boolean).join(' '));
    } else {
      if (state.tabindex === null) wrapper.removeAttribute('tabindex');
      else wrapper.setAttribute('tabindex', state.tabindex);
      if (state.description) wrapper.setAttribute('aria-describedby', state.description);
      else wrapper.removeAttribute('aria-describedby');
    }
    const headers = table.tHead?.rows[0]?.cells;
    const actions = headers?.length && /^actions?$/i.test(headers[headers.length - 1].textContent.trim());
    table.classList.toggle('table-with-actions', Boolean(actions));
  }

  function refresh() {
    pending = false;
    for (const [wrapper, state] of states) {
      if (!wrapper.isConnected) {
        resize?.unobserve(wrapper);
        if (state.table) resize?.unobserve(state.table);
        state.hint.remove();
        states.delete(wrapper);
      }
    }
    document.querySelectorAll('.table-wrap').forEach(wrapper => {
      if (!wrapper.querySelector('table')) return;
      let state = states.get(wrapper);
      if (!state) {
        const hint = document.createElement('p');
        hint.id = `admin-table-scroll-${++counter}`;
        hint.className = 'table-scroll-hint';
        hint.textContent = 'Faites défiler le tableau horizontalement au toucher ou avec les flèches du clavier.';
        hint.hidden = true;
        const heading = wrapper.closest('dialog, .panel')?.querySelector('h2, h3');
        if (!wrapper.hasAttribute('aria-label') && !wrapper.hasAttribute('aria-labelledby')) {
          wrapper.setAttribute('aria-label', `Tableau : ${heading?.textContent.trim() || 'données'}`);
        }
        wrapper.setAttribute('role', 'region');
        state = {hint, tabindex: wrapper.getAttribute('tabindex'), description: wrapper.getAttribute('aria-describedby'), table: null};
        states.set(wrapper, state);
        wrapper.before(hint);
        resize?.observe(wrapper);
      }
      update(wrapper, state);
    });
  }

  function schedule() {
    if (!pending) {
      pending = true;
      requestAnimationFrame(refresh);
    }
  }
  new MutationObserver(schedule).observe(document.body, {childList: true, subtree: true});
  window.addEventListener('resize', schedule);
  refresh();
})();

// Keep keyboard traversal inside the native modal, including at either boundary.
(() => {
  document.addEventListener('keydown', event => {
    if (event.key !== 'Tab' || event.defaultPrevented) return;
    const dialog = document.activeElement?.closest('dialog:modal');
    if (!dialog) return;
    const controls = [...dialog.querySelectorAll('button, a[href], input, select, textarea, [tabindex]')]
      .filter(element => element.tabIndex >= 0 && !element.disabled && element.getClientRects().length);
    const first = controls[0], last = controls[controls.length - 1];
    if (!first) return;
    if ((event.shiftKey && document.activeElement === first) ||
        (!event.shiftKey && document.activeElement === last)) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
    }
  });
})();

// RESP-08: presentation only; the existing HTTP/session implementation stays authoritative.
(() => {
  if (!document.querySelector('#dashboard-main')) return;
  const status = document.createElement('p');
  status.className = 'ui-feedback'; status.setAttribute('role','status');
  status.setAttribute('aria-live','polite'); status.hidden = true;
  status.textContent = 'Chargement en cours…'; document.body.append(status);
  let reads = 0, timer;
  const forms = new WeakMap();
  document.addEventListener('submit', event => {
    if (forms.has(event.target)) {event.preventDefault(); event.stopImmediatePropagation();}
  }, true);
  window.DeftaFeedback = Object.freeze({begin(options = {}) {
    const mutating = !['GET','HEAD'].includes((options.method || 'GET').toUpperCase());
    const form = mutating && document.activeElement?.closest('form.entity-form');
    if (form) {
      let state = forms.get(form);
      if (!state) {
        const note = document.createElement('p'); note.className = 'form-note';
        note.setAttribute('role','status'); note.textContent = 'Enregistrement en cours…';
        const buttons = [...form.querySelectorAll('button[type=submit],button:not([type])')]
          .map(button => ({button, disabled:button.disabled}));
        state = {count:0,note,buttons,focused:document.activeElement,busy:form.getAttribute('aria-busy')};
        forms.set(form,state); form.setAttribute('aria-busy','true');
        buttons.forEach(({button}) => {button.disabled = true;}); form.append(note);
      }
      state.count++;
      return () => {
        if (--state.count) return;
        state.buttons.forEach(({button,disabled}) => {button.disabled = disabled;});
        if (state.busy === null) form.removeAttribute('aria-busy'); else form.setAttribute('aria-busy',state.busy);
        state.note.remove(); forms.delete(form);
        if (state.focused.isConnected && document.activeElement === document.body) state.focused.focus({preventScroll:true});
      };
    }
    if (++reads === 1) timer = setTimeout(() => {status.hidden = false;},150);
    return () => {if (--reads === 0) {clearTimeout(timer);status.hidden = true;}};
  }});
})();
