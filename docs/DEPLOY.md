# Deploying Face Value

Production is a Forge VPS shared with the other Go apps. One Postgres cluster, one Go process per app, and nginx serving the frontend as static files. Platform rules (ports, build lock, backups) are [face-value#18](https://github.com/isAdamBailey/face-value/issues/18).

The previous server's PM2 layout is [DEPLOY-old.md](./DEPLOY-old.md). That site is stopped, and its push-to-deploy stays off.

**Repo:** https://github.com/isAdamBailey/face-value
**Domain:** `face-value.adambailey.io`
**Go API port:** `8101`

## Architecture

```
https://face-value.adambailey.io
         │
         ▼
      Nginx (Forge-managed)
         ├─ /api/*, /healthz  → 127.0.0.1:8101  (Go daemon)
         └─ /*                → frontend/.output/public  (static files)

Postgres → 127.0.0.1:5432, database facevalue
Images   → Amazon S3 (private bucket, presigned GETs)
Email    → SES
```

Cookie auth needs this one domain. Production API calls are same-origin (`/api/...`), so `NUXT_PUBLIC_API_BASE` is unset and `nuxt generate` bakes in an empty base.

---

## 1. Postgres role and database

Postgres 18 is already installed on the new server, listening on localhost. Create the app role and database once. Type the password at the prompt.

```sh
sudo -u postgres createuser --pwprompt facevalue
sudo -u postgres createdb -O facevalue facevalue
sudo -u postgres psql -c "REVOKE CONNECT ON DATABASE facevalue FROM PUBLIC; GRANT CONNECT ON DATABASE facevalue TO facevalue;"
```

`DATABASE_URL` is `postgres://facevalue:PASSWORD@127.0.0.1:5432/facevalue?sslmode=disable`.

The owning role creates `pgcrypto` from migration `000001`. No superuser is required at deploy time.

Go is installed at `/usr/local/go` (1.26.x). The deploy script adds it to `PATH`.

---

## 2. Forge site

1. **Sites → New Site** → `face-value.adambailey.io`.
2. Project type: a non-PHP site with zero-downtime (Static HTML, or whatever Forge currently calls a site that is not PHP). Branch `main`. Push-to-deploy is on: a push to `main` deploys this site.
3. Repository: `isAdamBailey/face-value`.
4. Replace the deploy script with:

```bash
$CREATE_RELEASE()

cd "$FORGE_RELEASE_DIRECTORY"
bash scripts/forge-deploy.sh

$ACTIVATE_RELEASE()

sudo supervisorctl restart daemon-1180520:*
```

`1180520` is the live face-value daemon.

`scripts/forge-deploy.sh` takes `/tmp/node-build.lock`, builds the Go API with `GOFLAGS=-p=2`, runs migrations, and runs `npm run generate`. Nginx serves `frontend/.output/public`. There is no Node process.

---

## 3. Environment

Forge → site → **Environment**. The file is `/home/forge/face-value.adambailey.io/.env`.

Copy the old site's environment, then change only:

| Variable | Production |
| --- | --- |
| `DATABASE_URL` | `postgres://facevalue:PASSWORD@127.0.0.1:5432/facevalue?sslmode=disable` |
| `PORT` | `8101` |
| `APP_BASE_URL` | `https://face-value.adambailey.io` |
| `COOKIE_SECURE` | `true` |
| `EMAIL_PROVIDER` | `ses` |
| `SMTP_PORT` | `2587`. This server cannot open 587 or 465; SES answers on 2587. |

`COOKIE_SIGNING_SECRET` must stay the same as the old server or the existing user is logged out. Remove `FORGE_API_DAEMON` and any `NUXT_*` port variables. Omit `NUXT_PUBLIC_API_BASE`, `S3_ENDPOINT`, and `S3_PUBLIC_ENDPOINT`.

S3 bucket setup for images is unchanged; the commands are in [DEPLOY-old.md](./DEPLOY-old.md) §3.

---

## 4. Go API daemon

Forge → **Background processes** → Custom.

- Directory: `/home/forge/face-value.adambailey.io/current`
- User: `forge`
- Command: `bash scripts/run-api.sh`

`PORT` comes from `.env`. The live daemon is `daemon-1180520`. Supervisor starts it after a reboot.

---

## 5. Nginx

Site settings: `root /home/forge/face-value.adambailey.io/current/frontend/.output/public;`.

Before the catch-all:

```nginx
client_max_body_size 12M;

location /api/ {
    proxy_pass http://127.0.0.1:8101;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 120s;
}

location = /healthz {
    proxy_pass http://127.0.0.1:8101;
}

location /_nuxt/ {
    expires 1y;
    add_header Cache-Control "public, immutable";
    try_files $uri =404;
}

location / {
    try_files $uri $uri/ /200.html;
}
```

`sudo nginx -t`, then reload.

`try_files $uri $uri/` redirects a generated page such as `/auth/callback` to `/auth/callback/`. The auth middleware strips a trailing slash before the public-route check, so the magic link still signs the user in.

---

## 6. Check a deploy

The deploy log should show the build lock, `go build`, migrate, and `npm run generate`, then the daemon restart.

```sh
curl -s 127.0.0.1:8101/healthz
curl -s -H 'Host: face-value.adambailey.io' 127.0.0.1/healthz
curl -s -H 'Host: face-value.adambailey.io' 127.0.0.1/ | head
```

The first is the daemon. The second is nginx proxying `/healthz`. The third is the generated HTML.

---

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| 502 on `/api` | `sudo supervisorctl status`. Run `bash scripts/run-api.sh` from `current` for the real error. |
| Daemon restart fails | `sudo supervisorctl restart daemon-<id>:*` — the `:*` suffix matches the process group. |
| `bind: address already in use` | `sudo supervisorctl stop daemon-<id>:*` before killing the port. `ss -tlnp` shows what holds `8101`. |
| 413 on upload | `client_max_body_size` missing from nginx. |
| Upload 504s | `proxy_read_timeout` too low. |
| Every upload fails, works locally | `S3_ENDPOINT` or `S3_PUBLIC_ENDPOINT` is still set. Omit both. |
| Login redirect broken | `APP_BASE_URL` must be `https://face-value.adambailey.io`. |
| Magic link opens the login screen | Nginx added a trailing slash and the public-route check missed `/auth/callback/`. The auth middleware strips that slash. |
| Magic link never arrives | `SMTP_PORT` must be `2587`. Connections to 587 and 465 time out from this server. |
| Deploy cannot find `.env` | The file belongs at the site root, `/home/forge/face-value.adambailey.io/.env`. |

eBay and Anthropic credentials: [EBAY_SETUP.md](./EBAY_SETUP.md), [ANTHROPIC_SETUP.md](./ANTHROPIC_SETUP.md).
