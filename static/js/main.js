function setView(mode) {
  if (!['card', 'table'].includes(mode)) mode = 'card';

  currentView = mode;
  localStorage.setItem('viewMode', mode);

  toggleButtons.forEach(btn => {
    btn.classList.toggle('active', btn.dataset.view === mode);
  });

  // Force l’affichage (très important pour contourner certains conflits CSS)
  cardsView.classList.toggle('view-active', mode === 'card');
  cardsView.classList.toggle('hidden', mode !== 'card');
  cardsView.style.display = mode === 'card' ? 'grid' : 'none';

  tableView.classList.toggle('view-active', mode === 'table');
  tableView.classList.toggle('hidden', mode !== 'table');
  tableView.style.display = mode === 'table' ? 'block' : 'none';
}

document.addEventListener('DOMContentLoaded', () => {
  const toggleButtons = document.querySelectorAll('.view-btn');
  const cardsView = document.getElementById('books-cards-view');
  const tableView = document.getElementById('books-table-view');

  if (!cardsView || !tableView || toggleButtons.length === 0) return;

  let currentView = localStorage.getItem('viewMode') || 'card';

  function setView(mode) {
    currentView = mode;
    localStorage.setItem('viewMode', mode);

    toggleButtons.forEach(btn => {
      btn.classList.toggle('active', btn.dataset.view === mode);
    });

    cardsView.classList.toggle('hidden', mode !== 'card');
    tableView.classList.toggle('hidden', mode !== 'table');
  }

  toggleButtons.forEach(btn => {
    btn.addEventListener('click', () => setView(btn.dataset.view));
  });

  setView(currentView);
});