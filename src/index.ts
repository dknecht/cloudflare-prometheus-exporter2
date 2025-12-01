/**
 * Cloudflare Prometheus Exporter Worker
 * Main entry point for the Cloudflare Worker using Effect
 * Uses Durable Objects for stateful metric collection
 */
import { MetricsCollectorDO } from "./MetricsCollectorDO"

// Re-export the Durable Object class for Cloudflare Workers
export { MetricsCollectorDO }

// Environment bindings type
interface Env {
  readonly CF_API_TOKEN?: string
  readonly CF_API_KEY?: string
  readonly CF_API_EMAIL?: string
  readonly SCRAPE_DELAY?: string
  readonly TIME_WINDOW?: string
  readonly CF_QUERY_LIMIT?: string
  readonly CF_BATCH_SIZE?: string
  readonly FREE_TIER?: string
  readonly EXCLUDE_HOST?: string
  readonly CF_HTTP_STATUS_GROUP?: string
  readonly METRICS_DENYLIST?: string
  readonly CF_ZONES?: string
  readonly CF_EXCLUDE_ZONES?: string
  readonly METRICS_PATH?: string
  readonly SSL_CONCURRENCY?: string
  readonly RATE_LIMIT_RPS?: string
  readonly DO_ALARM_INTERVAL?: string
  readonly METRICS_COLLECTOR: DurableObjectNamespace<MetricsCollectorDO>
}

// Handle /metrics endpoint via Durable Object
const handleMetrics = async (env: Env): Promise<Response> => {
  // Use a fixed ID for single DO per account architecture
  const id = env.METRICS_COLLECTOR.idFromName("metrics-collector")
  const stub = env.METRICS_COLLECTOR.get(id)

  try {
    return await stub.fetch(new Request("http://do/metrics"))
  } catch (error) {
    console.error("Failed to fetch metrics from DO:", error)
    return new Response(`Error collecting metrics: ${String(error)}`, {
      status: 500,
      headers: { "Content-Type": "text/plain" },
    })
  }
}

// Handle /health endpoint
const handleHealth = (): Response =>
  new Response(JSON.stringify({ status: "healthy" }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  })

// Handle root endpoint
const handleRoot = (metricsPath: string): Response =>
  new Response(
    `<!DOCTYPE html>
<html>
<head><title>Cloudflare Prometheus Exporter</title></head>
<body>
  <h1>Cloudflare Prometheus Exporter</h1>
  <p>Built with Effect TypeScript for Cloudflare Workers</p>
  <ul>
    <li><a href="${metricsPath}">${metricsPath}</a> - Prometheus metrics endpoint</li>
    <li><a href="/health">/health</a> - Health check endpoint</li>
  </ul>
</body>
</html>`,
    {
      status: 200,
      headers: { "Content-Type": "text/html" },
    }
  )

// Handle 404
const handle404 = (): Response =>
  new Response("Not Found", { status: 404 })

// Main fetch handler
export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url)
    const metricsPath = env.METRICS_PATH ?? "/metrics"

    switch (url.pathname) {
      case "/":
        return handleRoot(metricsPath)
      case metricsPath:
        return handleMetrics(env)
      case "/health":
        return handleHealth()
      default:
        return handle404()
    }
  },
}
