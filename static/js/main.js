// static/js/main.js
document.addEventListener('DOMContentLoaded', () => {
  // Éléments DOM - avec vérifications
  const resultsContainer = document.getElementById('results-container');
  if (!resultsContainer) {
    console.warn("Conteneur #results-container introuvable");
    return;
  }

  const booksList = document.getElementById('books-list');
  const loadingEl = document.getElementById('loading');
  const noMoreEl = document.getElementById('no-more');
  const searchForm = document.getElementById('search-form');
  const searchInput = document.getElementById('search-input');
  const viewBtns = document.querySelectorAll('.view-btn');

  if (!booksList || !viewBtns.length) {
    console.warn("Éléments essentiels introuvables (books-list ou view-btn)");
    return;
  }

  // Configuration
  const API_URL = '/api/books';
  const PAGE_SIZE = parseInt(resultsContainer.dataset.pageSize || 30);
  let currentView = localStorage.getItem('viewMode') || 'card';
  let currentQuery = '';

  // Vues
  const cardsView = document.getElementById('books-cards-view');
  const tableView = document.getElementById('books-table-view');

  if (!cardsView || !tableView) {
    console.warn("Conteneurs de vue introuvables (#books-cards-view ou #books-table-view)");
    return;
  }

  // Fonction pour changer la vue
  function setView(mode) {
    if (!['card', 'table'].includes(mode)) {
      mode = 'card';
    }

    currentView = mode;
    localStorage.setItem('viewMode', mode);

    // Activer le bouton correspondant
    viewBtns.forEach(btn => {
      btn.classList.toggle('active', btn.dataset.view === mode);
    });

    // Afficher la bonne vue
    cardsView.classList.toggle('view-active', mode === 'card');
    tableView.classList.toggle('view-active', mode === 'table');
  }

  // Initialisation de la vue sauvegardée
  setView(currentView);

  // Toggle au clic
  viewBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      const view = btn.dataset.view;
      if (view) {
        setView(view);
      } else {
        console.warn("Bouton sans data-view", btn);
      }
    });
  });

  // Recherche (recharge la page avec ?q=...)
  if (searchForm) {
    searchForm.addEventListener('submit', e => {
      e.preventDefault();
      const query = searchInput.value.trim();
      if (query) {
        window.location.href = `/?q=${encodeURIComponent(query)}`;
      } else {
        window.location.href = '/'; // recherche vide → retour à la liste vide
      }
    });
  }

  // Si tu veux plus tard implémenter infinite scroll / chargement dynamique :
  // window.addEventListener('scroll', () => {
  //   if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 400) {
  //     loadMoreBooks();
  //   }
  // });

  console.log("JS initialisé - vue courante :", currentView);
});