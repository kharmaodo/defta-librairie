# Recette OCR et gate v1.7.1 — US-1716

## État

L’outil et les contrôles techniques sont livrés. **La recette réelle et
l’activation ne sont pas validées.** Le workspace ne contient pas la copie du
catalogue ni le corpus réel annoté nécessaires. Le flag reste `false`, aucun
`.env`, livre ou résultat métier n’est modifié. Ce lot ne publie aucun tag.

Les résultats synthétiques US-1715 valident des cas de recherche ; ils ne
prouvent pas un gain OCR sur des couvertures réelles. Le comparateur Python
US-1712 conserve sa fonction de diagnostic. Pour comparer les politiques Go
réellement branchées, utiliser le nouvel outil Go ci-dessous.

## Préparer les preuves privées

1. Faire une copie cohérente du catalogue avec la procédure SQLite backup
   existante. Ne pas copier directement une base utilisée en WAL. Garder la
   copie, les images et leurs annotations dans `data/ocr-benchmark/`, ignoré par
   Git, avec répertoire `0700` et fichiers `0600`. Le comparateur ouvre la base
   en `mode=ro`, refuse un WAL non vide et contrôle son SHA-256 avant/après.
2. Sélectionner des images réelles représentatives : bruit, reflets, diacritiques,
   titres tardifs, candidats proches et absence réelle de livre au catalogue.
   Ne pas sélectionner seulement les cas réussis ; ne pas multiplier une même
   image. Les SHA-256 dupliqués sont refusés.
3. Annoter chaque image : identifiant neutre, chemin relatif au manifeste,
   SHA-256, bibliothèque active, identifiant exact du livre attendu **ou**
   `expectedAbsent=true`. Les deux choix sont exclusifs. Le livre attendu est
   vérifié dans la bibliothèque et doit être actif. L’absence est une annotation
   humaine ; ne pas la déduire d’un échec de recherche.
4. Transcrire tout le texte OCR visible, et pas seulement le titre, pour les
   images mesurant CER/WER. Les transcriptions et noms privés restent dans le
   manifeste local ; ils ne sont jamais copiés dans le rapport ou les logs.
5. Copier [le modèle](ocr-acceptance-manifest.example.json), remplacer tous les
   placeholders et compléter le corpus. Les deux cas du modèle ne suffisent
   pas. Fixer et faire valider les seuils **avant** la mesure. Les valeurs du
   modèle sont une proposition de recette, pas des critères déjà approuvés.

Proposition initiale : 20 images distinctes, au moins 10 positives, 5 négatives
et 5 transcriptions ; recall@1 ≥ 0,75, recall@5 ≥ 0,90, gain recall@5 ≥ 0,05 ;
aucune augmentation des faux candidats sur les négatifs, CER ≤ 0,20 et WER
≤ 0,35 sans régression ; p95 total ≤ 60 s ; enveloppe mémoire ≤ 1 GiB. Un corpus
plus large et plusieurs conditions CPU restent préférables pour une conclusion
robuste. Les seuils sont explicites dans le manifeste et liés à son empreinte.

## Exécuter les mêmes images dans les deux modes

Prérequis : Go/FTS5, Tesseract 5 et `ara` sur l’hôte, service Python privé prêt
selon son [guide](../services/ocr-experimental/README.md). Utiliser le port de
diagnostic lié à loopback. Le comparateur ne lit pas `.env` et n’active pas
les workers de l’application.

```bash
go build -tags fts5 -o data/ocr-benchmark/ocr-acceptance ./cmd/ocr-acceptance
data/ocr-benchmark/ocr-acceptance \
  --manifest data/ocr-benchmark/manifest.json \
  --db data/ocr-benchmark/catalogue.db \
  --endpoint http://127.0.0.1:8091 \
  --output data/ocr-benchmark/report-first.json
```

La baseline appelle le runner Tesseract Go (`ara`, PSM 6, timeout 20 s) et le
matching v1. L’expérimental appelle le runner HTTP Go (timeout 60 s) puis le
matching v2. Ces délais peuvent être alignés avec le déploiement grâce aux
options `--baseline-timeout-seconds` / `--experimental-timeout-seconds` (1 à
180 s). Les versions Tesseract réelles sont conservées. L’ordre des deux modes
alterne entre les images pour réduire le biais d’un runtime toujours chaud.

Images JPEG/PNG validées, 10 MiB et 24 millions de pixels au maximum. Corpus
1 à 100 images ; texte de référence 4 KiB maximum ; calcul CER/WER refusé au-delà
de 4 096 caractères normalisés. Le matching conserve les budgets US-1715.
Une annotation, empreinte ou copie invalide arrête la comparaison avec code 2.
Les erreurs runtime sont des codes fixes dans le rapport et bloquent le gate.

## Mesures et mémoire

Le rapport JSON `0600` est créé exclusivement, sans écrasement. Il contient
empreintes corpus/catalogue/images, timestamps, seuils, versions, identifiants
candidats et agrégats. Aucun texte OCR, transcription, chemin d’image, URL privée
ou secret. Ne pas publier ce rapport : les identifiants restent des données
privées. Le stdout contient uniquement la décision.

- Recall@1/5 : livre attendu en première position / dans les cinq ; toute
  erreur runtime compte comme échec, pas comme échantillon omis.
- Faux candidats : proportion des négatifs qui reçoivent au moins un candidat.
  Cela ne signifie pas que tous les livres alternatifs des positifs sont faux.
- Absence de candidat : proportion de traitements réussis sans candidat ;
  distincte des pannes. Les dénominateurs sont conservés.
- CER/WER : distance de Levenshtein sur caractères Unicode / mots ; agrégation
  pondérée par longueur de référence, espaces harmonisés, diacritiques conservés.
  Les erreurs runtime ne fournissent pas une transcription réussie et bloquent
  le gate. Une insertion peut produire un taux supérieur à 1.
- `titleLineExactRate` : proportion des positifs dont une ligne OCR correspond
  exactement au titre catalogue après normalisation US-1715. C’est une présence
  de titre en ligne, pas un extracteur de titre ni une correction fuzzy ; les
  titres répartis sur plusieurs lignes ne satisfont pas cette mesure.
- p50/p95 : nearest-rank sur la durée extraction + matching + calcul des métriques.
  La lecture/validation de l’image est commune et exclue. Ce petit corpus ne
  constitue pas un engagement de latence de production.

La mémoire du service distant ne peut pas être déduite du client HTTP. Elle
n’est jamais remplacée par la mémoire Go ni par la limite du conteneur. Collecter
une preuve sur toute la fenêtre de mesure, avec image Docker identifiée par
son digest. Relever les pics du groupe Go et de ses enfants (Tesseract) et du
conteneur Python, par cgroup v2 ou mesure équivalente documentée localement.
Les `memory.peak` cgroup v2 sont préférables aux instantanés `docker stats` qui
peuvent manquer un pic. Un conteneur fresh évite d’inclure une ancienne recette.

Dans un fichier mémoire privé, `baselinePeakBytes` couvre Go + Tesseract,
`experimentalPeakBytes` couvre Go + service Python ; `peakBytes` est le maximum
des deux. La somme des pics individuels est une enveloppe conservatrice quand
les pics ne sont pas simultanés. Le protocole est nommé `combined-process-peaks`.
Documenter les compteurs sources et leurs unités dans la preuve locale. **Aucune
valeur d’exemple ne doit être présentée comme mesurée.**

Format de preuve (remplacer les placeholders et valeurs nulles par les mesures) :

```json
{
  "corpusSHA256": "EMPREINTE_DU_RAPPORT",
  "catalogueSHA256": "EMPREINTE_DU_RAPPORT",
  "method": "combined-process-peaks",
  "baselinePeakBytes": null,
  "experimentalPeakBytes": null,
  "peakBytes": null,
  "serviceImageDigest": "sha256:DIGEST_REEL",
  "startedAt": "DEBUT_COUVRANT_LA_MESURE_UTC",
  "endedAt": "FIN_COUVRANT_LA_MESURE_UTC"
}
```

Le gate vérifie les empreintes, la fenêtre couvrante, le format du digest,
les pics positifs et leur maximum. Cette preuve demeure une attestation locale
à examiner humainement ; le JSON ne prouve pas à lui seul comment la mémoire
a été mesurée. Finaliser sans relancer l’OCR :

```bash
data/ocr-benchmark/ocr-acceptance \
  --evaluate data/ocr-benchmark/report-first.json \
  --memory-evidence data/ocr-benchmark/memory.json \
  --output data/ocr-benchmark/report-reviewed.json
```

Codes : **0** = `READY_FOR_HUMAN_REVIEW`, **3** = `BLOCKED` avec raisons dans le
rapport, **2** = erreur de preuve/configuration/sortie. Sans mémoire, corpus
réel suffisant, transcriptions, gain ou budgets satisfaits, le gate bloque.
Il recalcule les métriques à partir des records, sans faire confiance aux
agrégats ou à la décision enregistrée. Ne jamais contourner code 3 avec
`|| true` pour déclarer une recette réussie.

## Contrôles techniques et CI

```bash
sh scripts/check-release-v1.7.1.sh
```

Ce script conserve le gate v1.7.0 (Go, race, frontend, Playwright, build et
configuration Docker), puis vérifie Python et le conteneur OCR isolé, l’adaptateur
HTTP Go et le comparateur avec de vrais runners sur images **synthétiques**.
Installer les dépendances Python du service dans l’environnement utilisé par
`python3`, et les dépendances npm/Chromium du gate existant. Le test local utilise
le port loopback 8092 et nettoie son propre service après exécution.

La CI compose les workflows existants : gate release et intégration
MinIO/JetStream ; interface d’import ; OCR Python/Docker avec le nouveau smoke
comparateur. Les tests vérifient rollback, idempotence, isolation, off/on,
absence de mutation de la copie SQLite, refus d’écrasement et confidentialité.
La CI technique verte ne clôture pas la recette réelle.

## Décision et rollback

Après seuils approuvés, preuves réelles et contrôles verts, soumettre le rapport
privé, le corpus annoté et le protocole mémoire à la revue du propriétaire.
**READY_FOR_HUMAN_REVIEW n’autorise aucune activation automatique.** L’activation
par défaut exige la validation explicite prévue par le contrat v1.7.1.

Si approuvé : sauvegarde, vérification des probes, activation explicite du flag
et observation des imports. En cas de panne ou de qualité insuffisante : remettre
`OCR_EXPERIMENTAL_ENABLED=false`, redémarrer et suivre les jobs pending. Les baux
actifs sont respectés ; ceux expirés reviennent au runner local. Le matching
redevient v1 ; les résultats et candidats déjà persistés restent disponibles.
Ne pas annuler la migration 035, supprimer les audits ou contourner la revue.
Voir [reprise et rollback US-1714](OCR_GO_ADAPTER_V1_7_1.md).
