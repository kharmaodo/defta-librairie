# Service local de modération d'images

Ce conteneur reçoit une image JPEG ou PNG en mémoire et retourne une décision
technique normalisée. Il ne connaît ni SQLite, ni MinIO, ni NATS, ni secret de
l'application principale.

Le modèle ONNX n'est pas dans le dépôt ni dans l'image. Monter un répertoire
d'administration en lecture seule :

```text
/opt/models/nsfw/
├── model.onnx
└── model-manifest.json
```

Le manifeste est le contrat de prétraitement du modèle et doit contenir son
versionnement et son SHA-256. Au démarrage, le service refuse de devenir prêt si
le hash ne correspond pas, si les tenseurs annoncés sont absents ou si le modèle
n'est pas lisible.

Copier et compléter `model-manifest.example.json`, puis démarrer le service :

```bash
docker compose -f services/nsfw-moderator/compose.nsfw-moderator.yaml up --build
curl -fsS http://localhost:8090/health/ready
```

Le service expose :

- `GET /health/live` : processus démarré ;
- `GET /health/ready` : modèle vérifié et session ONNX chargée ;
- `POST /v1/moderate` : corps binaire JPEG/PNG, réponse
  `{"class":"SAFE|REVIEW|UNSAFE","score":0..1,"modelVersion":"..."}`.

Les seuils par défaut sont 0,20 et 0,75. Ils fournissent une décision technique,
pas une décision éditoriale : la zone `REVIEW` doit rester dans le flux de
revue humaine et les seuils doivent être calibrés sur un jeu de validation
représentatif avant mise en production.

Exemple :

```bash
curl --fail-with-body -sS -X POST http://localhost:8090/v1/moderate \
  -H 'Content-Type: image/jpeg' --data-binary @cover.jpg
```
