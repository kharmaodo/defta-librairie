// Configuration
const API_URL = '/api/books';
const PAGE_SIZE = parseInt(document.getElementById('results-container')?.dataset.pageSize || 30);
let currentView = localStorage.getItem('viewMode') || 'table';
let offset = 0;
let isLoading = false;
let hasMore = true;
let currentQuery = '';

// Éléments DOM
const resultsContainer = document.getElementById('results-container');
const booksList = document.getElementById('books-list');
const loadingEl = document.getElementById('loading');
const noMoreEl = document.getElementById('no-more');
const searchForm = document.getElementById('search-form');
const searchInput = document.getElementById('search-input');
const viewBtns = document.querySelectorAll('.view-btn');

// Fonction pour changer la vue (table / card)
function setView(mode) {
    currentView = mode;
    localStorage.setItem('viewMode', mode);

    viewBtns.forEach(btn => {
        btn.classList.toggle('active', btn.dataset.view === mode);
    });

    // Mettre à jour le conteneur
    if (mode === 'table') {
        booksList.className = 'table-view';
        booksList.innerHTML = '<table class="book-table"><thead><tr><th>الغلاف</th><th>العنوان</th><th>المؤلف</th><th>الناشر</th><th>السعر</th><th>التصنيف</th><th>الحالة</th></tr></thead><tbody id="table-body"></tbody></table>';
    } else {
        booksList.className = 'books-grid';
    }

    // Recharger les données actuelles
    offset = 0;
    hasMore = true;
    booksList.querySelector(mode === 'table' ? '#table-body' : '').innerHTML = '';
    loadMoreBooks(true);
}

// Charger des livres
async function loadMoreBooks(reset = false) {
    if (isLoading || !hasMore) return;
    isLoading = true;
    loadingEl.style.display = 'block';

    if (reset) {
        offset = 0;
        hasMore = true;
        if (currentView === 'table') {
            document.getElementById('table-body').innerHTML = '';
        } else {
            booksList.innerHTML = '';
        }
    }

    try {
        const params = new URLSearchParams({
            q: currentQuery,
            offset: offset,
            limit: PAGE_SIZE
        });

        const response = await fetch(`${API_URL}?${params}`);
        if (!response.ok) throw new Error('Network error');

        const data = await response.json();

        if (data.results.length === 0 || data.results.length < PAGE_SIZE) {
            hasMore = false;
            noMoreEl.style.display = 'block';
        }

        renderBooks(data.results);
        offset += data.results.length;

    } catch (err) {
        console.error('Error loading books:', err);
    } finally {
        isLoading = false;
        loadingEl.style.display = 'none';
    }
}

// Afficher les livres selon la vue actuelle
function renderBooks(books) {
    if (currentView === 'table') {
        const tbody = document.getElementById('table-body');
        books.forEach(book => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${book.coverUrl ? `<img src="${book.coverUrl}" alt="${book.title}" loading="lazy">` : 'لا غلاف'}</td>
                <td>${book.title}</td>
                <td>${book.auteur || 'غير محدد'}</td>
                <td>${book.editeur || 'غير محدد'}</td>
                <td>${book.price} دج</td>
                <td>${book.categorie || 'غير مصنف'}</td>
                <td class="status ${book.status === 'متوفر' ? 'available' : 'unavailable'}">
                    ${book.status || 'غير محدد'}
                </td>
            `;
            tbody.appendChild(row);
        });
    } else {
        books.forEach(book => {
            const card = document.createElement('div');
            card.className = 'book-card';
            card.innerHTML = `
                <div class="card-cover">
                    ${book.coverUrl ? `<img src="${book.coverUrl}" alt="${book.title}" loading="lazy">` : '<div class="no-cover">لا يوجد غلاف</div>'}
                </div>
                <div class="card-info">
                    <h3 class="title">${book.title}</h3>
                    <p class="author">${book.auteur || 'غير محدد'}</p>
                    <p class="publisher">${book.editeur || 'غير محدد'}</p>
                    <div class="meta">
                        <span class="price">${book.price} دج</span>
                        <span class="category">${book.categorie || 'غير مصنف'}</span>
                    </div>
                    <span class="status ${book.status === 'متوفر' ? 'available' : 'unavailable'}">
                        ${book.status || 'غير محدد'}
                    </span>
                </div>
            `;
            booksList.appendChild(card);
        });
    }
}

// Infinite scroll
window.addEventListener('scroll', () => {
    if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 400) {
        loadMoreBooks();
    }
});

// Recherche
searchForm.addEventListener('submit', e => {
    e.preventDefault();
    currentQuery = searchInput.value.trim();
    setView(currentView); // recharge avec nouvelle recherche
});

// Initialisation
viewBtns.forEach(btn => {
    btn.addEventListener('click', () => {
        setView(btn.dataset.view);
    });
});

// Charger la vue sauvegardée et les premiers résultats
setView(currentView);