# Infrastructure locale des couvertures

Cette pile démarre les dépendances de la future chaîne de couvertures `v1.4.0`.
Elle ne démarre pas encore l’API ni le worker.

## Préparer les secrets

Copier les variables utiles dans le fichier local `.env` puis générer des
valeurs distinctes :

```sh
cp .env.example .env
minio_key=$(openssl rand -hex 16)
minio_secret=$(openssl rand -base64 36)
nats_user=defta-covers
nats_password=$(openssl rand -base64 36)

sed -i "s|^MINIO_ACCESS_KEY=.*$|MINIO_ACCESS_KEY=$minio_key|" .env
sed -i "s|^MINIO_SECRET_KEY=.*$|MINIO_SECRET_KEY=$minio_secret|" .env
sed -i "s|^NATS_USER=.*$|NATS_USER=$nats_user|" .env
sed -i "s|^NATS_PASSWORD=.*$|NATS_PASSWORD=$nats_password|" .env
chmod 600 .env
```

Ne jamais commiter `.env`.

## Démarrer et contrôler

```sh
docker compose --env-file .env -f deploy/docker-compose.covers.yml config
docker compose --env-file .env -f deploy/docker-compose.covers.yml up -d
docker compose --env-file .env -f deploy/docker-compose.covers.yml ps
docker compose --env-file .env -f deploy/docker-compose.covers.yml logs minio-init
```

En développement seulement :

- API MinIO : `http://127.0.0.1:9000` ;
- console MinIO : `http://127.0.0.1:9001` ;
- client NATS : `nats://127.0.0.1:4222` ;
- supervision NATS : `http://127.0.0.1:8222`.

Les liaisons sont limitées à la boucle locale par défaut. Un déploiement de
production doit utiliser un réseau privé, TLS, un gestionnaire de secrets et ne
pas publier les consoles d’administration.

## Arrêter

```sh
docker compose --env-file .env -f deploy/docker-compose.covers.yml down
```

Cette commande conserve les volumes. Leur suppression détruit les objets et les
messages persistés et ne doit être réalisée qu’après sauvegarde et validation
explicite.
