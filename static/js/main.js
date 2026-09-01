document.addEventListener('DOMContentLoaded', () => {
  const toggleButtons = document.querySelectorAll('.view-btn');
  const cardsView = document.getElementById('books-cards-view');
  const tableView = document.getElementById('books-table-view');
  if (!cardsView || !tableView || toggleButtons.length === 0) return;

  const setView = (mode) => {
    const selectedView = mode === 'table' ? 'table' : 'card';
    const showCards = selectedView === 'card';
    localStorage.setItem('viewMode', selectedView);
    cardsView.hidden = !showCards;
    tableView.hidden = showCards;
    cardsView.classList.toggle('view-active', showCards);
    tableView.classList.toggle('view-active', !showCards);
    toggleButtons.forEach((button) => {
      const active = button.dataset.view === selectedView;
      button.classList.toggle('active', active);
      button.setAttribute('aria-pressed', String(active));
    });
  };

  toggleButtons.forEach((button) => button.addEventListener('click', () => setView(button.dataset.view)));
  setView(localStorage.getItem('viewMode') || 'card');
});
