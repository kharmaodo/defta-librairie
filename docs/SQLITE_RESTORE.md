# Restauration SQLite — Defta Librairie

La restauration prépare un **nouveau fichier** avec l’API de sauvegarde SQLite.
Elle ne remplace jamais la base active. Utiliser Linux/WSL, Python 3.10 ou plus
avec SQLite et FTS5, et un répertoire local permettant les liens physiques.
La base restaurée reçoit les permissions 0600.

## 1. Choisir et préparer la sauvegarde

Identifier la sauvegarde, sa date, la version du code qui l’a produite et, si
elle a été conservée, son empreinte SHA-256. Une restauration perd les opérations
postérieures à cette sauvegarde : valider ce point de reprise avec l’exploitant.
L’empreinte affichée par le script décrit le fichier restauré ; elle ne prouve
pas l’authenticité de la source et peut différer de celle de la sauvegarde.

Conserver la sauvegarde originale. Ne pas éditer les migrations déjà appliquées.
Les sauvegardes de versions futures du schéma ne doivent pas être ouvertes par
une version antérieure de l’application. En cas de doute, utiliser d’abord la
version de code correspondant à la sauvegarde.

```bash
mkdir -p ./data/restores
chmod 700 ./data/restores

python3 scripts/restore-db.py ./data/backups/SAUVEGARDE.db \
  --output ./data/restores/defta-restored-20260910.db
```

Remplacer le nom de sauvegarde et choisir un nom de destination neuf à chaque
tentative. Le répertoire parent doit exister. Les fichiers destination et ses
éventuels `-wal`, `-shm`, `-journal` sont refusés s’ils existent déjà, y compris
un lien symbolique. Aucun argument `--force` n’est disponible.

Le script ouvre la source en lecture seule, réalise une copie SQLite cohérente
(incluant les pages WAL validées), puis contrôle :

- `PRAGMA integrity_check` ;
- `PRAGMA foreign_key_check` ;
- présence des tables de catalogue, utilisateurs, librairies et migrations ;
- présence d’au moins une migration enregistrée.

Le fichier n’est publié qu’après ces contrôles, sans écrasement. Le transfert
SQLite est interrompu s’il dépasse deux minutes ; les contrôles qui suivent
ne sont pas soumis à cette limite. Le résultat JSON contient `output`, `sha256`
et `migrations`. Une erreur entraîne un code de sortie non nul. Si une erreur
de synchronisation survient après publication, le message indique que la
destination éventuelle doit être vérifiée avant utilisation.

## 2. Bascule en maintenance

1. Suspendre les opérations métier et arrêter proprement **toutes les instances**
   de l’application (Ctrl+C au premier plan, ou arrêt du service qui l’héberge).
   Vérifier qu’aucun processus n’utilise la base. Le script de restauration ne
   réalise pas cet arrêt à votre place.
2. Conserver le fichier actif et ses fichiers associés. Si la base active est
   lisible, exécuter `scripts/backup-db.sh` avec son `DB_PATH` pour conserver un
   point de retour. Si elle est endommagée, en préserver les fichiers pour
   diagnostic ; ne pas les supprimer pour contourner un échec de sauvegarde.
3. Noter la valeur précédente de `DB_PATH`. Modifier **seulement DB_PATH** dans
   `.env` pour utiliser le chemin absolu du nouveau fichier restauré. Vérifier
   aussi les variables du service/systemd/conteneur : une variable d’environnement
   existante peut prendre le pas sur `.env`. Conserver JWT_SECRET et les autres
   paramètres selon la configuration de la sauvegarde et du déploiement.
4. Redémarrer avec la version de code compatible. L’application crée sa sauvegarde
   avant migrations, contrôle leurs checksums puis applique les migrations manquantes.
   Ne pas lancer simultanément l’ancienne et la nouvelle base pour servir du trafic.

L’application reconstitue ses propres fichiers WAL/SHM au redémarrage. Ne pas
copier les anciens fichiers associés à côté du nouveau fichier restauré.
Ne pas démarrer l’application simplement sur un chemin inexistant : elle pourrait
créer une base vide au lieu d’utiliser la restauration attendue.

## 3. Recette avant réouverture

```bash
curl --fail-with-body -sS http://localhost:8080/api/health/live
curl --fail-with-body -sS http://localhost:8080/api/health/ready
```

Attendre `alive` et `ready`, vérifier les logs de démarrage/migrations et confirmer
le chemin utilisé. Avec un compte autorisé, comparer à la sauvegarde quelques
références connues de livres, stocks, ventes, achats et paiements. Ouvrir une
fiche et un reçu sans créer de fausse vente en production. Vérifier les droits
propriétaire/root. Les comptes, mots de passe et sessions sont ceux de la date
de sauvegarde : ne pas supposer qu’un changement postérieur a été conservé.

Enregistrer dans le compte rendu : source/date, commit applicatif, destination,
empreinte produite, durée réelle de l’arrêt, contrôles et opérateur. La réussite
des sondes ne remplace pas cette recette métier. La réouverture du trafic valide
la bascule. Les objectifs de délai de reprise (RTO), perte acceptable (RPO),
fréquence, conservation et copie hors machine des sauvegardes restent à définir
avec l’exploitant ; aucune valeur n’est présumée.

## 4. Retour arrière

Avant toute nouvelle écriture métier, un échec de recette permet d’arrêter le
serveur et de rétablir l’ancien DB_PATH et sa version de code compatible, en
conservant les fichiers restaurés pour diagnostic. Redémarrer et refaire la
recette. Si des opérations ont déjà été enregistrées sur la base restaurée,
un retour direct perdrait ces opérations : arrêter le trafic et organiser leur
réconciliation avant de revenir à l’ancienne base.

## 5. Recette automatisée isolée

```bash
python3 scripts/test-restore-db.py
```

Six tests utilisent exclusivement des bases temporaires et les migrations du
dépôt : copie d’une écriture validée encore dans le WAL, empreinte et permissions,
refus d’écrasement, refus de fichiers associés/liens, source corrompue, violation
de clé étrangère et schéma étranger. Aucun accès à `data/defta.db` ni au serveur.
Cette recette ne certifie pas les temps de reprise d’une base de production ;
une répétition sur une sauvegarde représentative reste nécessaire à l’exploitation.
