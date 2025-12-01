# cloudflare



## Authentication
Authentication towards the Cloudflare API can be done in two ways:

### API token
The preferred way of authenticating is with an API token, for which the scope can be configured at the Cloudflare
dashboard.

Required authentication scopes:
- `Analytics:Read` is required for zone-level metrics
- `Account.Account Analytics:Read` is required for Worker metrics
- `Account Settings:Read` is required for Worker metrics (for listing accessible accounts, scraping all available
  Workers included in authentication scope)
- `Firewall Services:Read` is required to fetch zone rule name for `cloudflare_zone_firewall_events_count` metric
- `Account. Account Rulesets:Read` is required to fetch account rule name for `cloudflare_zone_firewall_events_count` metric

To authenticate this way, only set `CF_API_TOKEN` (omit `CF_API_EMAIL` and `CF_API_KEY`)

### User email + API key
To authenticate with user email + API key, use the `Global API Key` from the Cloudflare dashboard.
Beware that this key authenticates with write access to every Cloudflare resource.

To authenticate this way, set both `CF_API_KEY` and `CF_API_EMAIL`.

## Configuration
The exporter can be configured using env variables or command flags.

| **KEY** | **description** |
|-|-|

| `EXCLUDE_HOST`
| `CF_API_EMAIL` |  user email (see https://support.cloudflare.com/hc/en-us/articles/200167836-Managing-API-Tokens-and-Keys) |
| `CF_API_KEY` |  API key associated with email (`CF_API_EMAIL` is required if this is set)|
| `CF_API_TOKEN` |  API authentication token (recommended before API key + email. Version 0.0.5+. see https://developers.cloudflare.com/analytics/graphql-api/getting-started/authentication/api-token-auth) |
| `CF_EXCLUDE_ZONES` |  (Optional) cloudflare zones to exclude, comma delimited list of zone ids. If not set, no zones from account are excluded |
| `FREE_TIER` | (Optional) scrape only metrics included in free plan. Accepts `true` or `false`, default `false`. |
| `LISTEN` |  listen on addr:port (default `:8080`), omit addr to listen on all interfaces |
| `METRICS_PATH` |  path for metrics, default `/metrics` |
| `SCRAPE_DELAY` | scrape delay in seconds, default `300` |
| `CF_BATCH_SIZE` | cloudflare request zones batch size (1 - 10), default `10` |
| `METRICS_DENYLIST` | (Optional) cloudflare-exporter metrics to not export, comma delimited list of cloudflare-exporter metrics. If not set, all metrics are exported |
| `ZONE_<NAME>` |  `DEPRECATED since 0.0.5` (optional) Zone ID. Add zones you want to scrape by adding env vars in this format. You can find the zone ids in Cloudflare dashboards. |


Corresponding flags:
```
  -cf_api_email="": cloudflare api email, works with api_key flag
  -cf_api_key="": cloudflare api key, works with api_email flag
  -cf_api_token="": cloudflare api token (version 0.0.5+, preferred)
  -cf_zones="": cloudflare zones to export, comma delimited list
  -cf_exclude_zones="": cloudflare zones to exclude, comma delimited list
  -free_tier=false: scrape only metrics included in free plan, default false
  -listen=":8080": listen on addr:port ( default :8080), omit addr to listen on all interfaces
  -metrics_path="/metrics": path for metrics, default /metrics
  -scrape_delay=300: scrape delay in seconds, defaults to 300
  -cf_batch_size=10: cloudflare zones batch size (1-10)
  -metrics_denylist="": cloudflare-exporter metrics to not export, comma delimited list
```

Note: `ZONE_<name>` configuration is not supported as flag.

## List of available metrics
```
"cloudflare_zone_requests_total"
"cloudflare_zone_requests_cached"
"cloudflare_zone_requests_ssl_encrypted"
"cloudflare_zone_requests_content_type"
"cloudflare_zone_requests_country"
"cloudflare_zone_requests_status"
"cloudflare_zone_requests_browser_map_page_views_count"
"cloudflare_zone_requests_origin_status_country_host"
"cloudflare_zone_requests_status_country_host"
"cloudflare_zone_bandwidth_total"
"cloudflare_zone_bandwidth_cached"
"cloudflare_zone_bandwidth_ssl_encrypted"
"cloudflare_zone_bandwidth_content_type"
"cloudflare_zone_bandwidth_country"
"cloudflare_zone_threats_total"
"cloudflare_zone_threats_country"
"cloudflare_zone_threats_type"
"cloudflare_zone_pageviews_total"
"cloudflare_zone_uniques_total"
"cloudflare_zone_colocation_visits"
"cloudflare_zone_colocation_edge_response_bytes"
"cloudflare_zone_colocation_requests_total"
"cloudflare_zone_firewall_events_count"
"cloudflare_zone_health_check_events_origin_count"
"cloudflare_worker_requests_count"
"cloudflare_worker_errors_count"
"cloudflare_worker_cpu_time"
"cloudflare_worker_duration"
"cloudflare_zone_pool_health_status"
"cloudflare_zone_pool_requests_total"
"cloudflare_logpush_failed_jobs_account_count"
"cloudflare_logpush_failed_jobs_zone_count"
Newly added_______________________________________________
"cloudflare_zone_customer_error_4xx_rate"
"cloudflare_zone_customer_error_5xx_rate"
"cloudflare_zone_edge_error_rate"
"cloudflare_zone_origin_error_rate"
"cloudflare_zone_bot_request_by_country"
"cloudflare_zone_cache_hit_ratio"
"cloudflare_zone_health_check_events_avg"
"cloudflare_zone_firewall_bots_detected"
"cloudflare_zone_firewall_request_action"
"cloudflare_zone_request_method_count"
"cloudflare_magic_transit_active_tunnels"
"cloudflare_magic_transit_healthy_tunnels"
"cloudflare_magic_transit_tunnel_failures"
"cloudflare_magic_transit_edge_colo_count"
"cloudflare_zone_certificate_validation_status"
"cloudflare_zone_origin_response_duration_ms"
"cloudflare_zone_colocation_visits_error"              
"cloudflare_zone_colocation_edge_response_bytes_error" 
"cloudflare_zone_colocation_requests_total_error"      

Docker file

download the file 
go to the file path where the cloudflare-exporter.tar

```
docker load -i cloudflare-exporter.tar
```

### Run

API token:
```
docker run -d -p 8080:8080 --name cloudflare-exporter -e CF_API_TOKEN=${CF_API_TOKEN} cloudflare-exporter
```


Authenticating with email + API key:
```
docker run --rm -p 8080:8080 --name cloudflare-exporter CF_API_KEY=${CF_API_KEY} -e CF_API_EMAIL=${CF_API_EMAIL} cloudflare-exporter
```


Configure zones and listening port:
```
docker run --rm -p 8080:8081 --name cloudflare-exporter CF_API_TOKEN=${CF_API_TOKEN} -e CF_ZONES=zoneid1,zoneid2,zoneid3 cloudflare-exporter
```

http status grouping - cloudflare_zone_requests_status - bool [false by default. true when groupig needed]
docker run --rm -p 8080:8080 --name cloudflare-exporter   -e CF_API_TOKEN=${CF_API_TOKEN} -e CF_HTTP_STATUS_GROUP=bool cloudflare-exporter


## Contributing and reporting issues
Feel free to create an issue in this repository if you have questions, suggestions or feature requests.




