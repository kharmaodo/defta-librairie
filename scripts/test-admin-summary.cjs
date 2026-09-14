const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'static/js/admin-summary.js'), 'utf8');
const template = fs.readFileSync(path.join(root, 'templates/admin.html'), 'utf8');

test('dashboard summary keeps its scoped API and accessible state contract', () => {
  assert.match(source, /Promise\.all/);
  assert.match(source, /\/api\/manage\/statistics\?/);
  for (const kind of ['OUT_OF_STOCK', 'LOW_STOCK', 'DRAFT_PURCHASE']) {
    assert.match(source, new RegExp(kind));
  }
  assert.match(source, /statistics\.netMargin === null/);
  assert.match(source, /Choisissez une librairie pour afficher sa synthèse/);
  assert.match(template, /id="dashboard-summary"[^>]+aria-busy="true"/);
  assert.equal((template.match(/class="summary-card"/g) || []).length, 4);
  assert.match(template, /data-summary-status role="status" aria-live="polite"/);
  assert.ok(template.indexOf('/static/js/admin-http.js') <
    template.indexOf('/static/js/admin-summary.js'));
});
