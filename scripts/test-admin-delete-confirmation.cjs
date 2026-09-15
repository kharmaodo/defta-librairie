const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '..', 'static/js/admin-delete-confirmation.js'), 'utf8');

class Node {
  constructor() {
    this.handlers = {};
    this.hidden = false;
    this.disabled = false;
    this.textContent = '';
    this.value = '';
    this.focused = 0;
  }
  addEventListener(type, handler) {
    (this.handlers[type] ||= []).push(handler);
  }
  async dispatch(type) {
    const event = {preventDefault() {}};
    for (const handler of this.handlers[type] || []) await handler(event);
  }
  focus() { this.focused++; }
}

function setup() {
  const dialog = new Node();
  const form = new Node();
  const input = new Node();
  const submit = new Node();
  const close = new Node();
  const cancel = new Node();
  const subject = new Node();
  const expected = new Node();
  const error = new Node();
  dialog.open = false;
  dialog.showModal = () => { dialog.open = true; };
  dialog.close = () => { dialog.open = false; queueMicrotask(() => dialog.dispatch('close')); };
  dialog.querySelector = selector => ({
    '[data-delete-subject]': subject,
    '[data-delete-expected]': expected,
    '[data-delete-error]': error
  })[selector];
  dialog.querySelectorAll = () => [close, cancel];
  form.elements = {confirmation: input};
  form.querySelector = () => submit;
  form.reset = () => { input.value = ''; };
  const document = {querySelector: selector => selector === '#delete-confirmation-dialog' ? dialog : form};
  const window = {};
  vm.runInNewContext(source, {window, document, Array, Error, TypeError, Promise, Object});
  return {api: window.DeftaDeleteConfirmation, dialog, form, input, submit, close, cancel, subject, expected, error};
}

test('confirmation is exact, resets and restores focus', async () => {
  const h = setup();
  const trigger = new Node();
  let executions = 0;
  const result = h.api.run({
    trigger,
    expected: 'Livre Exact',
    subject: 'le livre « Livre Exact »',
    execute: async () => { executions++; }
  });

  assert.equal(h.dialog.open, true);
  assert.equal(h.input.focused, 1);
  assert.equal(h.expected.textContent, 'Livre Exact');
  assert.equal(h.submit.disabled, true);

  h.input.value = 'Livre Exact ';
  await h.input.dispatch('input');
  assert.equal(h.submit.disabled, true);

  h.input.value = 'livre Exact';
  await h.input.dispatch('input');
  assert.equal(h.submit.disabled, true);

  h.input.value = 'Livre Exact';
  await h.input.dispatch('input');
  assert.equal(h.submit.disabled, false);

  await h.form.dispatch('submit');
  assert.equal(await result, true);
  assert.equal(executions, 1);
  assert.equal(h.dialog.open, false);
  assert.equal(h.input.value, '');
  assert.equal(h.submit.disabled, true);
  assert.equal(trigger.focused, 1);

  const cancelled = h.api.run({trigger, expected: 'Livre Exact', execute: async () => { executions++; }});
  assert.equal(h.input.value, '');
  await Promise.resolve();
  assert.equal(h.dialog.open, true);
  await h.cancel.dispatch('click');
  assert.equal(await cancelled, false);
  assert.equal(executions, 1);
});

test('failed execution remains announced and cannot double-submit', async () => {
  const h = setup();
  const trigger = new Node();
  let executions = 0;
  const result = h.api.run({
    trigger,
    expected: 'REF-1',
    execute: async () => {
      executions++;
      throw new Error('Suppression refusée');
    }
  });
  h.input.value = 'REF-1';
  await h.input.dispatch('input');
  await Promise.all([h.form.dispatch('submit'), h.form.dispatch('submit')]);

  assert.equal(executions, 1);
  assert.equal(h.dialog.open, true);
  assert.equal(h.error.hidden, false);
  assert.equal(h.error.textContent, 'Suppression refusée');
  assert.equal(h.submit.disabled, false);

  await h.close.dispatch('click');
  assert.equal(await result, false);
});
