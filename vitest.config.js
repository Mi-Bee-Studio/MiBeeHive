const { defineConfig } = require('vitest/config');

module.exports = defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./web/tests/setup.js'],
    include: ['web/tests/**/*.test.js'],
  },
});
