# Pattern: static-site deploy on shared gateway

This is the canonical shape for deploying a static-content site (landing page, docs, off-chain metadata mount) onto the clusterkit shared `clusterkit-gateway`. Established by torale/webwhen, repeated by agentdance, repowire-relay, and clitcoin/freetheclit. Codified here so future sites copy from the template, not from memory.

## When this pattern fits

- Single nginx container serving prebuilt static files (HTML/CSS/SVG/JSON/images).
- One domain, optionally with a `www.` redirect and one or more subdomains (docs, metadata, etc).
- Cloudflare-fronted, Cloudflare Origin CA wildcard cert at the GKE Gateway, Full Strict SSL on the zone.
- Cost ceiling: one deployment per subdomain, spot pods, no LB per app.

## When this pattern does NOT fit

- Multi-image apps (frontend + backend). See `bananagraph/` for the multi-image shape — different chart structure, HPA, healthcheckpolicy, Cloud SQL bindings.
- Apps needing Cloud SQL, GCS buckets, or other Workload Identity bindings. See `terraform/projects/<app>/` for the project-resources pattern.
- Anything that needs Cloudflare Pages (we don't — gateway is consistent and free).

## Role split (READ THIS FIRST)

The single most common mistake on this pattern is two people doing the same deploy because the role boundary is unclear. The split is:

- **clusterkit repo** owns: terraform variables (`origin_ca_domains`, `cloudflare_domains`, `app_namespaces`, `github_deploy_repos`), the gateway, the Origin CA cert, the ReferenceGrant, the WIF service account. One PR per new project. Reviews welcome.
- **App repo** owns: Dockerfile, nginx config, helm chart, deploy script, deploy execution, post-deploy verification. The app repo's contributor runs the deploy and verifies. **Not clusterkit.**

### Canonical anti-pattern: post-merge double-deploy

On the freetheclit cutover, clusterkit and the clitcoin app peer both ran `scripts/deploy-web.sh` against the same merged SHA. Result: `helm list` showed `REVISION: 2` on both releases, identical end state, zero artifact-side harm. But it was two deploys against the same SHA, not one. The race itself is the data point.

Avoid it by: clusterkit acks the merge but does not deploy; the app repo's contributor runs the script and reports the verify output back. Clusterkit verifies post-hoc only if asked.

### Canonical anti-pattern: cross-worktree contamination

When the work spans repos, a peer rooted in repo A should not check out branches or commit in repo B's worktree. The clusterkit peer once committed a chart slice directly onto another peer's `feat/docs-site` branch in their worktree, intending to "just borrow the build context for a dry run." Result: the commit rode along on the other peer's next push and contaminated `origin/feat/docs-site`. Cleanup required a coordinated `git reset` + force-push with the affected peer.

Avoid it by: **spawn a dedicated peer per repo.** If the chart work belongs in the repowire repo and you're rooted in clusterkit, ask the orchestrator to spawn a `<project>-chart-claude-code` peer in a sibling worktree of that repo. Hand off via a `git format-patch` saved to a known path. Never reach into another peer's worktree.

Corollary: claims about your own past git/CLI actions must be verified against the remote BEFORE asserting them in a multi-agent thread, not after. Memory of "I didn't push that" is unreliable across context boundaries — `gh api repos/<org>/<repo>/commits/<sha>` is authoritative, your recall is not.

## Clusterkit-side: one terraform PR

Edit `terraform/variables.tf`:

```hcl
variable "app_namespaces" {
  default = [..., "<app>"]                       # adds ReferenceGrant
}

variable "github_deploy_repos" {
  default = {
    ...
    <app> = { repo = "<github-repo-name>" }      # creates WIF SA
  }
}

variable "origin_ca_domains" {
  default = [..., "<domain>.<tld>"]              # wildcard Origin CA cert
}

variable "cloudflare_domains" {
  default = [..., "<domain>.<tld>"]              # zone settings (Full Strict)
}
```

Then:

1. Confirm the Cloudflare zone for `<domain>.<tld>` is **active** in the baldmaninc account. If the domain was just bought, this requires a human (registrar nameservers → `*.ns.cloudflare.com`, then "Add Site" in the CF dashboard). Verify: `dig NS <domain> @1.1.1.1`.
2. `cd terraform && terraform plan -var="project_id=baldmaninc"`. Expect ~11 adds for a fresh domain + namespace.
3. **Create the app namespace first.** `kubectl create namespace <app>`. The ReferenceGrant in the plan depends on the namespace existing; terraform will fail otherwise. (Known wart; not worth folding into terraform yet — manual one-liner.)
4. `terraform apply`. Merge the PR. No further clusterkit action.

What this gives the app side:
- `<domain>-<tld>-origin-cert` SSL cert attached to `clusterkit-gateway`.
- ReferenceGrant `allow-clusterkit-to-<app>-services` in `<app>` namespace.
- WIF service account `gh-deploy-<app>@baldmaninc.iam.gserviceaccount.com` with `roles/artifactregistry.writer` + `roles/container.developer`. Idle by default; used if/when a GHA workflow is wired.
- Cloudflare zone settings: SSL `strict`, `always_use_https=on`, `min_tls_version=1.2`, `tls_1_3=on`.

## App-side: copy from the template

The skeleton lives at `templates/static-site/` in this repo. Copy the relevant files into the app repo and rename `SITE` → your site name (e.g. `clitcoin-web`, `repowire-docs`).

```sh
cp -r clusterkit/templates/static-site/web        myapp/web
cp -r clusterkit/templates/static-site/charts/SITE myapp/charts/<site-name>
cp clusterkit/templates/static-site/scripts/deploy-web.sh myapp/scripts/deploy-web.sh
```

What you adapt:
- `charts/<site-name>/values.yaml`: set `hostname`, `image.repository`, `wwwRedirect` (true for landings on apex, false for subdomains).
- `web/index.html` + `web/styles.css`: the actual content.
- `web/nginx.conf`: tweak `Cache-Control` per path if the defaults don't fit.
- `scripts/deploy-web.sh`: set `APP_NAME`, `NAMESPACE`, and the chart/Dockerfile paths.

The `web/security-headers.conf` and `web/_headers` files don't need editing for most landings. Read the security-headers gotcha section below before changing them.

### Helm chart shape (non-negotiable bits)

- `Deployment` + `Service` in the **app namespace** (`<app>`).
- `HTTPRoute` in the **clusterkit namespace** (not the app namespace). The HTTPRoute's `backendRefs[].namespace` is the app namespace (cross-namespace, valid via the ReferenceGrant).
- HTTPRoute annotation: `external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"`. **Always.** ExternalDNS uses this to create proxied A records.
- Spot pods: `nodeSelector: { cloud.google.com/gke-spot: "true" }` + toleration. 60-91% cheaper, fine for stateless static-serve.

### Optional pieces

- **`www` → apex 301 redirect**: rendered conditionally from the same chart when `wwwRedirect: true`. Uses a redirect-only HTTPRoute (no backendRefs needed; `RequestRedirect` filter is sufficient per Gateway API spec).
- **Metadata / docs subdomain**: ship as a **separate** helm chart (`charts/<app>-metadata` or `charts/<app>-docs`), with its own Deployment/Service/HTTPRoute. Independent redeploy story — important when content immutability matters (Token-2022 URIs, doc versioning). Add ACAO and tighter CSP if it's a machine-consumed origin.

### Security headers and nginx's `add_header` trap

nginx's `add_header` does NOT inherit from a parent block into a child `location` block that has its own `add_header` directives. **A location with even one `add_header` silently drops every parent header.** Verified live on this exact pattern.

The skeleton solves it with `security-headers.conf`, included from every `location` that sets its own `Cache-Control`:

```nginx
location = /styles.css {
  include /etc/nginx/conf.d/security-headers.conf;   # re-applies CSP + X-Frame + etc
  add_header Cache-Control "public, max-age=3600, must-revalidate" always;
}
```

If you change `nginx.conf`, smoke-test with `curl -I` against `/`, `/styles.css`, `/assets/whatever` and assert all the headers land.

## Deploy: manual is the default

Run from app repo root:

```sh
./scripts/deploy-web.sh
```

This is `docker buildx build --platform linux/amd64` (matters on ARM Macs — GKE is amd64), push to AR, `helm upgrade --install`, `kubectl rollout status`, curl smoke checks. Idempotent.

### Why manual, not GHA

Private repos pay for GHA minutes. v1 prototype landings don't need automation. If you flip the repo public later, or if the deploy needs to gate on tests run in CI, wire a workflow then. The WIF service account already exists; the `bananagraph/.github/workflows/production.yml` is the reference shape (multi-image, GKE deploy, Helmfile).

### What "post-deploy verification" looks like

Three checks before reporting "live":

```sh
# 1. apex HTTP 200 + headers present
curl -sSI https://<domain> | grep -iE "HTTP|content-security|cache-control|x-frame"

# 2. www → apex 301 (if wwwRedirect: true)
curl -sSI https://www.<domain> | grep -iE "HTTP|location"

# 3. Cloudflare-managed cert at edge (Cloudflare Universal SSL: Let's Encrypt or Google)
echo | openssl s_client -connect <domain>:443 -servername <domain> 2>/dev/null \
  | openssl x509 -noout -issuer -subject
```

The TLS issuer at the **edge** will be Let's Encrypt or Google Trust (Cloudflare's Universal SSL), not Cloudflare Origin CA. That's correct: Cloudflare terminates client SSL with their managed cert, then validates our Origin CA cert server-side under Full Strict. To verify the Origin CA cert directly you'd bypass Cloudflare (connect to the Gateway IP `34.149.49.202` with `--resolve`), but that's only worth doing if you suspect the Origin CA cert is broken.

## Gotchas (kept short — the long versions live in the noindex-staging-pattern doc)

- **ExternalDNS is `policy: upsert-only`.** Deleting an HTTPRoute does NOT delete its Cloudflare DNS record. If you rename or retire a hostname, clean it up via the Cloudflare API. There's a script for this somewhere; if not, the API call is `DELETE /zones/<zid>/dns_records/<rid>`.
- **`dig` is unreliable on proxied zones.** Cloudflare returns edge IPs (`188.114.96.0` / `188.114.97.0`) for any name in a proxied zone, regardless of whether a record exists. Verify record presence/absence via the Cloudflare API, not dig.
- **GCP load balancer reconcile is 30-60s.** A freshly applied HTTPRoute will return 404 from Cloudflare edge for ~minute even when `kubectl describe httproute` shows `Accepted: True`. Wait, then retry.
- **`--platform linux/amd64` on Apple Silicon.** `docker buildx` defaults to native arch; GKE nodes are amd64. Without the flag, the pod will `CrashLoopBackOff` with an exec format error. The skeleton script has the flag baked in.

## Reference deploys

Living examples, with notes on what each one does well or poorly:

- **clitcoin / freetheclit.com** (`prassanna-ravishankar/clitcoin`): the cleanest current reference. Two charts (web + metadata subdomain), manual deploy script, CSP via `security-headers.conf` include, www → apex redirect. Path to copy from for a new static site. PR #2 has the full diff.
- **torale / webwhen.ai** (`prassanna-ravishankar/torale`): the most mature; ships the noindex-soak + cutover-redirect pattern in the same chart via values flags. Read `helm/torale/templates/httproute.yaml` for the noindex + 301-redirect block — copy that if your site needs a soak window or a rebrand cutover. Multi-domain (legacy torale.ai + live webwhen.ai routes in one chart).
- **agentdance / agentdance.ai** (`prassanna-ravishankar/agentdance`): instructive *counter-example*. Has `landing/{Dockerfile,nginx.conf,index.html,_headers}` but **no helm chart in the repo** — the live HTTPRoute was applied out-of-band. Don't do this; the chart belongs in the repo. Also the `nginx.conf` sets server-scope `add_header` without a location override, which works by luck (the single `location /` doesn't override) — fragile.
- **bananagraph / bananagraph.com** (`prassanna-ravishankar/bananagraph`): NOT a static-site reference (multi-image, frontend + backend + Cloud SQL). Use as the **public-repo GHA reference**: `.github/workflows/production.yml` shows the WIF auth + GKE deploy shape if you ever wire one of these sites to GHA.
- **repowire-relay**: not static, but lives on the same gateway. Mentioned only because its HTTPRoute was historically squatting `repowire.io` apex — a useful debugging story if a new static-landing chart for a `*.repowire.io` host finds the apex unexpectedly busy.

## Quick checklist for a new static site

- [ ] Cloudflare zone for the domain is active in baldmaninc account
- [ ] clusterkit terraform PR opened: domain added to `origin_ca_domains` + `cloudflare_domains`, ns added to `app_namespaces`, repo added to `github_deploy_repos`
- [ ] `kubectl create namespace <app>` before terraform apply
- [ ] terraform apply, clusterkit PR merged
- [ ] App repo: copy from `clusterkit/templates/static-site/`, rename, adapt values + content
- [ ] `helm lint charts/<site>` passes locally
- [ ] `docker build` succeeds for both web and (if present) metadata images
- [ ] `curl` against running container shows expected security headers + per-path Cache-Control
- [ ] App-repo contributor runs `./scripts/deploy-web.sh`, posts the verify output
- [ ] DNS A record present (Cloudflare API), edge HTTP 200, www → apex 301, TLS edge cert healthy
