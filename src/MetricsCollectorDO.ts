/**
 * Durable Object for Metrics Collection
 * Maintains state for proper Prometheus counter semantics across scrapes
 */
import { DurableObject } from "cloudflare:workers"
import { Effect, Layer, Exit } from "effect"
import { CloudflareClient, CloudflareClientLive } from "./CloudflareClient"
import { METRICS } from "./PrometheusRegistry"
import { ExporterConfig, makeConfigFromEnv } from "./Config"
import type { Zone, FirewallRulesResponse } from "./Types"

// Free tier plan ID
const FREE_TIER_PLAN_ID = "0feeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

// Counter state for accumulating deltas
interface CounterState {
  prev: number      // Previous raw value from API
  accumulated: number // Accumulated delta over time
}

// Stored metric state
interface MetricState {
  counters: Record<string, CounterState>  // metric_key -> { prev, accumulated }
  gauges: Record<string, number>          // metric_key -> latest_value
  lastFetch: number                       // Timestamp of last successful fetch
  lastError: string | undefined           // Last error message if any
}

// Environment bindings type (passed from worker)
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
}

// Type for firewall rules map
type FirewallRulesMap = Map<string, string>

// Helper to normalize account names
const normalizeAccountName = (name: string): string =>
  name.toLowerCase().replace(/ /g, "-")

// Helper to find zone info
const findZoneInfo = (
  zones: readonly Zone[],
  zoneTag: string
): { name: string; account: string } | undefined => {
  const zone = zones.find((z) => z.id === zoneTag)
  if (!zone) return undefined
  return {
    name: zone.name,
    account: normalizeAccountName(zone.account.name),
  }
}

// Helper to build firewall rules map from response
const buildFirewallRulesMap = (response: FirewallRulesResponse): FirewallRulesMap => {
  const map: FirewallRulesMap = new Map()
  for (const rule of response.result) {
    if (rule.description) {
      map.set(rule.id, rule.description)
    }
  }
  if (response.rulesets) {
    for (const ruleset of response.rulesets) {
      const desc = ruleset.description ?? ruleset.name
      map.set(ruleset.id, desc)
    }
  }
  return map
}

// Generate a unique key for a metric with labels
const metricKey = (name: string, labels: Record<string, string>): string => {
  const sortedLabels = Object.entries(labels)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join(",")
  return `${name}{${sortedLabels}}`
}

export class MetricsCollectorDO extends DurableObject<Env> {
  private state: MetricState = {
    counters: {},
    gauges: {},
    lastFetch: 0,
    lastError: undefined,
  }

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env)
    // Load state from storage on initialization
    this.ctx.blockConcurrencyWhile(async () => {
      const stored = await this.ctx.storage.get<MetricState>("metrics")
      if (stored) {
        this.state = stored
      }
    })
  }

  /**
   * Handle incoming fetch requests - returns cached metrics
   */
  override async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url)

    // Initialize alarm on first request if not set
    const currentAlarm = await this.ctx.storage.getAlarm()
    if (!currentAlarm) {
      const interval = parseInt(this.env.DO_ALARM_INTERVAL ?? "60", 10) * 1000
      await this.ctx.storage.setAlarm(Date.now() + interval)
    }

    if (url.pathname === "/metrics") {
      return this.serializeMetrics()
    }

    if (url.pathname === "/status") {
      return new Response(JSON.stringify({
        lastFetch: this.state.lastFetch,
        lastError: this.state.lastError,
        counterCount: Object.keys(this.state.counters).length,
        gaugeCount: Object.keys(this.state.gauges).length,
      }), {
        headers: { "Content-Type": "application/json" },
      })
    }

    return new Response("Not Found", { status: 404 })
  }

  /**
   * Alarm handler - fetches metrics from Cloudflare API on schedule
   */
  override async alarm(): Promise<void> {
    const interval = parseInt(this.env.DO_ALARM_INTERVAL ?? "60", 10) * 1000

    try {
      await this.collectMetrics()
      this.state.lastFetch = Date.now()
      this.state.lastError = undefined
    } catch (error) {
      console.error("Alarm: Failed to collect metrics:", error)
      this.state.lastError = error instanceof Error ? error.message : String(error)
    }

    // Save state and reschedule alarm
    await this.ctx.storage.put("metrics", this.state)
    await this.ctx.storage.setAlarm(Date.now() + interval)
  }

  /**
   * Collect metrics from Cloudflare API and update state
   */
  private async collectMetrics(): Promise<void> {
    const envRecord = this.env as Record<string, string | undefined>
    const configLayer = makeConfigFromEnv(envRecord)
    const clientLayer = CloudflareClientLive.pipe(Layer.provide(configLayer))
    const appLayer = Layer.merge(clientLayer, configLayer)

    // Temporary collectors for this fetch
    const rawCounters: Map<string, { name: string; labels: Record<string, string>; value: number }> = new Map()
    const rawGauges: Map<string, { name: string; labels: Record<string, string>; value: number }> = new Map()

    const collectProgram = Effect.gen(function* () {
      const config = yield* ExporterConfig
      const client = yield* CloudflareClient

      // Helper to add a raw counter value
      const addCounter = (name: string, labels: Record<string, string>, value: number) => {
        const key = metricKey(name, labels)
        rawCounters.set(key, { name, labels, value })
      }

      // Helper to add a raw gauge value
      const addGauge = (name: string, labels: Record<string, string>, value: number) => {
        const key = metricKey(name, labels)
        rawGauges.set(key, { name, labels, value })
      }

      // Helper to get labels with optional host
      const getLabels = (base: Record<string, string>, host?: string): Record<string, string> => {
        if (config.excludeHost || !host) return base
        return { ...base, host }
      }

      // Fetch zones and accounts
      const [zones, accounts] = yield* Effect.all([
        client.fetchZones(),
        client.fetchAccounts(),
      ])

      // Filter zones based on config
      let filteredZones = [...zones]
      if (config.zones.length > 0) {
        filteredZones = filteredZones.filter((z) => config.zones.includes(z.id))
      }
      if (config.excludeZones.length > 0) {
        filteredZones = filteredZones.filter((z) => !config.excludeZones.includes(z.id))
      }

      // Get non-free tier zones for premium metrics
      const nonFreeZones = config.freeTier
        ? filteredZones
        : filteredZones.filter((z) => z.plan.id !== FREE_TIER_PLAN_ID)
      const zoneIDs = nonFreeZones.map((z) => z.id)

      // Fetch firewall rules for all zones (used for metric labels)
      const firewallRules: FirewallRulesMap = new Map()
      if (zoneIDs.length > 0) {
        const results = yield* Effect.all(
          zoneIDs.map((zoneID) =>
            client.fetchFirewallRules(zoneID).pipe(
              Effect.map((response) => ({ zoneID, response })),
              Effect.catchAll(() => Effect.succeed({ zoneID, response: { result: [], rulesets: [] } as FirewallRulesResponse }))
            )
          ),
          { concurrency: config.sslConcurrency }
        )
        for (const { response } of results) {
          const zoneRules = buildFirewallRulesMap(response)
          for (const [id, desc] of zoneRules) {
            firewallRules.set(id, desc)
          }
        }
      }

      // Add exporter metrics
      addGauge(METRICS.EXPORTER_UP.name, {}, 1)
      addGauge(METRICS.ZONES_TOTAL.name, {}, zones.length)
      addGauge(METRICS.ZONES_FILTERED.name, {}, filteredZones.length)
      addGauge(METRICS.ZONES_PROCESSED.name, {}, zoneIDs.length)

      if (zoneIDs.length > 0) {
        // Process zones in batches
        for (let i = 0; i < zoneIDs.length; i += config.batchSize) {
          const batchIDs = zoneIDs.slice(i, i + config.batchSize)

          // Fetch HTTP metrics
          const httpData = yield* client.fetchHTTPMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

          for (const zone of httpData.viewer.zones) {
            const info = findZoneInfo(nonFreeZones, zone.zoneTag)
            if (!info) continue

            const groups = zone.httpRequests1mGroups
            if (groups.length === 0) continue

            const group = groups[0]!
            const baseLabels = { zone: info.name, account: info.account }

            // Request metrics
            addCounter(METRICS.ZONE_REQUESTS_TOTAL.name, baseLabels, group.sum.requests)
            addGauge(METRICS.ZONE_REQUESTS_CACHED.name, baseLabels, group.sum.cachedRequests)
            addCounter(METRICS.ZONE_REQUESTS_SSL_ENCRYPTED.name, baseLabels, group.sum.encryptedRequests)

            // Bandwidth metrics
            addCounter(METRICS.ZONE_BANDWIDTH_TOTAL.name, baseLabels, group.sum.bytes)
            addCounter(METRICS.ZONE_BANDWIDTH_CACHED.name, baseLabels, group.sum.cachedBytes)
            addCounter(METRICS.ZONE_BANDWIDTH_SSL_ENCRYPTED.name, baseLabels, group.sum.encryptedBytes)

            // Threat metrics
            addCounter(METRICS.ZONE_THREATS_TOTAL.name, baseLabels, group.sum.threats)

            // Page views and uniques
            addCounter(METRICS.ZONE_PAGEVIEWS_TOTAL.name, baseLabels, group.sum.pageViews)
            addCounter(METRICS.ZONE_UNIQUES_TOTAL.name, baseLabels, group.uniq.uniques)

            // Cache hit ratio
            if (group.sum.requests > 0) {
              addGauge(
                METRICS.ZONE_CACHE_HIT_RATIO.name,
                { ...baseLabels, requests: String(group.sum.requests), cachedRequests: String(group.sum.cachedRequests) },
                group.sum.cachedRequests / group.sum.requests
              )
            }

            // Content type breakdown
            for (const ct of group.sum.contentTypeMap) {
              const labels = { ...baseLabels, content_type: ct.edgeResponseContentTypeName }
              addCounter(METRICS.ZONE_REQUESTS_CONTENT_TYPE.name, labels, ct.requests)
              addCounter(METRICS.ZONE_BANDWIDTH_CONTENT_TYPE.name, labels, ct.bytes)
            }

            // Country breakdown
            for (const country of group.sum.countryMap) {
              const labels = { ...baseLabels, country: country.clientCountryName }
              addCounter(METRICS.ZONE_REQUESTS_COUNTRY.name, labels, country.requests)
              addCounter(METRICS.ZONE_BANDWIDTH_COUNTRY.name, labels, country.bytes)
              addCounter(METRICS.ZONE_THREATS_COUNTRY.name, labels, country.threats)
            }

            // HTTP status breakdown
            if (config.httpStatusGroup) {
              let s1xx = 0, s2xx = 0, s3xx = 0, s4xx = 0, s5xx = 0
              for (const status of group.sum.responseStatusMap) {
                const code = status.edgeResponseStatus
                if (code < 200) s1xx += status.requests
                else if (code < 300) s2xx += status.requests
                else if (code < 400) s3xx += status.requests
                else if (code < 500) s4xx += status.requests
                else s5xx += status.requests
              }
              addCounter(METRICS.ZONE_REQUESTS_STATUS.name, { ...baseLabels, status: "1xx" }, s1xx)
              addCounter(METRICS.ZONE_REQUESTS_STATUS.name, { ...baseLabels, status: "2xx" }, s2xx)
              addCounter(METRICS.ZONE_REQUESTS_STATUS.name, { ...baseLabels, status: "3xx" }, s3xx)
              addCounter(METRICS.ZONE_REQUESTS_STATUS.name, { ...baseLabels, status: "4xx" }, s4xx)
              addCounter(METRICS.ZONE_REQUESTS_STATUS.name, { ...baseLabels, status: "5xx" }, s5xx)
            } else {
              for (const status of group.sum.responseStatusMap) {
                addCounter(
                  METRICS.ZONE_REQUESTS_STATUS.name,
                  { ...baseLabels, status: String(status.edgeResponseStatus) },
                  status.requests
                )
              }
            }

            // Browser breakdown
            for (const browser of group.sum.browserMap) {
              addCounter(
                METRICS.ZONE_REQUESTS_BROWSER.name,
                { ...baseLabels, family: browser.uaBrowserFamily },
                browser.pageViews
              )
            }

            // Threat type breakdown
            for (const threat of group.sum.threatPathingMap) {
              addCounter(
                METRICS.ZONE_THREATS_TYPE.name,
                { ...baseLabels, type: threat.threatPathingName },
                threat.requests
              )
            }

            // Firewall events
            for (const fw of zone.firewallEventsAdaptiveGroups) {
              const ruleDesc = firewallRules.get(fw.dimensions.ruleId) ?? fw.dimensions.ruleId

              addCounter(METRICS.ZONE_FIREWALL_EVENTS_COUNT.name, baseLabels, fw.count)
              addCounter(
                METRICS.ZONE_FIREWALL_REQUEST_ACTION.name,
                { ...baseLabels, action: fw.dimensions.action, rule: ruleDesc },
                fw.count
              )
              addCounter(
                METRICS.ZONE_BOT_REQUEST_BY_COUNTRY.name,
                getLabels({ ...baseLabels, country: fw.dimensions.clientCountryName, action: fw.dimensions.action }, fw.dimensions.clientRequestHTTPHost),
                fw.count
              )
              addCounter(
                METRICS.ZONE_FIREWALL_BOTS_DETECTED.name,
                getLabels({ ...baseLabels, source: fw.dimensions.source, action: fw.dimensions.action }, fw.dimensions.clientRequestHTTPHost),
                fw.count
              )
            }
          }

          // Fetch health check metrics
          const healthData = yield* client.fetchHealthCheckMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

          for (const zone of healthData.viewer.zones) {
            const info = findZoneInfo(nonFreeZones, zone.zoneTag)
            if (!info) continue

            let totalEvents = 0
            let totalCount = 0

            for (const group of zone.healthCheckEventsAdaptiveGroups) {
              totalEvents += group.count
              totalCount++

              addCounter(
                METRICS.ZONE_HEALTH_CHECK_EVENTS_ORIGIN_COUNT.name,
                {
                  zone: info.name,
                  account: info.account,
                  health_status: group.dimensions.healthStatus,
                  origin_ip: group.dimensions.originIP,
                  fqdn: group.dimensions.fqdn,
                },
                group.count
              )
            }

            if (totalCount > 0) {
              addGauge(
                METRICS.ZONE_HEALTH_CHECK_EVENTS_AVG.name,
                { zone: info.name, account: info.account },
                totalEvents / totalCount
              )
            }
          }

          // Fetch adaptive metrics (error rates, origin response times)
          const adaptiveData = yield* client.fetchAdaptiveMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

          for (const zone of adaptiveData.viewer.zones) {
            const info = findZoneInfo(nonFreeZones, zone.zoneTag)
            if (!info) continue

            for (const group of zone.httpRequestsAdaptiveGroups) {
              const status = group.dimensions.originResponseStatus
              if (status === 0) continue

              const labels = getLabels(
                {
                  zone: info.name,
                  account: info.account,
                  status: String(status),
                  country: group.dimensions.clientCountryName,
                },
                group.dimensions.clientRequestHTTPHost
              )

              addCounter(METRICS.ZONE_REQUESTS_ORIGIN_STATUS_COUNTRY_HOST.name, labels, group.count)
              addGauge(METRICS.ZONE_ORIGIN_RESPONSE_DURATION_MS.name, labels, group.avg.originResponseDurationMs)

              if (status >= 400 && status < 500 && status !== 499) {
                addCounter(METRICS.ZONE_CUSTOMER_ERROR_4XX_RATE.name, labels, group.count)
              } else if (status >= 500) {
                addCounter(METRICS.ZONE_CUSTOMER_ERROR_5XX_RATE.name, labels, group.count)
              }
            }
          }

          // Fetch edge country metrics
          const edgeData = yield* client.fetchEdgeCountryMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

          for (const zone of edgeData.viewer.zones) {
            const info = findZoneInfo(nonFreeZones, zone.zoneTag)
            if (!info) continue

            for (const group of zone.httpRequestsEdgeCountryHost) {
              const labels = getLabels(
                {
                  zone: info.name,
                  account: info.account,
                  status: String(group.dimensions.edgeResponseStatus),
                  country: group.dimensions.clientCountryName,
                },
                group.dimensions.clientRequestHTTPHost
              )

              addCounter(METRICS.ZONE_REQUESTS_STATUS_COUNTRY_HOST.name, labels, group.count)

              const status = group.dimensions.edgeResponseStatus
              if ((status >= 400 && status < 500) || (status >= 500 && status < 600)) {
                addGauge(METRICS.ZONE_EDGE_ERROR_RATE.name, labels, 1)
              }
            }
          }

          // Fetch request method metrics
          const methodData = yield* client.fetchRequestMethodMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

          for (const zone of methodData.viewer.zones) {
            const info = findZoneInfo(nonFreeZones, zone.zoneTag)
            if (!info) continue

            for (const group of zone.httpRequestsAdaptiveGroups) {
              addCounter(
                METRICS.ZONE_REQUEST_METHOD_COUNT.name,
                {
                  zone: info.name,
                  account: info.account,
                  method: group.dimensions.clientRequestHTTPMethodName,
                },
                group.count
              )
            }
          }

          // Non-free tier metrics
          if (!config.freeTier) {
            // Fetch colo metrics
            const coloData = yield* client.fetchColoMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

            for (const zone of coloData.viewer.zones) {
              const info = findZoneInfo(nonFreeZones, zone.zoneTag)
              if (!info) continue

              for (const group of zone.httpRequestsAdaptiveGroups) {
                const labels = getLabels(
                  {
                    zone: info.name,
                    account: info.account,
                    colocation: group.dimensions.coloCode,
                  },
                  group.dimensions.clientRequestHTTPHost
                )

                addCounter(METRICS.ZONE_COLOCATION_VISITS.name, labels, group.sum.visits)
                addCounter(METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES.name, labels, group.sum.edgeResponseBytes)
                addCounter(METRICS.ZONE_COLOCATION_REQUESTS_TOTAL.name, labels, group.count)
              }
            }

            // Fetch colo error metrics
            const coloErrorData = yield* client.fetchColoErrorMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

            for (const zone of coloErrorData.viewer.zones) {
              const info = findZoneInfo(nonFreeZones, zone.zoneTag)
              if (!info) continue

              for (const group of zone.httpRequestsAdaptiveGroups) {
                const labels = getLabels(
                  {
                    zone: info.name,
                    account: info.account,
                    colocation: group.dimensions.coloCode,
                    status: String(group.dimensions.edgeResponseStatus),
                  },
                  group.dimensions.clientRequestHTTPHost
                )

                addCounter(METRICS.ZONE_COLOCATION_VISITS_ERROR.name, labels, group.sum.visits)
                addCounter(METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES_ERROR.name, labels, group.sum.edgeResponseBytes)
                addCounter(METRICS.ZONE_COLOCATION_REQUESTS_TOTAL_ERROR.name, labels, group.count)
              }
            }

            // Fetch load balancer metrics
            const lbData = yield* client.fetchLoadBalancerMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

            for (const zone of lbData.viewer.zones) {
              const info = findZoneInfo(nonFreeZones, zone.zoneTag)
              if (!info) continue

              for (const group of zone.loadBalancingRequestsAdaptiveGroups) {
                addCounter(
                  METRICS.POOL_REQUESTS_TOTAL.name,
                  {
                    zone: info.name,
                    account: info.account,
                    load_balancer_name: group.dimensions.lbName,
                    pool_name: group.dimensions.selectedPoolName,
                    origin_name: group.dimensions.selectedOriginName,
                  },
                  group.count
                )
              }

              for (const lb of zone.loadBalancingRequestsAdaptive) {
                for (const pool of lb.pools) {
                  addGauge(
                    METRICS.POOL_HEALTH_STATUS.name,
                    {
                      zone: info.name,
                      account: info.account,
                      load_balancer_name: lb.lbName,
                      pool_name: pool.poolName,
                    },
                    pool.healthy
                  )
                }
              }
            }

            // Fetch logpush zone metrics
            const logpushZoneData = yield* client.fetchLogpushZoneMetrics(batchIDs).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { zones: [] } })))

            for (const zone of logpushZoneData.viewer.zones) {
              for (const group of zone.logpushHealthAdaptiveGroups) {
                addCounter(
                  METRICS.LOGPUSH_FAILED_JOBS_ZONE_COUNT.name,
                  {
                    destination: group.dimensions.destinationType,
                    job_id: String(group.dimensions.jobId),
                    final: String(group.dimensions.final),
                  },
                  group.count
                )
              }
            }

            // Fetch SSL certificates
            for (const zoneID of batchIDs) {
              const certs = yield* client.fetchSSLCertificates(zoneID).pipe(Effect.catchAll(() => Effect.succeed([])))
              const zone = nonFreeZones.find((z) => z.id === zoneID)
              if (!zone) continue

              for (const cert of certs) {
                const zoneName = cert.hosts[0]?.startsWith("*.")
                  ? cert.hosts[1] ?? cert.hosts[0] ?? "unknown"
                  : cert.hosts[0] ?? "unknown"

                const expiresOn = new Date(cert.expires_on).getTime() / 1000

                addGauge(
                  METRICS.ZONE_CERTIFICATE_VALIDATION_STATUS.name,
                  {
                    zone_id: zoneID,
                    zone_name: zoneName,
                    status: cert.status,
                    issuer: cert.issuer,
                  },
                  expiresOn
                )
              }
            }
          }
        }
      }

      // Collect account-level metrics
      for (const account of accounts) {
        // Worker metrics
        const workerData = yield* client.fetchWorkerTotals(account.id).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { accounts: [] } })))
        const accountName = normalizeAccountName(account.name)

        for (const acc of workerData.viewer.accounts) {
          for (const worker of acc.workersInvocationsAdaptive) {
            const baseLabels = { script_name: worker.dimensions.scriptName, account: accountName }

            addCounter(METRICS.WORKER_REQUESTS_COUNT.name, baseLabels, worker.sum.requests)
            addCounter(METRICS.WORKER_ERRORS_COUNT.name, baseLabels, worker.sum.errors)

            // CPU time quantiles
            addGauge(METRICS.WORKER_CPU_TIME.name, { ...baseLabels, quantile: "P50" }, worker.quantiles.cpuTimeP50)
            addGauge(METRICS.WORKER_CPU_TIME.name, { ...baseLabels, quantile: "P75" }, worker.quantiles.cpuTimeP75)
            addGauge(METRICS.WORKER_CPU_TIME.name, { ...baseLabels, quantile: "P99" }, worker.quantiles.cpuTimeP99)
            addGauge(METRICS.WORKER_CPU_TIME.name, { ...baseLabels, quantile: "P999" }, worker.quantiles.cpuTimeP999)

            // Duration quantiles
            addGauge(METRICS.WORKER_DURATION.name, { ...baseLabels, quantile: "P50" }, Math.round(worker.quantiles.durationP50 * 1000) / 1000)
            addGauge(METRICS.WORKER_DURATION.name, { ...baseLabels, quantile: "P75" }, Math.round(worker.quantiles.durationP75 * 1000) / 1000)
            addGauge(METRICS.WORKER_DURATION.name, { ...baseLabels, quantile: "P99" }, Math.round(worker.quantiles.durationP99 * 1000) / 1000)
            addGauge(METRICS.WORKER_DURATION.name, { ...baseLabels, quantile: "P999" }, Math.round(worker.quantiles.durationP999 * 1000) / 1000)
          }
        }

        // Logpush account metrics
        const logpushData = yield* client.fetchLogpushAccountMetrics(account.id).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { accounts: [] } })))

        for (const acc of logpushData.viewer.accounts) {
          for (const group of acc.logpushHealthAdaptiveGroups) {
            addCounter(
              METRICS.LOGPUSH_FAILED_JOBS_ACCOUNT_COUNT.name,
              {
                account: account.name,
                account_type: account.type,
                destination: group.dimensions.destinationType,
                job_id: String(group.dimensions.jobId),
                final: String(group.dimensions.final),
              },
              group.count
            )
          }
        }

        // Magic Transit metrics
        const mtData = yield* client.fetchMagicTransitMetrics(account.id).pipe(Effect.catchAll(() => Effect.succeed({ viewer: { accounts: [] } })))
        const mtLabels = { account: account.name, account_type: account.type }

        let activeTunnels = 0
        let healthyTunnels = 0
        let tunnelFailures = 0
        let edgeColoCount = 0

        for (const acc of mtData.viewer.accounts) {
          for (const group of acc.magicTransitTunnelHealthChecksAdaptiveGroups) {
            if (group.dimensions.active === 1) activeTunnels++
            if (group.dimensions.resultStatus === "healthy") healthyTunnels++
            else tunnelFailures++
            if (group.dimensions.edgePopName) edgeColoCount++
          }
        }

        addGauge(METRICS.MAGIC_TRANSIT_ACTIVE_TUNNELS.name, mtLabels, activeTunnels)
        addGauge(METRICS.MAGIC_TRANSIT_HEALTHY_TUNNELS.name, mtLabels, healthyTunnels)
        addGauge(METRICS.MAGIC_TRANSIT_TUNNEL_FAILURES.name, mtLabels, tunnelFailures)
        addGauge(METRICS.MAGIC_TRANSIT_EDGE_COLO_COUNT.name, mtLabels, edgeColoCount)
      }

      return { counters: rawCounters, gauges: rawGauges }
    })

    const runnable = collectProgram.pipe(Effect.provide(appLayer))

    const exit = await Effect.runPromiseExit(runnable)

    if (Exit.isSuccess(exit)) {
      const { counters, gauges } = exit.value

      // Update counter state with deltas
      for (const [key, { value }] of counters) {
        const existing = this.state.counters[key]
        if (existing) {
          // Calculate delta (handle counter resets)
          const delta = value < existing.prev ? value : value - existing.prev
          this.state.counters[key] = {
            prev: value,
            accumulated: existing.accumulated + delta,
          }
        } else {
          // First time seeing this counter - start accumulating from current value
          this.state.counters[key] = {
            prev: value,
            accumulated: value,
          }
        }
      }

      // Update gauge state (just latest value)
      for (const [key, { value }] of gauges) {
        this.state.gauges[key] = value
      }
    } else {
      throw new Error(`Failed to collect metrics: ${String(exit.cause)}`)
    }
  }

  /**
   * Serialize collected metrics to Prometheus format
   */
  private serializeMetrics(): Response {
    const lines: string[] = []

    // Parse metric name and labels from key
    const parseKey = (key: string): { name: string; labels: string } => {
      const match = key.match(/^([^{]+)(\{.*\})?$/)
      if (!match) return { name: key, labels: "" }
      return { name: match[1]!, labels: match[2] ?? "" }
    }

    // Group by metric name for proper formatting
    const metricsByName: Map<string, Array<{ labels: string; value: number; type: "counter" | "gauge" }>> = new Map()

    // Collect counter metrics with accumulated values
    for (const [key, state] of Object.entries(this.state.counters)) {
      const { name, labels } = parseKey(key)
      if (!metricsByName.has(name)) {
        metricsByName.set(name, [])
      }
      metricsByName.get(name)!.push({ labels, value: state.accumulated, type: "counter" })
    }

    // Collect gauge metrics
    for (const [key, value] of Object.entries(this.state.gauges)) {
      const { name, labels } = parseKey(key)
      if (!metricsByName.has(name)) {
        metricsByName.set(name, [])
      }
      metricsByName.get(name)!.push({ labels, value, type: "gauge" })
    }

    // Look up help text from METRICS
    const getHelp = (name: string): string => {
      for (const metric of Object.values(METRICS)) {
        if (metric.name === name) return metric.help
      }
      return name
    }

    // Output in Prometheus format
    for (const [name, values] of metricsByName) {
      if (values.length === 0) continue

      const type = values[0]!.type
      const help = getHelp(name)

      lines.push(`# HELP ${name} ${help}`)
      lines.push(`# TYPE ${name} ${type}`)

      for (const { labels, value } of values) {
        lines.push(`${name}${labels} ${value}`)
      }
    }

    // Add staleness info as a comment
    const staleness = this.state.lastFetch > 0
      ? Math.round((Date.now() - this.state.lastFetch) / 1000)
      : -1
    lines.push(`# Metrics staleness: ${staleness}s`)

    if (this.state.lastError) {
      lines.push(`# Last error: ${this.state.lastError}`)
    }

    return new Response(lines.join("\n"), {
      status: 200,
      headers: {
        "Content-Type": "text/plain; version=0.0.4; charset=utf-8",
      },
    })
  }
}
