(() => {
  'use strict';

  let active = null;
  let initialized = false;
  let controlledClose = false;

  function nodes() {
    const dialog = document.querySelector('#delete-confirmation-dialog');
    const form = document.querySelector('#delete-confirmation-form');
    return {
      dialog,
      form,
      input: form?.elements.confirmation,
      submit: form?.querySelector('[data-delete-confirm]'),
      cancel: Array.from(dialog?.querySelectorAll('[data-delete-cancel]') || []),
      subject: dialog?.querySelector('[data-delete-subject]'),
      expected: dialog?.querySelector('[data-delete-expected]'),
      error: dialog?.querySelector('[data-delete-error]')
    };
  }

  function updateSubmit(view) {
    if (!active || active.busy) {
      view.submit.disabled = true;
      return;
    }
    view.submit.disabled = view.input.value !== active.expected;
  }

  function reset(view) {
    view.form.reset();
    view.error.hidden = true;
    view.error.textContent = '';
    view.input.disabled = false;
    view.cancel.forEach(button => { button.disabled = false; });
    view.submit.disabled = true;
  }

  function finish(view, confirmed) {
    const current = active;
    active = null;
    reset(view);
    if (view.dialog.open) {
      controlledClose = true;
      view.dialog.close();
    }
    current?.resolve(confirmed);
    current?.trigger?.focus?.();
  }

  function cancel(view) {
    if (!active || active.busy) return;
    finish(view, false);
  }

  function initialize() {
    if (initialized) return nodes();
    const view = nodes();
    if (!view.dialog || !view.form || !view.input || !view.submit) {
      throw new Error('Dialogue de suppression indisponible');
    }
    initialized = true;

    view.input.addEventListener('input', () => updateSubmit(view));
    view.cancel.forEach(button => button.addEventListener('click', () => cancel(view)));
    view.dialog.addEventListener('cancel', event => {
      event.preventDefault();
      cancel(view);
    });
    view.dialog.addEventListener('close', () => {
      if (controlledClose) {
        controlledClose = false;
        return;
      }
      if (active && !active.busy) finish(view, false);
    });
    view.form.addEventListener('submit', async event => {
      event.preventDefault();
      if (!active || active.busy || view.input.value !== active.expected) return;
      active.busy = true;
      view.input.disabled = true;
      view.cancel.forEach(button => { button.disabled = true; });
      updateSubmit(view);
      view.error.hidden = true;
      try {
        await active.execute();
        finish(view, true);
      } catch (error) {
        if (!active) return;
        active.busy = false;
        view.input.disabled = false;
        view.cancel.forEach(button => { button.disabled = false; });
        view.error.textContent = error?.message || 'La suppression a échoué.';
        view.error.hidden = false;
        updateSubmit(view);
        view.input.focus();
      }
    });
    return view;
  }

  function run({trigger, expected, subject, execute}) {
    const view = initialize();
    if (active) {
      view.input.focus();
      return Promise.resolve(false);
    }
    if (!trigger || typeof expected !== 'string' ||
        !expected || typeof execute !== 'function') {
      return Promise.reject(new TypeError('Confirmation de suppression invalide'));
    }

    reset(view);
    view.subject.textContent = subject || expected;
    view.expected.textContent = expected;
    active = {trigger, expected, execute, busy: false, resolve: null};
    view.dialog.showModal();
    view.input.focus();

    return new Promise(resolve => {
      active.resolve = resolve;
    });
  }

  window.DeftaDeleteConfirmation = Object.freeze({run});
})();
