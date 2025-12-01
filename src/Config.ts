/**
 * Configuration service for the Cloudflare Prometheus Exporter
 * Uses Effect Config for type-safe environment variable access
 */
import { Config, Context, Effect, Layer } from "effect"

export class ExporterConfig extends Context.Tag("ExporterConfig")<
  ExporterConfig,
  {
    readonly apiToken: string | undefined
    readonly apiKey: string | undefined
    readonly apiEmail: string | undefined
    readonly scrapeDelay: number
    readonly timeWindow: number
    readonly queryLimit: number
    readonly batchSize: number
    readonly freeTier: boolean
    readonly excludeHost: boolean
    readonly httpStatusGroup: boolean
    readonly metricsDenylist: ReadonlySet<string>
    readonly zones: readonly string[]
    readonly excludeZones: readonly string[]
    readonly metricsPath: string
    readonly sslConcurrency: number
    readonly rateLimitRps: number
    readonly doAlarmInterval: number
  }
>() {}

const config = Config.all({
  apiToken: Config.string("CF_API_TOKEN").pipe(Config.option),
  apiKey: Config.string("CF_API_KEY").pipe(Config.option),
  apiEmail: Config.string("CF_API_EMAIL").pipe(Config.option),
  scrapeDelay: Config.integer("SCRAPE_DELAY").pipe(Config.withDefault(300)),
  timeWindow: Config.integer("TIME_WINDOW").pipe(Config.withDefault(60)),
  queryLimit: Config.integer("CF_QUERY_LIMIT").pipe(Config.withDefault(1000)),
  batchSize: Config.integer("CF_BATCH_SIZE").pipe(Config.withDefault(10)),
  freeTier: Config.boolean("FREE_TIER").pipe(Config.withDefault(false)),
  excludeHost: Config.boolean("EXCLUDE_HOST").pipe(Config.withDefault(true)),
  httpStatusGroup: Config.boolean("CF_HTTP_STATUS_GROUP").pipe(Config.withDefault(false)),
  metricsDenylist: Config.string("METRICS_DENYLIST").pipe(
    Config.withDefault(""),
    Config.map((s) => new Set(s.split(",").map((m) => m.trim()).filter((m) => m.length > 0)))
  ),
  zones: Config.string("CF_ZONES").pipe(
    Config.withDefault(""),
    Config.map((s) => s.split(",").map((z) => z.trim()).filter((z) => z.length > 0))
  ),
  excludeZones: Config.string("CF_EXCLUDE_ZONES").pipe(
    Config.withDefault(""),
    Config.map((s) => s.split(",").map((z) => z.trim()).filter((z) => z.length > 0))
  ),
  metricsPath: Config.string("METRICS_PATH").pipe(Config.withDefault("/metrics")),
  sslConcurrency: Config.integer("SSL_CONCURRENCY").pipe(Config.withDefault(5)),
  rateLimitRps: Config.integer("RATE_LIMIT_RPS").pipe(Config.withDefault(4)),
  doAlarmInterval: Config.integer("DO_ALARM_INTERVAL").pipe(Config.withDefault(60)),
})

export const ExporterConfigLive = Layer.effect(
  ExporterConfig,
  Effect.gen(function* () {
    const cfg = yield* config
    return {
      apiToken: cfg.apiToken._tag === "Some" ? cfg.apiToken.value : undefined,
      apiKey: cfg.apiKey._tag === "Some" ? cfg.apiKey.value : undefined,
      apiEmail: cfg.apiEmail._tag === "Some" ? cfg.apiEmail.value : undefined,
      scrapeDelay: cfg.scrapeDelay,
      timeWindow: cfg.timeWindow,
      queryLimit: cfg.queryLimit,
      batchSize: cfg.batchSize,
      freeTier: cfg.freeTier,
      excludeHost: cfg.excludeHost,
      httpStatusGroup: cfg.httpStatusGroup,
      metricsDenylist: cfg.metricsDenylist,
      zones: cfg.zones,
      excludeZones: cfg.excludeZones,
      metricsPath: cfg.metricsPath,
      sslConcurrency: cfg.sslConcurrency,
      rateLimitRps: cfg.rateLimitRps,
      doAlarmInterval: cfg.doAlarmInterval,
    }
  })
)

/**
 * Create a config layer from Cloudflare Worker environment bindings
 */
export const makeConfigFromEnv = (env: Record<string, string | undefined>) =>
  Layer.succeed(ExporterConfig, {
    apiToken: env["CF_API_TOKEN"],
    apiKey: env["CF_API_KEY"],
    apiEmail: env["CF_API_EMAIL"],
    scrapeDelay: parseInt(env["SCRAPE_DELAY"] ?? "300", 10),
    timeWindow: parseInt(env["TIME_WINDOW"] ?? "60", 10),
    queryLimit: parseInt(env["CF_QUERY_LIMIT"] ?? "1000", 10),
    batchSize: parseInt(env["CF_BATCH_SIZE"] ?? "10", 10),
    freeTier: env["FREE_TIER"] === "true",
    excludeHost: env["EXCLUDE_HOST"] !== "false",
    httpStatusGroup: env["CF_HTTP_STATUS_GROUP"] === "true",
    metricsDenylist: new Set(
      (env["METRICS_DENYLIST"] ?? "").split(",").map((m) => m.trim()).filter((m) => m.length > 0)
    ),
    zones: (env["CF_ZONES"] ?? "").split(",").map((z) => z.trim()).filter((z) => z.length > 0),
    excludeZones: (env["CF_EXCLUDE_ZONES"] ?? "").split(",").map((z) => z.trim()).filter((z) => z.length > 0),
    metricsPath: env["METRICS_PATH"] ?? "/metrics",
    sslConcurrency: parseInt(env["SSL_CONCURRENCY"] ?? "5", 10),
    rateLimitRps: parseInt(env["RATE_LIMIT_RPS"] ?? "4", 10),
    doAlarmInterval: parseInt(env["DO_ALARM_INTERVAL"] ?? "60", 10),
  })
