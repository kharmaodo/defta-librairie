# Recette frontend — accessibilité des dialogues

Cet incrément nomme les 20 dialogues de l’administration, leurs boutons de
fermeture représentés par ×, les messages d’erreur et les en-têtes de colonnes.
Il ajoute un lien d’évitement vers le contenu et un focus clavier visible.
Il conserve le comportement natif de showModal/close et les protections existantes.
Ce travail ne constitue pas un audit complet de conformité WCAG.

## Contrôle structurel

```bash
python3 scripts/check-admin-accessibility.py
node --check static/js/admin-supplier-returns.js
```

Le contrôle lit le template et le bloc HTML dynamique du module des retours
fournisseurs. Il détecte les identifiants dupliqués, titres de dialogues absents,
boutons × non nommés, erreurs sans rôle alert et en-têtes sans scope. Il ne
simule ni le navigateur ni un lecteur d’écran et ne couvre pas les futurs blocs
HTML dynamiques sans adaptation du contrôle.

## Recette clavier avec un compte autorisé

1. Recharger /admin, puis appuyer sur Tab : le lien « Aller au contenu principal »
   doit apparaître avec un contour visible. Entrée mène au contenu principal.
2. Naviguer avec Tab et Maj+Tab entre les contrôles : le focus doit rester visible.
3. Ouvrir un formulaire (livre, client, fournisseur, caisse) au clavier.
   Vérifier que le focus se trouve dans le dialogue et ne traverse pas l’arrière-plan.
4. Fermer avec le bouton nommé ou Échap lorsque la fermeture est autorisée.
   Vérifier le retour du focus au déclencheur si celui-ci existe encore.
5. Tester le dialogue de retour fournisseur et l’historique client : leur titre
   doit être annoncé par le lecteur d’écran, même après modification du titre.
6. Déclencher une erreur de validation sans modifier de données métier : vérifier
   sa visibilité et son annonce. Les validations HTML natives peuvent bloquer
   l’envoi avant l’apparition d’un message serveur ; elles restent attendues.
7. Si un changement de mot de passe est obligatoire, vérifier que la fermeture
   demeure bloquée selon la règle existante ; ne pas contourner ce contrôle.
8. Ouvrir un reçu de vente et un bon d’achat : vérifier le titre annoncé et
   l’aperçu d’impression, où le lien d’évitement ne doit pas apparaître.

Répéter les parcours utiles avec propriétaire/root et à 200 % de zoom. Noter
navigateur, lecteur d’écran éventuel et anomalies. Les tests navigateur
automatisés, les erreurs globales et la poursuite du découpage JavaScript
restent au backlog de la priorité 12.
