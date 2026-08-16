# eBay Developer Setup

Face Value prices items by querying the eBay **Browse API** for active listings
matching the vision model's search query. This needs an eBay Developer account
and an application keyset — no user authorization flow, since Browse uses an
application (client-credentials) token, not a user token.

## 1. Create a developer account

1. Sign up at the [eBay Developers Program](https://developer.ebay.com/) with
   your eBay account (or create one).
2. Go to **My Account → Application Keys**.

## 2. Create a keyset

eBay issues two separate keysets — **Sandbox** and **Production** — each with
its own App ID (Client ID) and Cert ID (Client Secret). Sandbox has almost no
real inventory, so it's only useful to confirm the integration talks to the
API correctly, not to see realistic prices.

1. Under **Sandbox**, click **Create a keyset** if one doesn't already exist.
   Note the **App ID (Client ID)** and **Cert ID (Client Secret)**.
2. Under **Production**, request a keyset the same way. Production access for
   the Browse API is typically granted immediately for a new app; if it shows
   as pending, check the application's compliance requirements in the
   developer portal.

## 3. Confirm Browse API access

The Browse API needs no additional scope request beyond the default
`https://api.ebay.com/oauth/api_scope` — the app already requests exactly this
scope when fetching an application token (`internal/ebay/token.go`). No
extra enablement step should be needed, but if token requests come back
`invalid_scope`, check **Application Keys → your app → Browse API** is listed
under "APIs you can use."

## 4. Environment variables

| Variable | Sandbox (dev) | Production |
| --- | --- | --- |
| `EBAY_CLIENT_ID` | Sandbox App ID | Production App ID |
| `EBAY_CLIENT_SECRET` | Sandbox Cert ID | Production Cert ID |
| `EBAY_API_BASE` | `https://api.sandbox.ebay.com` | `https://api.ebay.com` |
| `EBAY_MARKETPLACE_ID` | `EBAY_US` | `EBAY_US` (or another marketplace — see note below) |
| `EBAY_COMP_LIMIT` | `50` | `50` |

## 5. Rate limits

Default production access is on the order of 5,000 Browse API calls/day,
tracked per application, not per user — since this is a single-user app, that
ceiling shouldn't matter in practice. `internal/ebay/client.go` logs the
`X-RateLimit-Remaining` response header when eBay sends one, so quota exhaustion
shows up in the daemon logs rather than as a silent failure.

## 6. Verifying it works

```sh
curl -s -X POST https://api.ebay.com/identity/v1/oauth2/token \
  -H "Authorization: Basic $(echo -n 'CLIENT_ID:CLIENT_SECRET' | base64)" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&scope=https%3A%2F%2Fapi.ebay.com%2Foauth%2Fapi_scope"
```

A successful response includes an `access_token`. Once `EBAY_CLIENT_ID` /
`EBAY_CLIENT_SECRET` are set in the app's environment, the easiest end-to-end
check is uploading a real photo and confirming the search reaches
`status: "complete"` with `comp_count > 0` — see [DEPLOY.md](./DEPLOY.md) step 7
for production, or run locally against sandbox first (expect 0 comps there;
sandbox has almost no inventory, which is expected, not a bug).

## Note on marketplace and currency

`internal/ebay/source.go` maps `EBAY_MARKETPLACE_ID` to an expected currency
(`EBAY_US` → `USD`, etc.) and silently skips any comp priced in a different
currency — there's no FX conversion in v1. If you switch marketplaces, confirm
the currency mapping in that file covers it.
