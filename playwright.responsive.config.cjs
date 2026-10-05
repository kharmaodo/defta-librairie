const {defineConfig} = require('@playwright/test');
const baseline = require('./playwright.config.cjs');
module.exports = defineConfig({
  ...baseline,
  testMatch: [
    '**/catalogue-responsive.spec.cjs',
    '**/admin-navigation-responsive.spec.cjs',
    '**/admin-forms-responsive.spec.cjs',
    '**/admin-tables-responsive.spec.cjs',
    '**/cover-imports-responsive.spec.cjs',
    '**/navigation-state.spec.cjs',
    '**/ui-feedback.spec.cjs',
    '**/experience-modes.spec.cjs',
    '**/responsive-final.spec.cjs'
  ],
  projects: ['chromium','firefox','webkit'].map(name => ({name,use:{browserName:name}}))
});
