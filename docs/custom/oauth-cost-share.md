# OAuth cost-share custom extension

This feature is a downstream customization for `Yachiyo-5i/xLyra`. Its
implementation is intentionally kept outside the upstream analytics, usage,
request-handler, and settings-handler packages.

## Owned custom code

- Backend: `server/internal/custom/oauthcostshare`
- Frontend: `web/src/features/oauth-cost-share`
- Localized strings: `web/src/locales/{zh,en,jp}/oauth-cost-share.json`
- Statistics display adapter: `web/src/features/oauth-cost-share/lib/statistics-display-utils.ts`

The backend exposes:

- `GET /api/v1/requests/oauth-cost-share`
- `GET /api/v1/settings/oauth-cost-share`
- `PUT /api/v1/settings/oauth-cost-share`

The configuration is stored at `global.oauth_cost_share` in the existing
configuration file, so no database migration is needed.

## Calculation contract

Only one selected site is accepted. The site must have an OAuth connection
with a supported plan: `plus`, `pro lite`, or `pro`.

The usage query always uses successful USD usage and ignores model and
downstream-key UUID filters. Statistics display names are normalized by
removing the text from the first half-width or full-width opening parenthesis
and the legacy `自用`/`员工用` suffix, then merged case-insensitively. In
particular, any key whose display name starts with `北海` is shown and
aggregated as `北海`.

```text
total_quota = single_quota * (1 + reset_count)
usage_share = grouped_usage_cost / total_quota
allocated_cost = usage_share * account_fee
```

## Upstream synchronization points

After updating from `Yachiyo-5i/xLyra/main`, check only these integration
points first:

1. `server/internal/app/router.go` still imports and mounts the custom handler
   inside the protected v1 router.
2. `web/src/routes/requests/charts.tsx` still renders
   `RequestsAnalyticsWorkspace` before `OAuthCostSharePanel`, keeping the
   shared date controls at the top.
3. `web/src/features/requests/components/requests-analytics-workspace.tsx`
   still applies the isolated statistics display adapter to bar and Sankey
   data without changing filter state or chart components.
4. `web/src/locales/i18n.ts` still lists and loads the `oauth-cost-share`
   namespace for all three locales.

If the upstream route layout changes, reapply these additive mount points. Do
not move the custom implementation into upstream `server/internal/analytics`
or the existing request analytics files unless the storage contract itself
requires an adapter update.

## Files intentionally left untouched

- `server/internal/analytics/**`
- `server/internal/usage/analytics.go`
- `server/internal/store/request_analytics_repository.go`
- `server/internal/admin/handler_requests.go`
- `web/src/features/requests/components/analytics-charts.tsx`
- `web/src/features/requests/lib/analytics-utils.ts`

The existing request analytics workspace is only touched at its presentation
boundary to apply the isolated name adapter; its filter and query behavior is
otherwise unchanged.
