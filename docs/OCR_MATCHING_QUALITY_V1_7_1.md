# Matching de qualité et isolation — US-1715

Cette politique reste expérimentale : `OCR_EXPERIMENTAL_ENABLED=false` conserve
le matching existant (douze premiers tokens, index catalogue global filtré par
bibliothèque). On active le nouveau matching avec le même flag que l’adaptateur
OCR US-1714. Aucune activation par défaut ni modification du rattachement humain.
La recherche manuelle US-1717 conserve son parcours existant.

## Politique v2

Le Go charge uniquement les livres actifs de la bibliothèque autorisée. Il
normalise les champs titre, éditeur, auteur, tags et catégorie et le texte OCR
avec la même politique : suppression des diacritiques/tatweel, lettres et chiffres,
minuscules ; conservation de l’ordre Unicode arabe, sans inversion ni folding
systématique des alefs. Les données du catalogue et le texte OCR ne sont pas
réécrits.

Les mots exacts utiles sont recherchés dans **tout** le texte OCR, même après
le bruit initial. Les répétitions n’augmentent pas leur poids. Un mot inconnu
long d’au moins cinq caractères, sans chiffres, peut être remplacé à une distance d’une édition
(insertion, suppression ou substitution), seulement si un unique mot du
catalogue local convient. Une ambiguïté entraîne l’abstention. Cette correction
sert à construire la requête ; elle ne modifie ni titre ni OCR enregistré.

Le classement utilise réellement FTS5/BM25, avec poids titre 5, éditeur 1,
auteur 2, tags/catégorie 0,5. L’index est un snapshot temporaire **en mémoire**,
sur une connexion dédiée, contenant uniquement cette bibliothèque. Ainsi les
statistiques BM25, les scores, la sélection des mots rares et le dictionnaire
ne dépendent pas d’un catalogue étranger. L’index est supprimé après le calcul
et le réglage SQLite temporaire est restauré. L’index permanent reste inchangé.

Les tokens sont cités et la requête est paramétrée ; les opérateurs issus de
l’OCR ne sont jamais interprétés. Les cinq meilleurs candidats sont déterministes
(score croissant, puis identifiant). Ce score est un rang BM25, pas une probabilité
ni une distance d’édition. L’explication et l’audit portent
`algorithm=scoped_fts5_bm25`, `policyVersion=v2`, sans contenu OCR.

La transaction existante recontrôle la bibliothèque et les livres supprimés,
persiste les candidats et leur audit, puis passe à `REVIEW_REQUIRED`, même avec
zéro candidat. Aucun livre n’est modifié. Une panne technique n’est jamais
convertie en absence de candidat ; les reprises JetStream existantes s’appliquent.

## Budgets de calcul

| Ressource | Limite v2 |
|---|---|
| Texte OCR | 1 MiB |
| Catalogue actif d’une bibliothèque | 10 000 livres |
| Données des champs catalogue | 8 MiB cumulés, 64 KiB par champ |
| Dictionnaire | 20 000 mots distincts |
| Mot utilisable | 2 à 64 caractères Unicode |
| Corrections | 32 mots inconnus distincts, tri lexical ; mots de 5 caractères minimum |
| Requête | 128 mots distincts, priorité aux mots rares puis tri lexical |
| Recherche | Deadline de 15 secondes, respect du contexte appelant |
| Candidats | 5 |

Les limites du catalogue provoquent une erreur technique explicite, sans utiliser
un dictionnaire partiel. Les mots exacts sont collectés sur tout le texte avant
le budget de correction ; au-delà de 128 preuves utiles, les plus rares sont
retenues. Ce bornage peut réduire le rappel sur un OCR extrêmement bruité ou un
catalogue très volumineux : vérifier ces cas avant activation. Le rollback du
flag rétablit la politique v1 pour les prochaines recherches ; les candidats
v2 déjà enregistrés restent disponibles à la revue.

## Benchmark annoté reproductible

Fixture synthétique publique dans
`internal/repositories/cover_import_matching_quality_test.go`. Le catalogue
contient des distracteurs partageant « تفسير », une autre bibliothèque et un
livre supprimé. Aucun fichier de couverture ni catalogue privé n’est ajouté.

| OCR synthétique | Livre attendu |
|---|---|
| تفسير القرآن العظيم | 2 |
| 40 mots de bruit, puis صحيح البخاري | 3 |
| البداية والنهاية, catalogue avec diacritiques | 1 |
| تفسير القرآم العظيم | 2 |
| البدايه, seule preuve avec une substitution | 1 |
| العذب الفائض شرح عمدة الفارض | 5 |
| مجهول تماما | Aucun |
| كتاب محذوف | Aucun, livre supprimé |

Mesure locale sur les six cas positifs : baseline recall@1/5 **4/6**, v2 recall@1
**6/6**, recall@5 **6/6** ; aucun candidat sur les deux cas négatifs. Un test
séparé refuse une correction ambiguë et vérifie que l’ajout de mots proches
étrangers ne change ni candidat ni score. Un catalogue synthétique de 1 001
livres est aussi testé ; une exécution locale a pris environ 36 ms, valeur
indicative sans engagement de performance.

Reproduire :

```bash
go test -tags fts5 ./internal/repositories -run QualityMatching -v
go test -tags fts5 ./internal/services -run MatchingQuality -v
```

Ces résultats mesurent la recherche à partir de textes annotés, **pas** la qualité
de l’extraction sur des images réelles. CER/WER, recall réel, faux candidats,
latence p50/p95 et mémoire sur le corpus privé restent à mesurer dans le suivi
OCR arabe avant activation générale. US-1716 est clôturée pour son outillage ;
la recette réelle n’est pas validée.
L’activation reste off en attendant cette mesure. Les tests couvrent aussi
le mode off/on, le scope, les doublons, le rollback de persistance et le nettoyage
des index temporaires ; le gate complet conserve les parcours navigateur.
