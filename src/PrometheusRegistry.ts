/**
 * Prometheus Registry Service
 * Handles metric collection and serialization in Prometheus exposition format
 */
import { Context, Effect, Layer, Ref } from "effect"
import { ExporterConfig } from "./Config"

// Metric types
type MetricType = "counter" | "gauge"

interface MetricValue {
  readonly labels: Record<string, string>
  readonly value: number
}

interface MetricDefinition {
  readonly name: string
  readonly help: string
  readonly type: MetricType
  readonly values: MetricValue[]
}

// All supported metric names
export const METRICS = {
  // Zone Request Metrics
  ZONE_REQUESTS_TOTAL: { name: "cloudflare_zone_requests_total", help: "Number of requests for zone" },
  ZONE_REQUESTS_CACHED: { name: "cloudflare_zone_requests_cached", help: "Number of cached requests for zone" },
  ZONE_REQUESTS_SSL_ENCRYPTED: { name: "cloudflare_zone_requests_ssl_encrypted", help: "Number of encrypted requests for zone" },
  ZONE_REQUESTS_CONTENT_TYPE: { name: "cloudflare_zone_requests_content_type", help: "Number of requests per content type" },
  ZONE_REQUESTS_COUNTRY: { name: "cloudflare_zone_requests_country", help: "Number of requests per country" },
  ZONE_REQUESTS_STATUS: { name: "cloudflare_zone_requests_status", help: "Number of requests per HTTP status" },
  ZONE_REQUESTS_BROWSER: { name: "cloudflare_zone_requests_browser_map_page_views_count", help: "Page views per browser" },
  ZONE_REQUESTS_ORIGIN_STATUS_COUNTRY_HOST: { name: "cloudflare_zone_requests_origin_status_country_host", help: "Requests per origin status, country, host" },
  ZONE_REQUESTS_STATUS_COUNTRY_HOST: { name: "cloudflare_zone_requests_status_country_host", help: "Requests per edge status, country, host" },
  ZONE_REQUEST_METHOD_COUNT: { name: "cloudflare_zone_request_method_count", help: "Number of requests per HTTP method" },

  // Zone Bandwidth Metrics
  ZONE_BANDWIDTH_TOTAL: { name: "cloudflare_zone_bandwidth_total", help: "Total bandwidth per zone in bytes" },
  ZONE_BANDWIDTH_CACHED: { name: "cloudflare_zone_bandwidth_cached", help: "Cached bandwidth per zone in bytes" },
  ZONE_BANDWIDTH_SSL_ENCRYPTED: { name: "cloudflare_zone_bandwidth_ssl_encrypted", help: "Encrypted bandwidth per zone in bytes" },
  ZONE_BANDWIDTH_CONTENT_TYPE: { name: "cloudflare_zone_bandwidth_content_type", help: "Bandwidth per content type" },
  ZONE_BANDWIDTH_COUNTRY: { name: "cloudflare_zone_bandwidth_country", help: "Bandwidth per country" },

  // Zone Threat Metrics
  ZONE_THREATS_TOTAL: { name: "cloudflare_zone_threats_total", help: "Threats per zone" },
  ZONE_THREATS_COUNTRY: { name: "cloudflare_zone_threats_country", help: "Threats per country" },
  ZONE_THREATS_TYPE: { name: "cloudflare_zone_threats_type", help: "Threats per type" },

  // Zone Page/Unique Metrics
  ZONE_PAGEVIEWS_TOTAL: { name: "cloudflare_zone_pageviews_total", help: "Page views per zone" },
  ZONE_UNIQUES_TOTAL: { name: "cloudflare_zone_uniques_total", help: "Unique visitors per zone" },

  // Colocation Metrics
  ZONE_COLOCATION_VISITS: { name: "cloudflare_zone_colocation_visits", help: "Visits per colocation" },
  ZONE_COLOCATION_EDGE_RESPONSE_BYTES: { name: "cloudflare_zone_colocation_edge_response_bytes", help: "Edge response bytes per colocation" },
  ZONE_COLOCATION_REQUESTS_TOTAL: { name: "cloudflare_zone_colocation_requests_total", help: "Requests per colocation" },
  ZONE_COLOCATION_VISITS_ERROR: { name: "cloudflare_zone_colocation_visits_error", help: "Visits per colocation with error status codes (4xx/5xx)" },
  ZONE_COLOCATION_EDGE_RESPONSE_BYTES_ERROR: { name: "cloudflare_zone_colocation_edge_response_bytes_error", help: "Edge response bytes per colocation with error codes" },
  ZONE_COLOCATION_REQUESTS_TOTAL_ERROR: { name: "cloudflare_zone_colocation_requests_total_error", help: "Requests per colocation with error codes" },

  // Firewall Metrics
  ZONE_FIREWALL_EVENTS_COUNT: { name: "cloudflare_zone_firewall_events_count", help: "Count of firewall events" },
  ZONE_FIREWALL_REQUEST_ACTION: { name: "cloudflare_zone_firewall_request_action", help: "Firewall events per action" },
  ZONE_FIREWALL_BOTS_DETECTED: { name: "cloudflare_zone_firewall_bots_detected", help: "Bot requests detected" },

  // Health Check Metrics
  ZONE_HEALTH_CHECK_EVENTS_ORIGIN_COUNT: { name: "cloudflare_zone_health_check_events_origin_count", help: "Health check events per origin" },
  ZONE_HEALTH_CHECK_EVENTS_AVG: { name: "cloudflare_zone_health_check_events_avg", help: "Average health check events" },

  // Worker Metrics
  WORKER_REQUESTS_COUNT: { name: "cloudflare_worker_requests_count", help: "Worker requests count" },
  WORKER_ERRORS_COUNT: { name: "cloudflare_worker_errors_count", help: "Worker errors count" },
  WORKER_CPU_TIME: { name: "cloudflare_worker_cpu_time", help: "Worker CPU time quantiles" },
  WORKER_DURATION: { name: "cloudflare_worker_duration", help: "Worker duration quantiles (GB*s)" },

  // Load Balancer Metrics
  POOL_HEALTH_STATUS: { name: "cloudflare_zone_pool_health_status", help: "Pool health status (1=healthy, 0=unhealthy)" },
  POOL_REQUESTS_TOTAL: { name: "cloudflare_zone_pool_requests_total", help: "Requests per pool" },

  // Logpush Metrics
  LOGPUSH_FAILED_JOBS_ACCOUNT_COUNT: { name: "cloudflare_logpush_failed_jobs_account_count", help: "Failed logpush jobs (account level)" },
  LOGPUSH_FAILED_JOBS_ZONE_COUNT: { name: "cloudflare_logpush_failed_jobs_zone_count", help: "Failed logpush jobs (zone level)" },

  // Error Rate Metrics
  ZONE_CUSTOMER_ERROR_4XX_RATE: { name: "cloudflare_zone_customer_error_4xx_rate", help: "4xx error rate" },
  ZONE_CUSTOMER_ERROR_5XX_RATE: { name: "cloudflare_zone_customer_error_5xx_rate", help: "5xx error rate" },
  ZONE_EDGE_ERROR_RATE: { name: "cloudflare_zone_edge_error_rate", help: "Edge error rate (4xx and 5xx)" },
  ZONE_ORIGIN_ERROR_RATE: { name: "cloudflare_zone_origin_error_rate", help: "Origin error rate" },
  ZONE_ORIGIN_RESPONSE_DURATION_MS: { name: "cloudflare_zone_origin_response_duration_ms", help: "Origin response duration in ms" },

  // Cache Metrics
  ZONE_CACHE_HIT_RATIO: { name: "cloudflare_zone_cache_hit_ratio", help: "Cache hit ratio" },

  // Bot Metrics
  ZONE_BOT_REQUEST_BY_COUNTRY: { name: "cloudflare_zone_bot_request_by_country", help: "Bot requests per country" },

  // Magic Transit Metrics
  MAGIC_TRANSIT_ACTIVE_TUNNELS: { name: "cloudflare_magic_transit_active_tunnels", help: "Number of active Magic Transit tunnels" },
  MAGIC_TRANSIT_HEALTHY_TUNNELS: { name: "cloudflare_magic_transit_healthy_tunnels", help: "Number of healthy Magic Transit tunnels" },
  MAGIC_TRANSIT_TUNNEL_FAILURES: { name: "cloudflare_magic_transit_tunnel_failures", help: "Number of failed Magic Transit tunnels" },
  MAGIC_TRANSIT_EDGE_COLO_COUNT: { name: "cloudflare_magic_transit_edge_colo_count", help: "Number of edge colocation sites" },

  // SSL Certificate Metrics
  ZONE_CERTIFICATE_VALIDATION_STATUS: { name: "cloudflare_zone_certificate_validation_status", help: "SSL certificate expiry timestamp" },

  // Exporter Info Metrics
  EXPORTER_UP: { name: "cloudflare_exporter_up", help: "Cloudflare exporter is up" },
  ZONES_TOTAL: { name: "cloudflare_zones_total", help: "Total number of zones" },
  ZONES_FILTERED: { name: "cloudflare_zones_filtered", help: "Zones after filtering" },
  ZONES_PROCESSED: { name: "cloudflare_zones_processed", help: "Zones actually processed" },
} as const

// Service interface
export class PrometheusRegistry extends Context.Tag("PrometheusRegistry")<
  PrometheusRegistry,
  {
    readonly counter: (
      name: string,
      help: string,
      labels: Record<string, string>,
      value: number
    ) => Effect.Effect<void>
    readonly gauge: (
      name: string,
      help: string,
      labels: Record<string, string>,
      value: number
    ) => Effect.Effect<void>
    readonly serialize: () => Effect.Effect<string>
    readonly clear: () => Effect.Effect<void>
  }
>() {}

// Implementation
export const PrometheusRegistryLive = Layer.effect(
  PrometheusRegistry,
  Effect.gen(function* () {
    const config = yield* ExporterConfig
    const metricsRef = yield* Ref.make<Map<string, MetricDefinition>>(new Map())

    const isAllowed = (name: string): boolean => !config.metricsDenylist.has(name)

    const getOrCreateMetric = (
      metrics: Map<string, MetricDefinition>,
      name: string,
      help: string,
      type: MetricType
    ): MetricDefinition => {
      const existing = metrics.get(name)
      if (existing) return existing
      const metric: MetricDefinition = { name, help, type, values: [] }
      metrics.set(name, metric)
      return metric
    }

    const formatLabels = (labels: Record<string, string>): string => {
      const entries = Object.entries(labels)
      if (entries.length === 0) return ""
      const formatted = entries
        .map(([k, v]) => `${k}="${v.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n")}"`)
        .join(",")
      return `{${formatted}}`
    }

    return {
      counter: (name, help, labels, value) =>
        Effect.gen(function* () {
          if (!isAllowed(name)) return
          yield* Ref.update(metricsRef, (metrics) => {
            const metric = getOrCreateMetric(metrics, name, help, "counter")
            metric.values.push({ labels, value })
            return metrics
          })
        }),

      gauge: (name, help, labels, value) =>
        Effect.gen(function* () {
          if (!isAllowed(name)) return
          yield* Ref.update(metricsRef, (metrics) => {
            const metric = getOrCreateMetric(metrics, name, help, "gauge")
            metric.values.push({ labels, value })
            return metrics
          })
        }),

      serialize: () =>
        Effect.gen(function* () {
          const metrics = yield* Ref.get(metricsRef)
          const lines: string[] = []

          for (const metric of metrics.values()) {
            lines.push(`# HELP ${metric.name} ${metric.help}`)
            lines.push(`# TYPE ${metric.name} ${metric.type}`)
            for (const { labels, value } of metric.values) {
              lines.push(`${metric.name}${formatLabels(labels)} ${value}`)
            }
          }

          return lines.join("\n")
        }),

      clear: () => Ref.set(metricsRef, new Map()),
    }
  })
)
