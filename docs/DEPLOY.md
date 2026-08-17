# Deploying Face Value

Production setup: a **VPS managed by [Laravel Forge](https://forge.laravel.com)**,
PostgreSQL on the same server, Nuxt via PM2, Go API as a Forge daemon, and images
in a private S3 bucket. Same shape as `isAdamBailey/massa` — see that repo's
`docs/DEPLOY.md` if you're maintaining both side by side.

**Repo:** https://github.com/isAdamBailey/face-value

## Architecture

```
https://facevalue.example.com
         │
         ▼
      Nginx (Forge-managed)
         ├─ /api/*, /healthz  → 127.0.0.1:8080  (Go daemon)
         └─ /*                → 127.0.0.1:3005  (Nuxt via PM2)

Postgres → 127.0.0.1 on the VPS
Images   → Amazon S3 (private bucket, presigned GETs straight to the browser)
Email    → your existing SMTP provider (e.g. SES)
```

Because images live in S3, the VPS is fully stateless apart from Postgres —
nothing in the release directory needs to survive a deploy.

Cookie auth requires **one domain**. Nginx proxies `/api` to Go; everything else
goes to Nuxt. Production builds use same-origin API requests (`/api/...`), so
`NUXT_PUBLIC_API_BASE` is omitted in production.

---

## 1. Server prerequisites

Use an existing Forge server (e.g. the one running massa) or provision a new one.

### PostgreSQL

Install Postgres on the VPS if Forge doesn't provide it, or reuse the instance
massa already set up — just add the new role/database:

```sh
sudo apt update
sudo apt install -y postgresql postgresql-contrib   # skip if already installed
sudo -u postgres createuser --pwprompt facevalue
sudo -u postgres createdb -O facevalue facevalue
sudo -u postgres psql -d facevalue -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"
```

Note `DATABASE_URL` for step 4, e.g.
`postgres://facevalue:PASSWORD@127.0.0.1:5432/facevalue?sslmode=disable`.

### Go

Skip if already installed for massa on this server.

```sh
uname -m   # x86_64 → amd64, aarch64 → arm64
cd /tmp
curl -LO https://go.dev/dl/go1.26.4.linux-amd64.tar.gz   # or linux-arm64
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.4.linux-amd64.tar.gz
/usr/local/go/bin/go version
```

The deploy script adds `/usr/local/go/bin` to `PATH`.

---

## 2. Create the Forge site

1. **Sites → New Site** → your domain (e.g. `facevalue.example.com`).
2. Project type: **Nuxt**. Mode: **Node.js Server**, not Static Site — the
   app builds with `nuxt build` (not `nuxt generate`) and needs a running
   Node process for Forge's Nginx to proxy to.
3. **Web directory:** `frontend`. Unlike massa's non-zero-downtime setup,
   this matters here: Forge's auto-generated PM2 config for zero-downtime
   Nuxt sites assumes `.output/server/index.mjs` sits directly under
   `current/` — this monorepo builds it to `frontend/.output/...` instead,
   so the PM2 block's `cwd` needs `/frontend` appended (see below). The Go
   backend and deploy script still live at the repo root regardless of this
   setting; it only affects Forge's own generated PM2/build assumptions.
4. **Server port:** `3005` — the other ports on this server are already taken
   by other apps.
5. Connect **GitHub** → `isAdamBailey/face-value`, branch `main`.
6. Enable **Push to deploy**.
7. **SSL** → obtain a Let's Encrypt certificate.

Forge clones the repo under `/home/forge/facevalue.example.com/`, with
`current/` symlinked to whichever release is active.

### Zero-downtime deployments (mandatory for Nuxt sites)

Forge **always** uses zero-downtime deployments for Nuxt and Next.js sites —
this isn't a toggle. Each deploy clones into a fresh `releases/<id>/`
directory; `current` only gets re-symlinked to it once every step succeeds.
Concretely this means:

- There's no `git pull` in our deploy script — Forge already did the clone
  before calling it.
- `$FORGE_SITE_PATH` points at `current` (the **previous**, still-live
  release) until activation — not the new code being built. Our script
  never references it; it just uses `$PWD`, since Forge already `cd`s into
  the new release directory first.
- `.env` is a Forge "shared path" and is symlinked into every new release
  automatically (this is Forge's default for zero-downtime sites — nothing
  to configure).
- Forge auto-generates the site's **Deploy Script** as a template with
  `$CREATE_RELEASE()` / `$ACTIVATE_RELEASE()` markers. Anything before
  `$ACTIVATE_RELEASE()` runs in the new (not-yet-live) release directory;
  anything after it runs once `current` points at the new release — that's
  where PM2 and the Go daemon restart belong, per
  [Forge's own docs](https://forge.laravel.com/docs/sites/deployments).

Replace the site's **Deploy Script** with the following — keep Forge's own
`$CREATE_RELEASE()` / `$ACTIVATE_RELEASE()` macros and its generated PM2
block (the `site-<id>.json` PM2 config Forge scaffolds for you), just add
the two lines noted below. Find the current template under the site's
**Apps** tab (or wherever your Forge version surfaces the deploy script) and
edit in place rather than replacing it wholesale, since Forge's own PM2
snippet already has your site's actual ID baked into the process name and
port — don't hand-copy those values from an example:

```bash
$CREATE_RELEASE()

cd $FORGE_RELEASE_DIRECTORY

bash scripts/forge-deploy.sh          # <-- add this line

$ACTIVATE_RELEASE()

# Ensure PM2 config exists...
if [ ! -f /home/forge/.pm2-conf/site-<id>.json ]; then
    mkdir -p /home/forge/.pm2-conf
    cat <<'EOF' > /home/forge/.pm2-conf/site-<id>.json
{
    name: "site-<id>",
    cwd: "/home/forge/facevalue.example.com/current/frontend",   # <-- add /frontend here
    script: "./.output/server/index.mjs",
    instances: "max",
    exec_mode: "cluster",
    port: "3005",
}
EOF
fi

# Start or reload the PM2 process...
pm2 start /home/forge/.pm2-conf/site-<id>.json || pm2 reload site-<id> --update-env
pm2 save

# Restart the Go API daemon now that `current` points at the new
# release — must come after $ACTIVATE_RELEASE(), never before.
sudo supervisorctl restart FORGE_API_DAEMON:*   # <-- add this line; :* restarts the whole process group, using the real daemon-XXXXXXX name from step 5
```

`<id>` above is Forge's numeric site ID — it's already correct in whatever
Forge generated for you; only the `cwd` line needs a manual edit. This is
also the fix if you hit `[PM2][ERROR] Error: Script not found: .../current/.output/server/index.mjs`
after a deploy that otherwise built successfully — the build produced
`frontend/.output/...`, but `cwd` was still pointing at the repo root.

Two things worth calling out:

- `bash scripts/forge-deploy.sh` (relative, not `$FORGE_SITE_PATH/scripts/forge-deploy.sh`)
  — a relative path is unambiguous since the preceding `cd $FORGE_RELEASE_DIRECTORY`
  already put you in the new release; `$FORGE_SITE_PATH` points at the *old*
  release and may not even exist yet on a brand-new site's first deploy.
- The Go daemon restart is **not** inside `scripts/forge-deploy.sh` — that
  script only runs during `$CREATE_RELEASE()`, before `current` is updated,
  so restarting the daemon there would restart it against the *old* code.
  It has to be a separate line in Forge's own script, after
  `$ACTIVATE_RELEASE()`.

---

## 3. S3 bucket

Private bucket, no public access ever — reads are presigned GETs generated by
the API, uploads go server-side through the API. No CORS config is needed.

```sh
aws s3api create-bucket \
  --bucket face-value-images \
  --region us-west-2 \
  --create-bucket-configuration LocationConstraint=us-west-2

aws s3api put-public-access-block \
  --bucket face-value-images \
  --public-access-block-configuration \
    BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true

aws s3api put-bucket-encryption \
  --bucket face-value-images \
  --server-side-encryption-configuration '{
    "Rules": [{"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}]
  }'

aws s3api put-bucket-lifecycle-configuration \
  --bucket face-value-images \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "images-to-ia",
      "Filter": {"Prefix": "images/"},
      "Status": "Enabled",
      "Transitions": [{"Days": 90, "StorageClass": "STANDARD_IA"}]
    }]
  }'
```

Dedicated IAM user, no console access, scoped to exactly the prefix the app
writes to:

```sh
aws iam create-user --user-name face-value-s3

cat > face-value-s3-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"],
    "Resource": "arn:aws:s3:::face-value-images/images/*"
  }]
}
EOF

aws iam put-user-policy \
  --user-name face-value-s3 \
  --policy-name face-value-s3-images \
  --policy-document file://face-value-s3-policy.json

aws iam create-access-key --user-name face-value-s3
```

Save the `AccessKeyId`/`SecretAccessKey` for step 4.

(Console equivalent: S3 → Create bucket, default "Block all public access" left
on, Default encryption → SSE-S3, Management → lifecycle rule; IAM → Users →
Create user with no console access → attach the inline policy above → Security
credentials → Create access key, use case "Application running outside AWS".)

---

## 4. Environment variables

Forge → site → **Environment**. Forge writes these to
`/home/forge/facevalue.example.com/.env` (site root, not inside each release).

Copy values from `.env.example` for local dev, then swap the following for
production:

| Variable | Local (`.env.example`) | Production (Forge) |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://…@postgres:5432/…` | `postgres://…@127.0.0.1:5432/facevalue?sslmode=disable` |
| `APP_BASE_URL` | `http://localhost:3000` | `https://facevalue.example.com` |
| `COOKIE_SIGNING_SECRET` | dev placeholder | `openssl rand -base64 32` |
| `COOKIE_SECURE` | `false` | `true` |
| `EMAIL_PROVIDER` | `smtp` | `ses` |
| `SMTP_HOST` | `mailpit` | omit (auto from `SES_REGION` when using `ses`) |
| `SMTP_PORT` | `1025` | omit (defaults to `587`) |
| `SMTP_USERNAME` | empty | your SMTP username |
| `SMTP_PASSWORD` | empty | your SMTP password |
| `SES_REGION` | empty | your SES region, e.g. `us-west-2` |
| `MAGIC_LINK_FROM_EMAIL` | `login@facevalue.local` | your verified sender address |
| `ALLOWED_EMAILS` | your email | same — must match what you type at login |
| `EBAY_API_BASE` | `https://api.sandbox.ebay.com` | `https://api.ebay.com` (production keyset — see [EBAY_SETUP.md](./EBAY_SETUP.md)) |
| `S3_BUCKET` / `S3_REGION` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | dev/MinIO values | real values from step 3 |
| `S3_ENDPOINT` | `http://minio:9000` | **omit** |
| `S3_PUBLIC_ENDPOINT` | `http://localhost:9000` | **omit** |
| `S3_FORCE_PATH_STYLE` | `true` | `false` |
| `NUXT_PUBLIC_API_BASE` | `http://localhost:8080` | omit (same-origin `/api` in production) |

`S3_ENDPOINT` and `S3_PUBLIC_ENDPOINT` exist only to point the S3 client at
MinIO for local dev. Leaving either set in production sends S3 calls to a host
that doesn't exist — see the troubleshooting table.

Production-only (not in `.env.example`):

| Variable | Value |
| --- | --- |
| `PORT` | `8080` (Go API — nginx proxies `/api` here) |
| `FORGE_API_DAEMON` | Supervisor name from Forge → Daemons, e.g. `daemon-1234567` — used in the Deploy Script's post-`$ACTIVATE_RELEASE()` restart line, not read by the app itself |

`NUXT_PORT` isn't something we set here — Forge's own generated PM2 config
(the `site-<id>.json` block in the Deploy Script) hardcodes the Nuxt port
directly via its `port` field, which PM2 turns into the process's `PORT`
env var.

Changing env vars only requires restarting the API daemon — no full redeploy:

```sh
sudo supervisorctl restart daemon-1234567:*
```

---

## 5. Go API daemon

1. **Server → Daemons → New Daemon**
2. **Command:** `/home/forge/facevalue.example.com/current/scripts/run-api.sh`
3. **Directory:** `/home/forge/facevalue.example.com/current`
4. **User:** `forge`

Copy the supervisor name from the daemon page (e.g. `daemon-1234567`, including
the `daemon-` prefix) into `FORGE_API_DAEMON`, then use that same name in the
`sudo supervisorctl restart FORGE_API_DAEMON` line you add to the site's
Deploy Script (step 2) — Forge doesn't restart daemons automatically on
deploy, and this app's daemon needs an explicit restart to pick up the
newly-built Go binary in the just-activated release.

---

## 6. Nginx — proxy `/api` to Go

Forge → site → **Nginx** — add **before** the existing `location /` block.
`client_max_body_size` and `proxy_read_timeout` are not needed by massa but are
required here — omitting them fails in ways that look like app bugs (413s on
upload, mystery 504s on slow appraisals):

```nginx
client_max_body_size 12M;

location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 120s;
}

location = /healthz {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Ensure `location /` proxies to the Nuxt port (`127.0.0.1:3005`). Save — Forge
reloads Nginx.

---

## 7. First deploy

1. **Deploy Now** in Forge (or push to `main` with push-to-deploy enabled).
2. Confirm the log shows: `go build`, the migrate step, `npm run build`, PM2
   start, and the daemon restart — see the troubleshooting table below for
   the (many) ways this can go sideways on a monorepo's first zero-downtime
   deploy; none of them are subtle once you know what to check.
3. Verify, from the server:

   ```sh
   curl http://127.0.0.1:8080/healthz    # Go daemon directly
   pm2 list                              # site-<id> should be "online"
   ```

   Then publicly:

   ```sh
   curl https://facevalue.example.com/healthz
   curl -I https://facevalue.example.com/
   ```

4. Request a magic link, complete login, and upload one photo end to end —
   confirm it reaches `status: "complete"` with a headline price. (If eBay
   credentials are still pending approval, expect it to reach
   `status: "failed"` at the pricing step instead — that's the app working
   correctly with an incomplete environment, not a bug.)

---

## eBay and Hugging Face credentials

See [EBAY_SETUP.md](./EBAY_SETUP.md) and [HUGGINGFACE_SETUP.md](./HUGGINGFACE_SETUP.md)
for obtaining production API credentials before switching `EBAY_API_BASE` off
sandbox.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| 502 on `/` | `pm2 list` — the site's `site-<id>` process must be online. Check the PM2 block in the Deploy Script ran (Forge deploy log). |
| 502 on `/api` | Check **Server → Daemons**; run `scripts/run-api.sh` manually for errors |
| `[PM2][ERROR] Process or Namespace site-<id> not found`, preceded by `Error: Script not found: .../current/.output/server/index.mjs` | The PM2 block's `cwd` points at the repo root, but this monorepo builds Nuxt to `frontend/.output/...`. Add `/frontend` to `cwd` in the PM2 config block — see §2's Deploy Script section. |
| Same errors, but `.output` genuinely doesn't exist anywhere | The build itself failed earlier in the log — check for a `go build`/`npm ci`/`npm run build` error above the PM2 section; Forge doesn't necessarily stop the deploy just because `scripts/forge-deploy.sh` exited non-zero. |
| PM2 step runs against the old code | The daemon/PM2 restart lines must come **after** `$ACTIVATE_RELEASE()` in the Deploy Script — before that marker, `current` still points at the previous release |
| 413 on upload | `client_max_body_size` missing from nginx |
| Upload 504s | `proxy_read_timeout` too low, or the pipeline is blocking the request (it shouldn't be — see `internal/appraisal`) |
| Thumbnails break after a while | Presigned URLs expired on a long-open tab — refetch, or raise `S3_PRESIGN_TTL` |
| All images 403 | IAM policy missing `s3:GetObject`, or the key prefix doesn't match `images/*` |
| Every upload fails, works locally | `S3_ENDPOINT` or `S3_PUBLIC_ENDPOINT` still set to a MinIO host in Forge env — omit both in production |
| Every search fails at identify | `HF_TOKEN` unset, or `HF_VISION_MODEL` no longer servable on the router |
| Every search returns 0 comps | Still pointing at `api.sandbox.ebay.com`, which has almost no inventory |
| Magic link 200, no email | Email not on `ALLOWED_EMAILS`, or SMTP misconfigured — check daemon logs |
| Login redirect broken | `APP_BASE_URL` must match your HTTPS domain |
| Deploy fails on `go build` | Install Go (see step 1) |
| Daemon restart fails in deploy | Use the full name **with the `:*` suffix**: `sudo supervisorctl restart daemon-1234567:*` — Supervisor registers Forge daemons as a process group, and `restart <name>` alone often doesn't match it |
| `daemon-<id>: ERROR (no such file)` on restart | Almost always a typo'd path in **Server → Daemons → this daemon**, not a missing file. Diff the `Command`/`Directory` fields against your site's *real* directory name character-for-character (`cat /etc/supervisor/conf.d/daemon-<id>.conf` shows exactly what Supervisor has) — easy to typo when copying from this doc's example domain instead of your actual one |
| Server binary runs by hand but the daemon won't start | `go run ./cmd/migrate up` / `config.Load()` fail fast on any missing required env var — run `scripts/run-api.sh` directly by hand (bypassing Supervisor) to see the *actual* error; Supervisor's own error messages are much vaguer |
| `bind: address already in use` on 8080, or the daemon respawns under new PIDs forever | **Stop** Supervisor first (`supervisorctl stop daemon-<id>:*`) before killing anything on the port — `autorestart=true` means killing the live process while Supervisor is still trying to manage it just respawns a new PID into the same race. Once stopped, `lsof -i :8080`, kill everything found, confirm the port is empty, *then* `supervisorctl start` |
| `.env` missing on the server despite Forge's "Linking environment file" step succeeding | If **Web Directory** is set to `frontend` (as this monorepo needs — see §2), Forge symlinks the shared `.env` relative to *that* directory, not the release/site root. Both `scripts/forge-deploy.sh` and `scripts/run-api.sh` already check `frontend/.env` first for this reason — if you see this on a fresh checkout, check the Web Directory setting matches |
| Two PM2 processes running for this site (e.g. `facevalue-web` *and* `site-<id>`) | `facevalue-web` (from `frontend/ecosystem.config.cjs`) is not used by Forge's zero-downtime deploys — only `site-<id>` (Forge's auto-generated config) actually serves traffic. `pm2 delete facevalue-web` if it's running; it's dead weight, not a second copy of the site |

**Diagnose magic links:**

```sh
sudo -u postgres psql -d facevalue -c \
  "SELECT user_email, created_at FROM magic_link_tokens ORDER BY created_at DESC LIMIT 5;"
sudo -u postgres psql -d facevalue -c "SELECT email FROM allowed_users;"
sudo supervisorctl tail -100 daemon-1234567
```

No token rows → address not allowed or rate-limited (5/hour). Token rows but no
mail → check SMTP credentials and sender address in Forge env.

**Diagnose a stuck appraisal:**

```sh
sudo -u postgres psql -d facevalue -c \
  "SELECT id, status, error_message, created_at FROM searches ORDER BY created_at DESC LIMIT 5;"
```

A row still `pending`/`identifying`/`pricing` more than 5 minutes after
`created_at` should have been marked `failed` on the next daemon boot
(`appraisal.Service.MarkStaleFailed`) — if it's still stuck, the daemon hasn't
restarted since it was orphaned.

---

## Cost

Typical monthly cost for personal use: VPS (~$6–12, often shared with other
apps on the same server), an existing Forge plan, and S3 storage/requests
(well under $1/month at this scale — the Standard-IA lifecycle rule is
future-proofing, not a real saving today).
