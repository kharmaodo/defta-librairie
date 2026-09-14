const {defineConfig} = require('@playwright/test');
module.exports = defineConfig({
  testDir: './tests/browser',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30000,
  expect: {timeout: 10000},
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:18080',
    browserName: 'chromium',
    trace: 'off', video: 'off', screenshot: 'off'
  },
  webServer: {
    command: 'node scripts/browser-test-server.cjs',
    url: 'http://127.0.0.1:18080/api/health/ready',
    reuseExistingServer: false,
    timeout: 180000,
    gracefulShutdown: {signal: 'SIGTERM', timeout: 10000}
  }
});
