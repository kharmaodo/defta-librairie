document.addEventListener('DOMContentLoaded', () => {
  const toggleButtons = document.querySelectorAll('.view-btn');
  const cardsView = document.getElementById('books-cards-view');
  const tableView = document.getElementById('books-table-view');
  const resultsContainer = document.getElementById('results-container');

  if (!cardsView || !tableView || toggleButtons.length === 0) {
    console.warn("Éléments de vue introuvables");
    return;
  }

  // Vue initiale : serveur > localStorage > card
  let currentView = resultsContainer?.dataset.view ||
                    localStorage.getItem('viewMode') ||
                    'card';

  function setView(mode) {
    if (!['card', 'table'].includes(mode)) mode = 'card';

    currentView = mode;
    localStorage.setItem('viewMode', mode);

    toggleButtons.forEach(btn => {
      btn.classList.toggle('active', btn.dataset.view === mode);
    });

    cardsView.classList.toggle('hidden', mode !== 'card');
    tableView.classList.toggle('hidden', mode !== 'table');
  }

  toggleButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const view = btn.dataset.view;
      if (view) {
        setView(view);
      }
    });
  });

  setView(currentView);

  console.log("Toggle initialisé – vue courante :", currentView);
});