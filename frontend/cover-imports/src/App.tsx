import {useCallback, useEffect, useRef, useState} from 'react';
import {batches, decide, http, libraries, preview, quarantine, review, upload, type Batch, type Candidate, type Job, type Owner, type Review, type User} from './api';
import './style.css';

const TERMINAL = new Set(['READY', 'REJECTED', 'FAILED', 'CANCELLED']);
const labels: Record<string, string> = {
  PENDING_SCAN: 'En attente de modération', NSFW_SCANNING: 'Modération', NSFW_DECIDED: 'Modération terminée',
  OCR_PENDING: 'OCR en attente', OCR_PROCESSING: 'OCR en cours', MATCHING: 'Recherche de livres',
  REVIEW_REQUIRED: 'Rattachement à revoir', QUARANTINED: 'Revue de sécurité', READY: 'Rattachement validé',
  REJECTED: 'Rejeté', FAILED: 'Échec', CANCELLED: 'Annulé'
};

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return 'Une erreur est survenue.';
}
function onError(error: unknown, setError: (text: string) => void) {
  if ((error as {status?: number})?.status === 401) {http.clearSession(); window.location.replace('/login'); return;}
  setError(errorMessage(error));
}
function formatDate(value: string) {return new Date(value).toLocaleString('fr-FR');}
function jobLabel(job: Job) {return labels[job.status] || job.status;}

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [owners, setOwners] = useState<Owner[]>([]);
  const [libraryId, setLibraryId] = useState('');
  const [items, setItems] = useState<Batch[]>([]);
  const [files, setFiles] = useState<File[]>([]);
  const key = useRef(crypto.randomUUID());
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [selected, setSelected] = useState<Job | null>(null);
  const [detail, setDetail] = useState<Review | null>(null);
  const [reviewError, setReviewError] = useState('');
  const reviewHeading = useRef<HTMLHeadingElement | null>(null);
  const [image, setImage] = useState('');
  const [chosen, setChosen] = useState<number | null>(null);
  const [deciding, setDeciding] = useState(false);
  const isRoot = user?.role === 'SUPER_ADMIN_ROOT';

  useEffect(() => {
    if (selected) {
      reviewHeading.current?.focus({preventScroll: true});
      reviewHeading.current?.scrollIntoView({block: 'nearest'});
    }
  }, [selected]);

  useEffect(() => {
    http.enableSessionRefresh();
    http.profile().then(async profile => {
      if (profile.passwordChangeRequired) {window.location.replace('/admin'); return;}
      if (profile.role !== 'SUPER_ADMIN_ROOT' && profile.role !== 'OWNER_LIBRARY') {window.location.replace('/admin'); return;}
      setUser(profile);
      if (profile.role === 'OWNER_LIBRARY') setLibraryId(profile.libraryId || '');
      else {
        const available = await libraries();
        setOwners(available);
        setLibraryId(available[0]?.library.id || '');
      }
    }).catch(err => onError(err, setError));
  }, []);

  const refresh = useCallback(async () => {
    if (!user || !libraryId) return;
    try {setItems(await batches(isRoot ? libraryId : ''));}
    catch (err) {onError(err, setError);}
  }, [user, libraryId, isRoot]);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => {if (!document.hidden) void refresh();}, 3000);
    const visible = () => {if (!document.hidden) void refresh();};
    document.addEventListener('visibilitychange', visible);
    return () => {window.clearInterval(timer); document.removeEventListener('visibilitychange', visible);};
  }, [refresh]);

  useEffect(() => {
    if (!selected || !libraryId) {setDetail(null); setImage(''); return;}
    let cancelled = false;
    let objectURL = '';
    setChosen(null);
    setReviewError('');
    setDetail(null);
    setImage('');
    const scoped = isRoot ? libraryId : '';
    Promise.all([review(selected.id, scoped), preview(selected.id, scoped)]).then(([result, url]) => {
      if (cancelled) {URL.revokeObjectURL(url); return;}
      objectURL = url; setDetail(result); setImage(url);
    }).catch(err => {if (!cancelled) onError(err, setReviewError);});
    return () => {cancelled = true; if (objectURL) URL.revokeObjectURL(objectURL);};
  }, [selected, libraryId, isRoot]);

  function chooseFiles(chosenFiles: FileList | File[]) {
    const next = Array.from(chosenFiles);
    if (!next.length) return;
    if (next.length > 100 || next.some(file => file.size === 0 || file.size > 10 * 1024 * 1024 || !['image/jpeg', 'image/png'].includes(file.type))) {
      setError('Choisissez entre 1 et 100 images JPEG ou PNG, de 10 Mo maximum chacune.'); return;
    }
    setFiles(next); key.current = crypto.randomUUID(); setError(''); setNotice('');
  }

  async function send() {
    if (!files.length || !libraryId || uploading) return;
    setUploading(true); setError(''); setNotice('Envoi en cours…');
    try {
      await upload(files, isRoot ? libraryId : '', key.current);
      setFiles([]); key.current = crypto.randomUUID();
      setNotice('Lot accepté. Les étapes sont actualisées automatiquement.');
      await refresh();
    } catch (err) {setNotice(''); onError(err, setError);}
    finally {setUploading(false);}
  }

  async function decideSelected(action: 'ACCEPT' | 'REJECT' | 'APPROVE') {
    if (!selected || !libraryId || deciding || (action === 'ACCEPT' && !chosen)) return;
    setDeciding(true); setError('');
    try {
      const scope = isRoot ? libraryId : '';
      if (selected.status === 'QUARANTINED') await quarantine(selected.id, scope, action === 'APPROVE' ? 'APPROVE' : 'REJECT');
      else await decide(selected.id, scope, action === 'ACCEPT' ? 'ACCEPT' : 'REJECT', chosen || undefined);
      setSelected(null); setNotice('Décision enregistrée.'); await refresh();
    } catch (err) {onError(err, setError); await refresh();}
    finally {setDeciding(false);}
  }

  async function logout() {
    http.clearSession();
    try {await http.authJSON('logout'); window.location.replace('/login');}
    catch {window.location.replace('/login?logoutFailed=1');}
  }

  if (!user) return <main className="wrap"><h1>Imports de couvertures</h1><p role="status">Vérification de la session…</p>{error && <p role="alert">{error}</p>}</main>;
  const reviewContent = selected && <section className="panel review" aria-labelledby="review-title"><div className="section-heading"><h2 ref={reviewHeading} tabIndex={-1} id="review-title">Revue de l’image</h2><button type="button" onClick={() => setSelected(null)}>Fermer</button></div>
        {reviewError ? <p className="alert" role="alert">{reviewError}</p> : !detail ? <p role="status">Chargement de la revue…</p> : <div className="review-grid">
          <div className="preview">{image && <img src={image} alt="Couverture importée à examiner"/>}</div>
          {selected.status === 'QUARANTINED' ? <div><p>Contenu ambigu : seule une décision root peut poursuivre l’OCR.</p><div className="actions"><button className="primary" type="button" disabled={deciding} onClick={() => void decideSelected('APPROVE')}>Autoriser l’OCR</button><button type="button" disabled={deciding} onClick={() => void decideSelected('REJECT')}>Rejeter</button></div></div>
            : <div><h3>Livres proposés</h3>{!detail.candidates.length ? <p>Aucun candidat : rejetez le rattachement.</p> : <fieldset><legend>Choisissez un livre de cette librairie</legend>{detail.candidates.map((candidate: Candidate) => <label className="candidate" key={candidate.bookId}><input type="radio" name="candidate" value={candidate.bookId} checked={chosen === candidate.bookId} onChange={() => setChosen(candidate.bookId)}/><span>{candidate.title}<small>Livre n°{candidate.bookId} · proposition {candidate.rank}</small></span></label>)}</fieldset>}
              <div className="actions"><button className="primary" type="button" disabled={deciding || !chosen} onClick={() => void decideSelected('ACCEPT')}>Rattacher au livre choisi</button><button type="button" disabled={deciding} onClick={() => void decideSelected('REJECT')}>Rejeter le rattachement</button></div>
            </div>}
        </div>}
      </section>;
  return <>
    <a className="skip" href="#content">Aller au contenu</a>
    <header className="top"><a href="/admin">← Administration</a><strong>Defta · Imports de couvertures</strong><button type="button" onClick={logout}>Se déconnecter</button></header>
    <main id="content" className="wrap">
      <div className="intro"><div><p className="eyebrow">DEFTA-LIBRAIRIE · v1.7</p><h1>Importer et revoir les couvertures</h1><p>Déposez un lot, suivez chaque image et choisissez le livre avant tout rattachement.</p></div><span className="badge">{isRoot ? 'SUPER ADMIN ROOT' : 'PROPRIÉTAIRE'}</span></div>
      {isRoot && <label className="scope">Librairie
        <select value={libraryId} onChange={event => {setLibraryId(event.target.value); setSelected(null); setItems([]);}}>
          {owners.map(owner => <option key={owner.library.id} value={owner.library.id}>{owner.library.name} · {owner.username}</option>)}
        </select></label>}
      {error && <p className="alert" role="alert">{error}</p>}
      {notice && <p className="notice" role="status">{notice}</p>}
      <section aria-labelledby="upload-title" className="panel"><div className="section-heading"><h2 id="upload-title">Nouveau lot</h2><small>1 à 100 fichiers · JPEG ou PNG · 10 Mo/image</small></div>
        <div className="drop" onDragOver={event => event.preventDefault()} onDrop={event => {event.preventDefault(); chooseFiles(event.dataTransfer.files);}}>
          <label htmlFor="cover-files">Déposez vos images ici ou choisissez des fichiers</label>
          <input id="cover-files" type="file" accept="image/jpeg,image/png" multiple onChange={event => chooseFiles(event.target.files || [])}/>
          <p>{files.length ? `${files.length} fichier(s) sélectionné(s)` : 'Les images restent privées pendant le traitement.'}</p>
        </div>
        <button className="primary" type="button" disabled={!files.length || !libraryId || uploading} onClick={() => void send()}>{uploading ? 'Envoi en cours…' : 'Importer les couvertures'}</button>
      </section>
      <section aria-labelledby="progress-title" className="panel"><div className="section-heading"><h2 id="progress-title">Suivi des imports</h2><button type="button" onClick={() => void refresh()}>Actualiser</button></div>
        <p className="hint">Actualisation automatique toutes les 3 secondes lorsque cette page est visible.</p>
        {!items.length ? <p>Aucun lot récent dans cette librairie.</p> : items.map(batch => {
          const done = batch.jobs.filter(job => TERMINAL.has(job.status)).length;
          return <article className="batch" key={batch.id}>
            <div className="batch-heading"><strong>Lot du {formatDate(batch.createdAt)}</strong><span>{done}/{batch.totalFiles} décisions finales</span></div>
            <progress value={done} max={batch.totalFiles} aria-label={`Progression du lot : ${done} sur ${batch.totalFiles}`}/>
            <ul className="jobs">{batch.jobs.map((job, index) => <li key={job.id}>
              <span><strong>Image {index + 1}</strong> <small>{jobLabel(job)}</small></span>
              {(job.status === 'REVIEW_REQUIRED' || (isRoot && job.status === 'QUARANTINED')) && <button type="button" onClick={() => setSelected(job)}>Revoir</button>}
              {selected?.id === job.id && reviewContent}
            </li>)}</ul>
          </article>;
        })}
      </section>

    </main>
  </>;
}
