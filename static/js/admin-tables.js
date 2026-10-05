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
