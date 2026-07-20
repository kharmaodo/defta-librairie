document.addEventListener('DOMContentLoaded', () => {
  const toggleButtons = document.querySelectorAll('.view-btn');
  const cardsView = document.getElementById('books-cards-view');
  const tableView = document.getElementById('books-table-view');

  // Récupérer la vue sauvegardée (ou 'card' par défaut)
  let currentView = localStorage.getItem('preferredView') || 'card';

  function setView(view) {
    currentView = view;
    localStorage.setItem('preferredView', view);

    // Activer le bon bouton
    toggleButtons.forEach(btn => {
      btn.classList.toggle('active', btn.dataset.view === view);
    });

    // Afficher la vue correspondante
    if (view === 'card') {
      cardsView.classList.add('view-active');
      tableView.classList.remove('view-active');
    } else {
      tableView.classList.add('view-active');
      cardsView.classList.remove('view-active');

      // Optionnel : si table vide, copier les données des cards
      if (tableView.querySelector('tbody').children.length === 0) {
        copyCardsToTable();
      }
    }
  }

  // Fonction pour copier les cards vers la table (premier basculement)
  function copyCardsToTable() {
    const tbody = document.getElementById('table-body');
    tbody.innerHTML = ''; // vider d'abord

    const cards = document.querySelectorAll('.book-card');
    cards.forEach(card => {
      const title = card.querySelector('.title').textContent;
      const author = card.querySelector('.author').textContent;
      const publisher = card.querySelector('.publisher').textContent;
      const price = card.querySelector('.price').textContent;
      const category = card.querySelector('.category').textContent;
      const statusEl = card.querySelector('.status');
      const status = statusEl ? statusEl.textContent.trim() : 'غير محدد';
      const statusClass = statusEl ? statusEl.className : '';

      const imgSrc = card.querySelector('img') ? card.querySelector('img').src : '';

      const row = document.createElement('tr');
      row.innerHTML = `
        <td>${imgSrc ? `<img src="${imgSrc}" alt="${title}" style="width:80px;">` : 'لا غلاف'}</td>
        <td>${title}</td>
        <td>${author}</td>
        <td>${publisher}</td>
        <td>${price}</td>
        <td>${category}</td>
        <td class="${statusClass}">${status}</td>
      `;
      tbody.appendChild(row);
    });
  }

  // Écouteur sur les boutons
  toggleButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      setView(btn.dataset.view);
    });
  });

  // Charger la vue sauvegardée au démarrage
  setView(currentView);
});