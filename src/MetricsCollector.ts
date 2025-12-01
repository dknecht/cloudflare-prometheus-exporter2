/**
 * Metrics Collector Service
 * Orchestrates fetching metrics from Cloudflare APIs and populating the Prometheus registry
 */
import { Context, Effect, Layer } from "effect"
import { CloudflareClient, CloudflareApiError, GraphQLError } from "./CloudflareClient"
import { PrometheusRegistry, METRICS } from "./PrometheusRegistry"
import { ExporterConfig } from "./Config"
import type { Zone, Account, FirewallRulesResponse } from "./Types"

const FREE_TIER_PLAN_ID = "0feeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

// Type for firewall rules map
type FirewallRulesMap = Map<string, string>

// Service interface
export class MetricsCollector extends Context.Tag("MetricsCollector")<
  MetricsCollector,
  {
    readonly collect: () => Effect.Effect<string, CloudflareApiError | GraphQLError>
  }
>() {}

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

  // Add traditional firewall rules
  for (const rule of response.result) {
    if (rule.description) {
      map.set(rule.id, rule.description)
    }
  }

  // Add managed rulesets
  if (response.rulesets) {
    for (const ruleset of response.rulesets) {
      const desc = ruleset.description ?? ruleset.name
      map.set(ruleset.id, desc)
    }
  }

  return map
}

// Implementation
export const MetricsCollectorLive = Layer.effect(
  MetricsCollector,
  Effect.gen(function* () {
    const config = yield* ExporterConfig
    const client = yield* CloudflareClient
    const registry = yield* PrometheusRegistry

    // Filter zones based on config
    const filterZones = (zones: readonly Zone[]): readonly Zone[] => {
      let filtered = [...zones]

      // Filter by specific zone IDs if configured
      if (config.zones.length > 0) {
        filtered = filtered.filter((z) => config.zones.includes(z.id))
      }

      // Exclude specific zones if configured
      if (config.excludeZones.length > 0) {
        filtered = filtered.filter((z) => !config.excludeZones.includes(z.id))
      }

      return filtered
    }

    // Filter out free tier zones
    const filterNonFreeTierZones = (zones: readonly Zone[]): readonly Zone[] =>
      zones.filter((z) => z.plan.id !== FREE_TIER_PLAN_ID)

    // Get labels with optional host
    const getLabels = (
      base: Record<string, string>,
      host?: string
    ): Record<string, string> => {
      if (config.excludeHost || !host) return base
      return { ...base, host }
    }

    // Fetch firewall rules for zones and build a combined map
    const fetchFirewallRulesForZones = (zoneIDs: readonly string[]) =>
      Effect.gen(function* () {
        const allRules: FirewallRulesMap = new Map()

        // Fetch rules for each zone concurrently
        const results = yield* Effect.all(
          zoneIDs.map((zoneID) =>
            client.fetchFirewallRules(zoneID).pipe(
              Effect.map((response) => ({ zoneID, response })),
              Effect.catchAll(() => Effect.succeed({ zoneID, response: { result: [], rulesets: [] } }))
            )
          ),
          { concurrency: config.sslConcurrency }
        )

        // Combine all rules into a single map
        for (const { response } of results) {
          const zoneRules = buildFirewallRulesMap(response)
          for (const [id, desc] of zoneRules) {
            allRules.set(id, desc)
          }
        }

        return allRules
      })

    // Collect HTTP metrics for zones
    const collectHTTPMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[],
      firewallRules: FirewallRulesMap
    ) =>
      Effect.gen(function* () {
        const httpData = yield* client.fetchHTTPMetrics(zoneIDs)

        for (const zone of httpData.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
          if (!info) continue

          const groups = zone.httpRequests1mGroups
          if (groups.length === 0) continue

          const group = groups[0]!
          const baseLabels = { zone: info.name, account: info.account }

          // Request metrics
          yield* registry.counter(METRICS.ZONE_REQUESTS_TOTAL.name, METRICS.ZONE_REQUESTS_TOTAL.help, baseLabels, group.sum.requests)
          yield* registry.gauge(METRICS.ZONE_REQUESTS_CACHED.name, METRICS.ZONE_REQUESTS_CACHED.help, baseLabels, group.sum.cachedRequests)
          yield* registry.counter(METRICS.ZONE_REQUESTS_SSL_ENCRYPTED.name, METRICS.ZONE_REQUESTS_SSL_ENCRYPTED.help, baseLabels, group.sum.encryptedRequests)

          // Bandwidth metrics
          yield* registry.counter(METRICS.ZONE_BANDWIDTH_TOTAL.name, METRICS.ZONE_BANDWIDTH_TOTAL.help, baseLabels, group.sum.bytes)
          yield* registry.counter(METRICS.ZONE_BANDWIDTH_CACHED.name, METRICS.ZONE_BANDWIDTH_CACHED.help, baseLabels, group.sum.cachedBytes)
          yield* registry.counter(METRICS.ZONE_BANDWIDTH_SSL_ENCRYPTED.name, METRICS.ZONE_BANDWIDTH_SSL_ENCRYPTED.help, baseLabels, group.sum.encryptedBytes)

          // Threat metrics
          yield* registry.counter(METRICS.ZONE_THREATS_TOTAL.name, METRICS.ZONE_THREATS_TOTAL.help, baseLabels, group.sum.threats)

          // Page views and uniques
          yield* registry.counter(METRICS.ZONE_PAGEVIEWS_TOTAL.name, METRICS.ZONE_PAGEVIEWS_TOTAL.help, baseLabels, group.sum.pageViews)
          yield* registry.counter(METRICS.ZONE_UNIQUES_TOTAL.name, METRICS.ZONE_UNIQUES_TOTAL.help, baseLabels, group.uniq.uniques)

          // Cache hit ratio
          if (group.sum.requests > 0) {
            yield* registry.gauge(
              METRICS.ZONE_CACHE_HIT_RATIO.name,
              METRICS.ZONE_CACHE_HIT_RATIO.help,
              { ...baseLabels, requests: String(group.sum.requests), cachedRequests: String(group.sum.cachedRequests) },
              group.sum.cachedRequests / group.sum.requests
            )
          }

          // Content type breakdown
          for (const ct of group.sum.contentTypeMap) {
            const labels = { ...baseLabels, content_type: ct.edgeResponseContentTypeName }
            yield* registry.counter(METRICS.ZONE_REQUESTS_CONTENT_TYPE.name, METRICS.ZONE_REQUESTS_CONTENT_TYPE.help, labels, ct.requests)
            yield* registry.counter(METRICS.ZONE_BANDWIDTH_CONTENT_TYPE.name, METRICS.ZONE_BANDWIDTH_CONTENT_TYPE.help, labels, ct.bytes)
          }

          // Country breakdown
          for (const country of group.sum.countryMap) {
            const labels = { ...baseLabels, country: country.clientCountryName }
            yield* registry.counter(METRICS.ZONE_REQUESTS_COUNTRY.name, METRICS.ZONE_REQUESTS_COUNTRY.help, labels, country.requests)
            yield* registry.counter(METRICS.ZONE_BANDWIDTH_COUNTRY.name, METRICS.ZONE_BANDWIDTH_COUNTRY.help, labels, country.bytes)
            yield* registry.counter(METRICS.ZONE_THREATS_COUNTRY.name, METRICS.ZONE_THREATS_COUNTRY.help, labels, country.threats)
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
            yield* registry.counter(METRICS.ZONE_REQUESTS_STATUS.name, METRICS.ZONE_REQUESTS_STATUS.help, { ...baseLabels, status: "1xx" }, s1xx)
            yield* registry.counter(METRICS.ZONE_REQUESTS_STATUS.name, METRICS.ZONE_REQUESTS_STATUS.help, { ...baseLabels, status: "2xx" }, s2xx)
            yield* registry.counter(METRICS.ZONE_REQUESTS_STATUS.name, METRICS.ZONE_REQUESTS_STATUS.help, { ...baseLabels, status: "3xx" }, s3xx)
            yield* registry.counter(METRICS.ZONE_REQUESTS_STATUS.name, METRICS.ZONE_REQUESTS_STATUS.help, { ...baseLabels, status: "4xx" }, s4xx)
            yield* registry.counter(METRICS.ZONE_REQUESTS_STATUS.name, METRICS.ZONE_REQUESTS_STATUS.help, { ...baseLabels, status: "5xx" }, s5xx)
          } else {
            for (const status of group.sum.responseStatusMap) {
              yield* registry.counter(
                METRICS.ZONE_REQUESTS_STATUS.name,
                METRICS.ZONE_REQUESTS_STATUS.help,
                { ...baseLabels, status: String(status.edgeResponseStatus) },
                status.requests
              )
            }
          }

          // Browser breakdown
          for (const browser of group.sum.browserMap) {
            yield* registry.counter(
              METRICS.ZONE_REQUESTS_BROWSER.name,
              METRICS.ZONE_REQUESTS_BROWSER.help,
              { ...baseLabels, family: browser.uaBrowserFamily },
              browser.pageViews
            )
          }

          // Threat type breakdown
          for (const threat of group.sum.threatPathingMap) {
            yield* registry.counter(
              METRICS.ZONE_THREATS_TYPE.name,
              METRICS.ZONE_THREATS_TYPE.help,
              { ...baseLabels, type: threat.threatPathingName },
              threat.requests
            )
          }

          // Firewall events
          for (const fw of zone.firewallEventsAdaptiveGroups) {
            // Look up rule description from firewall rules map
            const ruleDesc = firewallRules.get(fw.dimensions.ruleId) ?? fw.dimensions.ruleId

            yield* registry.counter(METRICS.ZONE_FIREWALL_EVENTS_COUNT.name, METRICS.ZONE_FIREWALL_EVENTS_COUNT.help, baseLabels, fw.count)
            yield* registry.counter(
              METRICS.ZONE_FIREWALL_REQUEST_ACTION.name,
              METRICS.ZONE_FIREWALL_REQUEST_ACTION.help,
              { ...baseLabels, action: fw.dimensions.action, rule: ruleDesc },
              fw.count
            )
            yield* registry.counter(
              METRICS.ZONE_BOT_REQUEST_BY_COUNTRY.name,
              METRICS.ZONE_BOT_REQUEST_BY_COUNTRY.help,
              getLabels({ ...baseLabels, country: fw.dimensions.clientCountryName, action: fw.dimensions.action }, fw.dimensions.clientRequestHTTPHost),
              fw.count
            )
            // Bot detection metrics - track by source (bot_management, user_agent, etc.)
            yield* registry.counter(
              METRICS.ZONE_FIREWALL_BOTS_DETECTED.name,
              METRICS.ZONE_FIREWALL_BOTS_DETECTED.help,
              getLabels({ ...baseLabels, source: fw.dimensions.source, action: fw.dimensions.action }, fw.dimensions.clientRequestHTTPHost),
              fw.count
            )
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect health check metrics
    const collectHealthCheckMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchHealthCheckMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
          if (!info) continue

          let totalEvents = 0
          let totalCount = 0

          for (const group of zone.healthCheckEventsAdaptiveGroups) {
            totalEvents += group.count
            totalCount++

            yield* registry.counter(
              METRICS.ZONE_HEALTH_CHECK_EVENTS_ORIGIN_COUNT.name,
              METRICS.ZONE_HEALTH_CHECK_EVENTS_ORIGIN_COUNT.help,
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
            yield* registry.gauge(
              METRICS.ZONE_HEALTH_CHECK_EVENTS_AVG.name,
              METRICS.ZONE_HEALTH_CHECK_EVENTS_AVG.help,
              { zone: info.name, account: info.account },
              totalEvents / totalCount
            )
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect adaptive metrics (error rates, origin response times)
    const collectAdaptiveMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchAdaptiveMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
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

            yield* registry.counter(
              METRICS.ZONE_REQUESTS_ORIGIN_STATUS_COUNTRY_HOST.name,
              METRICS.ZONE_REQUESTS_ORIGIN_STATUS_COUNTRY_HOST.help,
              labels,
              group.count
            )

            yield* registry.gauge(
              METRICS.ZONE_ORIGIN_RESPONSE_DURATION_MS.name,
              METRICS.ZONE_ORIGIN_RESPONSE_DURATION_MS.help,
              labels,
              group.avg.originResponseDurationMs
            )

            if (status >= 400 && status < 500 && status !== 499) {
              yield* registry.counter(METRICS.ZONE_CUSTOMER_ERROR_4XX_RATE.name, METRICS.ZONE_CUSTOMER_ERROR_4XX_RATE.help, labels, group.count)
            } else if (status >= 500) {
              yield* registry.counter(METRICS.ZONE_CUSTOMER_ERROR_5XX_RATE.name, METRICS.ZONE_CUSTOMER_ERROR_5XX_RATE.help, labels, group.count)
            }
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect edge country metrics
    const collectEdgeCountryMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchEdgeCountryMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
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

            yield* registry.counter(
              METRICS.ZONE_REQUESTS_STATUS_COUNTRY_HOST.name,
              METRICS.ZONE_REQUESTS_STATUS_COUNTRY_HOST.help,
              labels,
              group.count
            )

            const status = group.dimensions.edgeResponseStatus
            if ((status >= 400 && status < 500) || (status >= 500 && status < 600)) {
              yield* registry.gauge(METRICS.ZONE_EDGE_ERROR_RATE.name, METRICS.ZONE_EDGE_ERROR_RATE.help, labels, 1)
            }
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect colocation metrics
    const collectColoMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchColoMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
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

            yield* registry.counter(METRICS.ZONE_COLOCATION_VISITS.name, METRICS.ZONE_COLOCATION_VISITS.help, labels, group.sum.visits)
            yield* registry.counter(METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES.name, METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES.help, labels, group.sum.edgeResponseBytes)
            yield* registry.counter(METRICS.ZONE_COLOCATION_REQUESTS_TOTAL.name, METRICS.ZONE_COLOCATION_REQUESTS_TOTAL.help, labels, group.count)
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect colocation error metrics (4xx/5xx)
    const collectColoErrorMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchColoErrorMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
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

            yield* registry.counter(METRICS.ZONE_COLOCATION_VISITS_ERROR.name, METRICS.ZONE_COLOCATION_VISITS_ERROR.help, labels, group.sum.visits)
            yield* registry.counter(METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES_ERROR.name, METRICS.ZONE_COLOCATION_EDGE_RESPONSE_BYTES_ERROR.help, labels, group.sum.edgeResponseBytes)
            yield* registry.counter(METRICS.ZONE_COLOCATION_REQUESTS_TOTAL_ERROR.name, METRICS.ZONE_COLOCATION_REQUESTS_TOTAL_ERROR.help, labels, group.count)
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect worker metrics
    const collectWorkerMetrics = (account: Account) =>
      Effect.gen(function* () {
        const data = yield* client.fetchWorkerTotals(account.id)
        const accountName = normalizeAccountName(account.name)

        for (const acc of data.viewer.accounts) {
          for (const worker of acc.workersInvocationsAdaptive) {
            const baseLabels = { script_name: worker.dimensions.scriptName, account: accountName }

            yield* registry.counter(METRICS.WORKER_REQUESTS_COUNT.name, METRICS.WORKER_REQUESTS_COUNT.help, baseLabels, worker.sum.requests)
            yield* registry.counter(METRICS.WORKER_ERRORS_COUNT.name, METRICS.WORKER_ERRORS_COUNT.help, baseLabels, worker.sum.errors)

            // CPU time quantiles
            yield* registry.gauge(METRICS.WORKER_CPU_TIME.name, METRICS.WORKER_CPU_TIME.help, { ...baseLabels, quantile: "P50" }, worker.quantiles.cpuTimeP50)
            yield* registry.gauge(METRICS.WORKER_CPU_TIME.name, METRICS.WORKER_CPU_TIME.help, { ...baseLabels, quantile: "P75" }, worker.quantiles.cpuTimeP75)
            yield* registry.gauge(METRICS.WORKER_CPU_TIME.name, METRICS.WORKER_CPU_TIME.help, { ...baseLabels, quantile: "P99" }, worker.quantiles.cpuTimeP99)
            yield* registry.gauge(METRICS.WORKER_CPU_TIME.name, METRICS.WORKER_CPU_TIME.help, { ...baseLabels, quantile: "P999" }, worker.quantiles.cpuTimeP999)

            // Duration quantiles
            yield* registry.gauge(METRICS.WORKER_DURATION.name, METRICS.WORKER_DURATION.help, { ...baseLabels, quantile: "P50" }, Math.round(worker.quantiles.durationP50 * 1000) / 1000)
            yield* registry.gauge(METRICS.WORKER_DURATION.name, METRICS.WORKER_DURATION.help, { ...baseLabels, quantile: "P75" }, Math.round(worker.quantiles.durationP75 * 1000) / 1000)
            yield* registry.gauge(METRICS.WORKER_DURATION.name, METRICS.WORKER_DURATION.help, { ...baseLabels, quantile: "P99" }, Math.round(worker.quantiles.durationP99 * 1000) / 1000)
            yield* registry.gauge(METRICS.WORKER_DURATION.name, METRICS.WORKER_DURATION.help, { ...baseLabels, quantile: "P999" }, Math.round(worker.quantiles.durationP999 * 1000) / 1000)
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect load balancer metrics
    const collectLoadBalancerMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchLoadBalancerMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
          if (!info) continue

          for (const group of zone.loadBalancingRequestsAdaptiveGroups) {
            yield* registry.counter(
              METRICS.POOL_REQUESTS_TOTAL.name,
              METRICS.POOL_REQUESTS_TOTAL.help,
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
              yield* registry.gauge(
                METRICS.POOL_HEALTH_STATUS.name,
                METRICS.POOL_HEALTH_STATUS.help,
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
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect logpush account metrics
    const collectLogpushAccountMetrics = (account: Account) =>
      Effect.gen(function* () {
        const data = yield* client.fetchLogpushAccountMetrics(account.id)

        for (const acc of data.viewer.accounts) {
          for (const group of acc.logpushHealthAdaptiveGroups) {
            yield* registry.counter(
              METRICS.LOGPUSH_FAILED_JOBS_ACCOUNT_COUNT.name,
              METRICS.LOGPUSH_FAILED_JOBS_ACCOUNT_COUNT.help,
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
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect logpush zone metrics
    const collectLogpushZoneMetrics = (zoneIDs: readonly string[]) =>
      Effect.gen(function* () {
        const data = yield* client.fetchLogpushZoneMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          for (const group of zone.logpushHealthAdaptiveGroups) {
            yield* registry.counter(
              METRICS.LOGPUSH_FAILED_JOBS_ZONE_COUNT.name,
              METRICS.LOGPUSH_FAILED_JOBS_ZONE_COUNT.help,
              {
                destination: group.dimensions.destinationType,
                job_id: String(group.dimensions.jobId),
                final: String(group.dimensions.final),
              },
              group.count
            )
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect Magic Transit metrics
    const collectMagicTransitMetrics = (account: Account) =>
      Effect.gen(function* () {
        const data = yield* client.fetchMagicTransitMetrics(account.id)
        const labels = { account: account.name, account_type: account.type }

        let activeTunnels = 0
        let healthyTunnels = 0
        let tunnelFailures = 0
        let edgeColoCount = 0

        for (const acc of data.viewer.accounts) {
          for (const group of acc.magicTransitTunnelHealthChecksAdaptiveGroups) {
            if (group.dimensions.active === 1) activeTunnels++
            if (group.dimensions.resultStatus === "healthy") healthyTunnels++
            else tunnelFailures++
            if (group.dimensions.edgePopName) edgeColoCount++
          }
        }

        yield* registry.gauge(METRICS.MAGIC_TRANSIT_ACTIVE_TUNNELS.name, METRICS.MAGIC_TRANSIT_ACTIVE_TUNNELS.help, labels, activeTunnels)
        yield* registry.gauge(METRICS.MAGIC_TRANSIT_HEALTHY_TUNNELS.name, METRICS.MAGIC_TRANSIT_HEALTHY_TUNNELS.help, labels, healthyTunnels)
        yield* registry.gauge(METRICS.MAGIC_TRANSIT_TUNNEL_FAILURES.name, METRICS.MAGIC_TRANSIT_TUNNEL_FAILURES.help, labels, tunnelFailures)
        yield* registry.gauge(METRICS.MAGIC_TRANSIT_EDGE_COLO_COUNT.name, METRICS.MAGIC_TRANSIT_EDGE_COLO_COUNT.help, labels, edgeColoCount)
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect SSL certificate metrics with concurrent fetching
    const collectSSLMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        // Fetch certificates concurrently with configurable concurrency
        const fetchCertsForZone = (zoneID: string) =>
          Effect.gen(function* () {
            const zone = zones.find((z) => z.id === zoneID)
            if (!zone) return

            const certs = yield* client.fetchSSLCertificates(zoneID).pipe(Effect.catchAll(() => Effect.succeed([])))

            for (const cert of certs) {
              const zoneName = cert.hosts[0]?.startsWith("*.")
                ? cert.hosts[1] ?? cert.hosts[0] ?? "unknown"
                : cert.hosts[0] ?? "unknown"

              const expiresOn = new Date(cert.expires_on).getTime() / 1000

              yield* registry.gauge(
                METRICS.ZONE_CERTIFICATE_VALIDATION_STATUS.name,
                METRICS.ZONE_CERTIFICATE_VALIDATION_STATUS.help,
                {
                  zone_id: zoneID,
                  zone_name: zoneName,
                  status: cert.status,
                  issuer: cert.issuer,
                },
                expiresOn
              )
            }
          })

        // Process all zones with concurrent limit
        yield* Effect.all(
          zoneIDs.map(fetchCertsForZone),
          { concurrency: config.sslConcurrency }
        )
      }).pipe(Effect.catchAll(() => Effect.void))

    // Collect request method metrics
    const collectRequestMethodMetrics = (
      zoneIDs: readonly string[],
      zones: readonly Zone[]
    ) =>
      Effect.gen(function* () {
        const data = yield* client.fetchRequestMethodMetrics(zoneIDs)

        for (const zone of data.viewer.zones) {
          const info = findZoneInfo(zones, zone.zoneTag)
          if (!info) continue

          for (const group of zone.httpRequestsAdaptiveGroups) {
            yield* registry.counter(
              METRICS.ZONE_REQUEST_METHOD_COUNT.name,
              METRICS.ZONE_REQUEST_METHOD_COUNT.help,
              {
                zone: info.name,
                account: info.account,
                method: group.dimensions.clientRequestHTTPMethodName,
              },
              group.count
            )
          }
        }
      }).pipe(Effect.catchAll(() => Effect.void))

    // Main collect function
    return {
      collect: () =>
        Effect.gen(function* () {
          yield* registry.clear()

          // Always add exporter up metric
          yield* registry.gauge(METRICS.EXPORTER_UP.name, METRICS.EXPORTER_UP.help, {}, 1)

          // Fetch zones and accounts
          const [zones, accounts] = yield* Effect.all([
            client.fetchZones(),
            client.fetchAccounts(),
          ])

          yield* registry.gauge(METRICS.ZONES_TOTAL.name, METRICS.ZONES_TOTAL.help, {}, zones.length)

          // Filter zones
          const filteredZones = filterZones(zones)
          yield* registry.gauge(METRICS.ZONES_FILTERED.name, METRICS.ZONES_FILTERED.help, {}, filteredZones.length)

          // Get non-free tier zones for premium metrics
          const nonFreeZones = config.freeTier ? filteredZones : filterNonFreeTierZones(filteredZones)
          const zoneIDs = nonFreeZones.map((z) => z.id)

          yield* registry.gauge(METRICS.ZONES_PROCESSED.name, METRICS.ZONES_PROCESSED.help, {}, zoneIDs.length)

          if (zoneIDs.length > 0) {
            // Fetch firewall rules for all zones (used for metric labels)
            const firewallRules = yield* fetchFirewallRulesForZones(zoneIDs)

            // Process zones in batches
            for (let i = 0; i < zoneIDs.length; i += config.batchSize) {
              const batchIDs = zoneIDs.slice(i, i + config.batchSize)

              yield* Effect.all([
                collectHTTPMetrics(batchIDs, nonFreeZones, firewallRules),
                collectHealthCheckMetrics(batchIDs, nonFreeZones),
                collectAdaptiveMetrics(batchIDs, nonFreeZones),
                collectEdgeCountryMetrics(batchIDs, nonFreeZones),
                collectRequestMethodMetrics(batchIDs, nonFreeZones),
              ], { concurrency: "unbounded" })

              if (!config.freeTier) {
                yield* Effect.all([
                  collectColoMetrics(batchIDs, nonFreeZones),
                  collectColoErrorMetrics(batchIDs, nonFreeZones),
                  collectLoadBalancerMetrics(batchIDs, nonFreeZones),
                  collectLogpushZoneMetrics(batchIDs),
                  collectSSLMetrics(batchIDs, nonFreeZones),
                ], { concurrency: "unbounded" })
              }
            }
          }

          // Collect account-level metrics
          for (const account of accounts) {
            yield* Effect.all([
              collectWorkerMetrics(account),
              collectLogpushAccountMetrics(account),
              collectMagicTransitMetrics(account),
            ], { concurrency: "unbounded" })
          }

          // Serialize and return metrics
          return yield* registry.serialize()
        }),
    }
  })
)
