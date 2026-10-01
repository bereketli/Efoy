# Staging deployment

Staging runs on a single-node [k3s](https://k3s.io) cluster. Argo CD syncs this
repository; every push to `main` that passes CI deploys automatically.

```
push to main ─► CI builds and pushes ghcr.io/bereketli/efoy-<service>:<sha>
             └► CI commits image.tag=<sha> to deploy/helm/efoy/values-staging.yaml
                  └► Argo CD syncs: migrate job (PreSync) ─► rolling update
```

| Path | What |
| --- | --- |
| `helm/efoy` | The five Go services and the web console, plus the migration job and the Ingress |
| `helm/efoy-deps` | Staging data stores: Postgres 16 + PostGIS + TimescaleDB, Redis, NATS JetStream, MinIO |
| `argocd/staging.yaml` | The two Argo CD applications |
| `k3s/traefik-config.yaml` | Let's Encrypt TLS and the HTTP→HTTPS redirect for k3s's bundled Traefik |
| `secrets/` | SOPS-encrypted Secrets (`*.enc.yaml`) and the template |

## One-time setup

You need `kubectl`, `sops` and `age` locally, and a server with a public IP.

1. **DNS:** point `api.staging.efoy.et`, `rt.staging.efoy.et` and
   `console.staging.efoy.et` at the server.

2. **k3s and TLS:**
   ```sh
   curl -sfL https://get.k3s.io | sh -
   sudo cp deploy/k3s/traefik-config.yaml /var/lib/rancher/k3s/server/manifests/
   ```
   The config targets the Traefik chart that ships with k3s 1.32 and later.
   Copy `/etc/rancher/k3s/k3s.yaml` to your machine as your kubeconfig.

3. **Argo CD:**
   ```sh
   kubectl create namespace argocd
   kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
   ```
   If the repository is private, add it in Argo CD (Settings → Repositories)
   with a read-only deploy key.

4. **Secrets (SOPS + age):**
   ```sh
   age-keygen -o ~/.config/sops/age/keys.txt   # once per person; share the public key
   ```
   Put the team's age public keys in `.sops.yaml`, then:
   ```sh
   cp deploy/secrets/staging.example.yaml deploy/secrets/staging.enc.yaml
   make gen-keys          # paste the signing key and TOTP key into the file
   # fill in the passwords
   sops --encrypt --in-place deploy/secrets/staging.enc.yaml
   kubectl create namespace efoy-staging
   make staging-secrets   # decrypt and apply
   git add deploy/secrets/staging.enc.yaml
   ```
   Only the encrypted file is committed; `.gitignore` blocks anything else in
   `deploy/secrets/`. Re-run `make staging-secrets` after every change.

5. **Image pull secret:** GHCR packages are private by default.
   ```sh
   kubectl -n efoy-staging create secret docker-registry ghcr-pull \
     --docker-server=ghcr.io --docker-username=<github user> --docker-password=<PAT with read:packages>
   ```

6. **Applications:**
   ```sh
   kubectl apply -n argocd -f deploy/argocd/staging.yaml
   ```
   Argo CD creates the data stores first, then runs `core-api migrate up` and
   starts the services.

7. **First console account:**
   ```sh
   kubectl -n efoy-staging exec deploy/efoy-core-api -- /app create-staff \
     --email you@efoy.et --name "Your Name" --role SUPER_ADMIN
   ```
   It prints a one-time password and a TOTP secret/URL for an authenticator app.

## Day to day

- **Deploys** happen on every green push to `main`. To roll back, revert the
  `deploy(staging): …` commit (or the change itself) on `main`.
- **OTP codes:** until the SMS gateway is configured (day 5), staging uses the
  console SMS fake. Read codes with
  `kubectl -n efoy-staging logs deploy/efoy-core-api | grep sms`.
- **Rotating the signing key:** run `make gen-keys`. Set the new key as
  `EFOY_AUTH__SIGNING_KEY` with a new `EFOY_AUTH__SIGNING_KEY_ID`. Move the old
  public key and id to `EFOY_AUTH__PREVIOUS_PUBLIC_KEY` and
  `EFOY_AUTH__PREVIOUS_KEY_ID` so tokens already issued stay valid.
- **Never change `EFOY_AUTH__TOTP_KEY`** without re-enrolling every staff
  user's TOTP: it encrypts their secrets.

## Not here yet

- No OSRM in staging until route building needs it (day 6).
- No monitoring stack yet (day 18).
- Production uses CloudNativePG instead of the single Postgres pod (day 19).
