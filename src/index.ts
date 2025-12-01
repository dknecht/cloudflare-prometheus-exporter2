/**
 * Cloudflare Prometheus Exporter Worker
 * Main entry point for the Cloudflare Worker using Effect
 */
import { Effect, Layer, Exit } from "effect"
import { makeConfigFromEnv } from "./Config"
import { CloudflareClientLive } from "./CloudflareClient"
import { PrometheusRegistryLive } from "./PrometheusRegistry"
import { MetricsCollector, MetricsCollectorLive } from "./MetricsCollector"

// Environment bindings type
interface Env {
  readonly CF_API_TOKEN?: string
  readonly CF_API_KEY?: string
  readonly CF_API_EMAIL?: string
  readonly SCRAPE_DELAY?: string
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
}

// Create the full application layer
const makeAppLayer = (env: Env) => {
  const configLayer = makeConfigFromEnv(env as Record<string, string | undefined>)
  const clientLayer = CloudflareClientLive.pipe(Layer.provide(configLayer))
  const registryLayer = PrometheusRegistryLive.pipe(Layer.provide(configLayer))
  const collectorLayer = MetricsCollectorLive.pipe(
    Layer.provide(clientLayer),
    Layer.provide(registryLayer),
    Layer.provide(configLayer)
  )
  return collectorLayer
}

// Handle /metrics endpoint
const handleMetrics = (env: Env): Promise<Response> => {
  const program = Effect.gen(function* () {
    const collector = yield* MetricsCollector
    return yield* collector.collect()
  })

  const runnable = program.pipe(Effect.provide(makeAppLayer(env)))

  return Effect.runPromiseExit(runnable).then((exit) => {
    if (Exit.isSuccess(exit)) {
      return new Response(exit.value, {
        status: 200,
        headers: {
          "Content-Type": "text/plain; version=0.0.4; charset=utf-8",
        },
      })
    } else {
      const error = exit.cause
      console.error("Failed to collect metrics:", error)
      return new Response(`Error collecting metrics: ${String(error)}`, {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      })
    }
  })
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
