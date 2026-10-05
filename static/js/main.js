document.addEventListener('DOMContentLoaded', () => {
  const main = document.querySelector('main');
  if (!main?.querySelector('.catalogue')) return;
  let controller, busy = false, scrollTimer;
  const readMode = () => {try {return localStorage.getItem('viewMode') || 'card';} catch {return 'card';}};
  function setView(mode) {
    const selected = mode === 'table' ? 'table' : 'card';
    try {localStorage.setItem('viewMode', selected);} catch { /* Storage is optional. */ }
    const cards = main.querySelector('#books-cards-view'), table = main.querySelector('#books-table-view');
    if (!cards || !table) return;
    cards.hidden = selected !== 'card'; table.hidden = selected === 'card';
    cards.classList.toggle('view-active', !cards.hidden); table.classList.toggle('view-active', !table.hidden);
    main.querySelectorAll('.view-btn').forEach(button => {
      const active = button.dataset.view === selected;
      button.classList.toggle('active', active); button.setAttribute('aria-pressed', String(active));
    });
  }
  function prepare() {
    main.querySelectorAll('img.public-book-cover').forEach(image => image.addEventListener('error', () => {
      if (image.dataset.fallback) image.src = image.dataset.fallback;
    }, {once:true}));
    setView(readMode());
  }
  function saveScroll() {
    if (!busy) history.replaceState({...history.state, deftaCatalogue:{scroll:[scrollX,scrollY]}}, '', location.href);
  }
  async function navigate(url, {pop = false, position = [0,0]} = {}) {
    if (!pop) saveScroll();
    controller?.abort();
    const request = new AbortController(); controller = request; busy = true;
    main.setAttribute('aria-busy', 'true');
    try {
      // Only the same-origin server-rendered catalogue is imported; no API/HTML contract changes.
      const response = await fetch(url, {signal:request.signal, credentials:'same-origin', headers:{Accept:'text/html'}});
      const destination = new URL(response.url);
      if (!response.ok || destination.origin !== location.origin || destination.pathname !== '/' ||
          !response.headers.get('Content-Type')?.includes('text/html')) throw new Error('Catalogue unavailable');
      const documentHTML = new DOMParser().parseFromString(await response.text(), 'text/html');
      const content = documentHTML.querySelector('main');
      if (!content?.querySelector('.hero') || !content.querySelector('.catalogue') || content.querySelector('script')) throw new Error('Unexpected catalogue');
      if (request.signal.aborted) return;
      main.replaceChildren(...[...content.childNodes].map(node => document.importNode(node, true)));
      document.title = documentHTML.title;
      if (!pop) history.pushState({deftaCatalogue:{scroll:[0,0]}}, '', url);
      prepare();
      if (pop) {
        const heading = main.querySelector('.catalogue h2') || main.querySelector('h1');
        heading?.setAttribute('tabindex','-1'); heading?.focus({preventScroll:true});
        window.scrollTo({left:position[0], top:position[1], behavior:'instant'});
      }
      else {
        const target = main.querySelector('.catalogue h2') || main.querySelector('h1');
        target?.setAttribute('tabindex','-1'); target?.focus({preventScroll:true});
        if (url.searchParams.get('q')) main.querySelector('.catalogue').scrollIntoView({block:'start'});
        else window.scrollTo({top:0,left:0,behavior:'instant'});
      }
    } catch (error) {
      if (request.signal.aborted) return;
      // Ordinary document navigation preserves the existing SSR path on any failure.
      if (pop) location.reload(); else location.assign(url.href);
    } finally {
      if (controller === request) {busy = false; main.removeAttribute('aria-busy');}
    }
  }
  main.addEventListener('submit', event => {
    if (event.target.id !== 'search-form') return;
    event.preventDefault();
    const url = new URL('/', location.origin);
    url.search = new URLSearchParams(new FormData(event.target)).toString();
    void navigate(url);
  });
  document.addEventListener('click', event => {
    const button = event.target.closest('.view-btn');
    if (button && main.contains(button)) {setView(button.dataset.view); return;}
    const link = event.target.closest('a[href]');
    if (!link || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey ||
        link.hasAttribute('download') || (link.target && link.target !== '_self')) return;
    const url = new URL(link.href);
    if (url.origin !== location.origin || url.pathname !== '/' || url.hash) return;
    event.preventDefault(); void navigate(url);
  });
  window.addEventListener('popstate', event => {
    if (location.pathname === '/') void navigate(new URL(location.href), {pop:true, position:event.state?.deftaCatalogue?.scroll || [0,0]});
  });
  if ('scrollRestoration' in history) history.scrollRestoration = 'manual';
  window.addEventListener('scroll', () => {
    clearTimeout(scrollTimer); scrollTimer = setTimeout(saveScroll, 150);
  }, {passive:true});
  window.addEventListener('pagehide', saveScroll);
  prepare();
  const saved = history.state?.deftaCatalogue?.scroll;
  if (saved) window.scrollTo({left:saved[0],top:saved[1],behavior:'instant'});
  saveScroll();
});
