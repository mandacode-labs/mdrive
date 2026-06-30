# mdrive Helm Chart

Deploys [mdrive](https://github.com/mandacode-labs/mdrive) on Kubernetes.

## Install

```bash
helm install mdrive ./charts/mdrive \
  --set config.database.host=postgres.example.com \
  --set secrets.database.existingSecret=mdrive-db-credentials \
  --set config.auth.issuer=sso.example.com \
  --set config.auth.clientID=client-id \
  --set secrets.auth.existingSecret=mdrive-auth-secrets
```

The chart will refuse to render if `config.auth.issuer` is set without an
encryption key source (see `values.schema.json` for the exact rules).

## OIDC login flow

`/auth/login`, `/auth/callback`, `/auth/logout` are handled by
[zitadel-go](https://github.com/zitadel/zitadel-go)'s `Authenticator`
mounted at the `/auth` path prefix by the chart's `AuthPassthrough`
middleware. The OpenAPI spec lists these endpoints (with the OIDC
browser-redirect contract) but ogen never actually serves them — the
middleware runs first.

The flow:

```
Browser                     mdrive (chart)                  IdP (Zitadel)
  │                              │                              │
  │ GET /auth/login              │                              │
  ├─────────────────────────────►│                              │
  │                              │ 302 → /authorize?...        │
  │                              ├─────────────────────────────►│
  │◄─────────────────────────────┤                              │
  │ 302 Location: IdP login      │                              │
  │                              │                              │
  │ (user authenticates at IdP)  │                              │
  │                              │                              │
  │ GET /auth/callback?code=..&state=..                          │
  ├─────────────────────────────►│                              │
  │                              │ POST /token (code exchange)  │
  │                              ├─────────────────────────────►│
  │                              │◄────── access/refresh ──────┤
  │                              │ GET /userinfo                │
  │                              ├─────────────────────────────►│
  │                              │◄────── user profile ─────────┤
  │                              │ (user upserted via          │
  │                              │  WithOnAuthenticated)        │
  │ 302 Set-Cookie: mdrive_session=...                            │
  │◄─────────────────────────────┤                              │
```

After login, subsequent requests carry the `mdrive_session` cookie.
The chart's bridge middleware decrypts it, looks up the user, and
synthesizes `Authorization: Bearer <userID>` for ogen's bearer auth.

## Required secrets

| Secret | Required when | How |
|---|---|---|
| `database.password` | always | K8s Secret + `existingSecret` |
| `auth.encryption_key` | `config.auth.issuer` is set | K8s Secret + `secrets.auth.existingSecret` |

`secrets.auth.encryption_key` MUST be at least 16 characters.
Rotate it together with the chart-managed Secret for zero-downtime
rollover (not yet automated — see ROADMAP.md).

## Architecture

See [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md) for the
application architecture. The chart's job is environment wiring —
configmaps, secrets, probes, resources — not business logic.
