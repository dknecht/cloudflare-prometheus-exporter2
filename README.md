# Cloudflare Prometheus Exporter

A Cloudflare Prometheus Exporter built with [Effect TypeScript](https://effect.website) for deployment on Cloudflare Workers.

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/lablabs/cloudflare-exporter)

## Features

- **50+ Prometheus Metrics** - Comprehensive Cloudflare metrics including HTTP requests, bandwidth, threats, workers, load balancers, and more
- **Effect TypeScript** - Type-safe, composable error handling with the Effect library
- **Cloudflare Workers** - Serverless deployment with global edge distribution
- **Configurable** - Extensive configuration options via environment variables
- **Metrics Denylist** - Exclude specific metrics from collection
- **Zone Filtering** - Filter metrics by specific zones or exclude zones
- **Free Tier Support** - Option to limit metrics to free tier availability

## Quick Start

### One-Click Deploy

Click the deploy button above to deploy directly to Cloudflare Workers. You'll need to configure your Cloudflare API credentials as secrets.

### Manual Deployment

1. Clone the repository:
   ```bash
   git clone https://github.com/lablabs/cloudflare-exporter.git
   cd cloudflare-exporter
   ```

2. Install dependencies:
   ```bash
   pnpm install
   ```

3. Configure your API credentials:
   ```bash
   cp .dev.vars.example .dev.vars
   # Edit .dev.vars with your Cloudflare API token
   ```

4. Deploy:
   ```bash
   pnpm deploy
   ```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CF_API_TOKEN` | Cloudflare API Token (recommended) | - |
| `CF_API_KEY` | Cloudflare API Key (legacy) | - |
| `CF_API_EMAIL` | Cloudflare API Email (required with API Key) | - |
| `SCRAPE_DELAY` | Delay in seconds before fetching metrics | `300` |
| `TIME_WINDOW` | Time window in seconds for metrics queries | `60` |
| `CF_QUERY_LIMIT` | Maximum results per GraphQL query | `1000` |
| `CF_BATCH_SIZE` | Number of zones to process per batch | `10` |
| `FREE_TIER` | Only collect free tier metrics | `false` |
| `EXCLUDE_HOST` | Exclude host labels from metrics | `true` |
| `CF_HTTP_STATUS_GROUP` | Group HTTP status codes (2xx, 4xx, etc.) | `false` |
| `METRICS_DENYLIST` | Comma-separated list of metrics to exclude | - |
| `CF_ZONES` | Comma-separated list of zone IDs to include | - |
| `CF_EXCLUDE_ZONES` | Comma-separated list of zone IDs to exclude | - |
| `METRICS_PATH` | Custom path for metrics endpoint | `/metrics` |
| `SSL_CONCURRENCY` | Concurrent SSL certificate fetches | `5` |
| `RATE_LIMIT_RPS` | API rate limit (requests per second) | `4` |
| `DO_ALARM_INTERVAL` | Durable Object alarm interval in seconds | `60` |

### Setting Secrets

For deployment, set your API token as a secret:

```bash
wrangler secret put CF_API_TOKEN
```

### Creating an API Token

#### Quick Setup

Click the link below to create a token with the required permissions pre-filled:

**[Create Cloudflare Exporter Token](https://dash.cloudflare.com/profile/api-tokens?permissionGroupKeys=%5B%7B%22key%22%3A%22zone_analytics%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22account_analytics%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22workers_scripts%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22ssl_certificates%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22firewall_services%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22load_balancers%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22logpush%22%2C%22type%22%3A%22read%22%7D%5D&name=Cloudflare%20Prometheus%20Exporter)**

#### Manual Setup

1. Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Click "Create Token"
3. Select "Create Custom Token"
4. Add the following permissions:

**Required:**
| Permission | Access |
|------------|--------|
| Zone > Analytics | Read |
| Account > Account Analytics | Read |
| Account > Workers Scripts | Read |

**Optional (for additional metrics):**
| Permission | Access | Metrics |
|------------|--------|---------|
| Zone > SSL and Certificates | Read | Certificate expiry |
| Zone > Firewall Services | Read | Firewall rules labels |
| Zone > Load Balancers | Read | Load balancer health |
| Account > Magic Transit | Read | Magic Transit tunnels |
| Account > Logpush | Read | Logpush job status |

5. Set Zone/Account Resources:
   - **All zones** - or select specific zones
   - **All accounts** - or select specific accounts

6. Click "Continue to summary" → "Create Token"
7. Copy the token and store it securely

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/` | Landing page |
| `/metrics` | Prometheus metrics endpoint |
| `/health` | Health check endpoint |

## Available Metrics

### Zone Metrics
- `cloudflare_zone_requests_total` - Total requests per zone
- `cloudflare_zone_requests_cached` - Cached requests per zone
- `cloudflare_zone_requests_ssl_encrypted` - SSL encrypted requests
- `cloudflare_zone_requests_content_type` - Requests by content type
- `cloudflare_zone_requests_country` - Requests by country
- `cloudflare_zone_requests_status` - Requests by HTTP status
- `cloudflare_zone_requests_browser_map_page_views_count` - Page views by browser
- `cloudflare_zone_requests_origin_status_country_host` - Requests by origin status, country, host
- `cloudflare_zone_requests_status_country_host` - Requests by edge status, country, host
- `cloudflare_zone_request_method_count` - Requests by HTTP method
- `cloudflare_zone_bandwidth_total` - Total bandwidth in bytes
- `cloudflare_zone_bandwidth_cached` - Cached bandwidth
- `cloudflare_zone_bandwidth_ssl_encrypted` - SSL encrypted bandwidth
- `cloudflare_zone_bandwidth_content_type` - Bandwidth by content type
- `cloudflare_zone_bandwidth_country` - Bandwidth by country
- `cloudflare_zone_threats_total` - Total threats
- `cloudflare_zone_threats_country` - Threats by country
- `cloudflare_zone_threats_type` - Threats by type
- `cloudflare_zone_pageviews_total` - Total page views
- `cloudflare_zone_uniques_total` - Unique visitors
- `cloudflare_zone_cache_hit_ratio` - Cache hit ratio

### Colocation Metrics
- `cloudflare_zone_colocation_visits` - Visits per colocation
- `cloudflare_zone_colocation_edge_response_bytes` - Edge response bytes per colocation
- `cloudflare_zone_colocation_requests_total` - Requests per colocation
- `cloudflare_zone_colocation_visits_error` - Visits per colocation with error status codes
- `cloudflare_zone_colocation_edge_response_bytes_error` - Edge response bytes per colocation with errors
- `cloudflare_zone_colocation_requests_total_error` - Requests per colocation with errors

### Error Rate Metrics
- `cloudflare_zone_customer_error_4xx_rate` - 4xx error rate
- `cloudflare_zone_customer_error_5xx_rate` - 5xx error rate
- `cloudflare_zone_edge_error_rate` - Edge error rate
- `cloudflare_zone_origin_error_rate` - Origin error rate
- `cloudflare_zone_origin_response_duration_ms` - Origin response duration

### Worker Metrics
- `cloudflare_worker_requests_count` - Worker requests
- `cloudflare_worker_errors_count` - Worker errors
- `cloudflare_worker_cpu_time` - CPU time quantiles (P50, P75, P99, P999)
- `cloudflare_worker_duration` - Duration quantiles (P50, P75, P99, P999)

### Load Balancer Metrics
- `cloudflare_zone_pool_health_status` - Pool health status (1=healthy, 0=unhealthy)
- `cloudflare_zone_pool_requests_total` - Pool requests

### Health Check Metrics
- `cloudflare_zone_health_check_events_origin_count` - Health check events per origin
- `cloudflare_zone_health_check_events_avg` - Average health check events

### Firewall Metrics
- `cloudflare_zone_firewall_events_count` - Firewall events
- `cloudflare_zone_firewall_request_action` - Firewall actions
- `cloudflare_zone_firewall_bots_detected` - Bots detected
- `cloudflare_zone_bot_request_by_country` - Bot requests by country

### Logpush Metrics
- `cloudflare_logpush_failed_jobs_account_count` - Failed logpush jobs (account level)
- `cloudflare_logpush_failed_jobs_zone_count` - Failed logpush jobs (zone level)

### Magic Transit Metrics
- `cloudflare_magic_transit_active_tunnels` - Active tunnels
- `cloudflare_magic_transit_healthy_tunnels` - Healthy tunnels
- `cloudflare_magic_transit_tunnel_failures` - Tunnel failures
- `cloudflare_magic_transit_edge_colo_count` - Edge colocation sites

### SSL Certificate Metrics
- `cloudflare_zone_certificate_validation_status` - Certificate expiry timestamp

### Exporter Metrics
- `cloudflare_exporter_up` - Exporter health status
- `cloudflare_zones_total` - Total zones
- `cloudflare_zones_filtered` - Zones after filtering
- `cloudflare_zones_processed` - Zones processed

## Prometheus Configuration

Add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'cloudflare'
    scrape_interval: 5m
    scrape_timeout: 2m
    static_configs:
      - targets: ['your-worker.your-subdomain.workers.dev']
```

## Development

```bash
# Install dependencies
pnpm install

# Run locally
pnpm dev

# Type check
pnpm typecheck

# Deploy
pnpm deploy
```

## Architecture

This exporter is built using:

- **[Effect](https://effect.website)** - Type-safe functional programming library for TypeScript
- **Cloudflare Workers** - Serverless edge computing platform
- **Cloudflare Durable Objects** - Stateful storage for counter accumulation
- **Cloudflare GraphQL Analytics API** - For fetching metrics data
- **Cloudflare REST API** - For zones, accounts, and SSL certificates

### Durable Objects for Stateful Metrics

The exporter uses Cloudflare Durable Objects to maintain state for proper Prometheus counter semantics. Unlike gauges which represent point-in-time values, counters must monotonically increase and accumulate values over time.

**How it works:**
1. A single Durable Object instance runs per account
2. An alarm triggers every 60 seconds (configurable via `DO_ALARM_INTERVAL`) to fetch fresh data from Cloudflare APIs
3. Counter metrics track `{prev, accumulated}` values to compute deltas and accumulate them over time
4. Gauge metrics simply store the latest value
5. When Prometheus scrapes `/metrics`, the cached accumulated values are returned immediately

**Important tradeoffs:**
- **Staleness**: Metrics may be up to 60 seconds stale (or whatever `DO_ALARM_INTERVAL` is set to). The scrape response includes a staleness comment showing how old the data is.
- **Counter resets**: If a counter's raw value decreases (e.g., zone was removed and re-added), the exporter treats the new value as a delta addition to handle resets gracefully.
- **Fast scrapes**: Since data is pre-fetched, `/metrics` requests return immediately without waiting for API calls.

The Effect library provides:
- Type-safe error handling with tagged errors
- Dependency injection via services and layers
- Composable effects with automatic resource management
- Concurrent execution of API calls

## License

MIT
