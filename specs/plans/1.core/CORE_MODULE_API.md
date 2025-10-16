# Core Module API

## Monitoring

### POST `/api/monitoring/vitals`

- **Description:** Accepts batched Web Vitals metrics emitted by authenticated clients and pipes the values into the monitoring module for aggregation and Prometheus scraping.
- **Authentication:** Required (any logged-in user); no additional permission gate.
- **Request Body:**

```json
{
  "metrics": [
    {
      "metric": "LCP",
      "value": 1250.42,
      "rating": "good",
      "navigation_type": "navigate",
      "delta": 40.15
    }
  ]
}
```

- **Response:** `202 Accepted`

```json
{
  "success": true,
  "count": 1
}
```

- **Notes:**
  - Metrics are stored in seconds server-side for duration-based vital types (LCP, FID, INP, TTFB) and as-is for CLS-like unitless values.
  - Invalid or missing metric entries are ignored; the handler tolerates NaN/Inf payloads by dropping them.
  - A bundle analysis report is written during builds at `dist/bundle-report.json` to help track regressions relative to the 300 KB SSH workspace guardrail.
