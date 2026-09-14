(() => {
  'use strict';

  const storageKey = 'defta.adminTheme';
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const saved = localStorage.getItem(storageKey);
  let manual = saved === 'light' || saved === 'dark' ? saved : null;

  function resolvedTheme() {
    return manual || (media.matches ? 'dark' : 'light');
  }

  function applyTheme() {
    const theme = resolvedTheme();
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    const button = document.querySelector('[data-theme-toggle]');
    if (button) {
      const dark = theme === 'dark';
      button.setAttribute('aria-pressed', String(dark));
      button.setAttribute('aria-label', dark ? 'Activer le thème clair' : 'Activer le thème sombre');
      button.querySelector('[data-theme-label]').textContent = dark ? 'Clair' : 'Sombre';
    }
  }

  applyTheme();

  function initialize() {
    const button = document.querySelector('[data-theme-toggle]');
    if (!button) return;
    applyTheme();
    button.addEventListener('click', () => {
      manual = resolvedTheme() === 'dark' ? 'light' : 'dark';
      localStorage.setItem(storageKey, manual);
      applyTheme();
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initialize, {once: true});
  } else {
    initialize();
  }

  media.addEventListener('change', () => {
    if (!manual) applyTheme();
  });
})();
