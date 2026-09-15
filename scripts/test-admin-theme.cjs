const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'static/js/admin-theme.js'), 'utf8');
const template = fs.readFileSync(path.join(root, 'templates/admin.html'), 'utf8');
const css = fs.readFileSync(path.join(root, 'static/css/admin.css'), 'utf8');

test('admin theme follows system preference and persists an accessible override', () => {
  assert.match(source, /prefers-color-scheme: dark/);
  assert.match(source, /defta\.adminTheme/);
  assert.match(source, /localStorage\.setItem/);
  assert.match(source, /documentElement\.dataset\.theme/);
  assert.match(source, /aria-pressed/);
  assert.match(template, /data-theme-toggle[^>]+aria-pressed="false"/);
  assert.ok(template.indexOf('/static/js/admin-theme.js') <
    template.indexOf('/static/css/admin.css'));
  assert.match(css, /:root\[data-theme="dark"\]/);
  assert.match(css, /--surface-soft:/);
  assert.match(css, /--input-border-color:/);
});
