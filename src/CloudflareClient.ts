/**
 * Cloudflare API Client Service
 * Uses Effect for type-safe error handling and composition
 */
import { Context, Data, Effect, Layer } from "effect"
import type {
  AdaptiveGroupsResponse,
  ColoGroupsResponse,
  FirewallGroupsResponse,
  GraphQLResponse,
  HealthCheckGroupsResponse,
  HTTPEdgeCountryResponse,
  HTTPGroupsResponse,
  LoadBalancerResponse,
  LogpushAccountResponse,
  LogpushZoneResponse,
  MagicTransitResponse,
  RequestMethodResponse,
  SSLCertificate,
  SSLCertificateResponse,
  WorkerTotalsResponse,
  Zone,
  Account,
} from "./Types"
import { ExporterConfig } from "./Config"

const CF_GRAPHQL_ENDPOINT = "https://api.cloudflare.com/client/v4/graphql/"

// Error types
export class CloudflareApiError extends Data.TaggedError("CloudflareApiError")<{
  readonly message: string
  readonly statusCode?: number
}> {}

export class GraphQLError extends Data.TaggedError("GraphQLError")<{
  readonly messages: readonly string[]
}> {}

export class AuthenticationError extends Data.TaggedError("AuthenticationError")<{
  readonly message: string
}> {}

// Service interface
export class CloudflareClient extends Context.Tag("CloudflareClient")<
  CloudflareClient,
  {
    readonly fetchZones: () => Effect.Effect<readonly Zone[], CloudflareApiError>
    readonly fetchAccounts: () => Effect.Effect<readonly Account[], CloudflareApiError>
    readonly fetchHTTPMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<HTTPGroupsResponse, CloudflareApiError | GraphQLError>
    readonly fetchFirewallMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<FirewallGroupsResponse, CloudflareApiError | GraphQLError>
    readonly fetchHealthCheckMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<HealthCheckGroupsResponse, CloudflareApiError | GraphQLError>
    readonly fetchAdaptiveMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<AdaptiveGroupsResponse, CloudflareApiError | GraphQLError>
    readonly fetchEdgeCountryMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<HTTPEdgeCountryResponse, CloudflareApiError | GraphQLError>
    readonly fetchColoMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<ColoGroupsResponse, CloudflareApiError | GraphQLError>
    readonly fetchWorkerTotals: (
      accountID: string
    ) => Effect.Effect<WorkerTotalsResponse, CloudflareApiError | GraphQLError>
    readonly fetchLoadBalancerMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<LoadBalancerResponse, CloudflareApiError | GraphQLError>
    readonly fetchLogpushAccountMetrics: (
      accountID: string
    ) => Effect.Effect<LogpushAccountResponse, CloudflareApiError | GraphQLError>
    readonly fetchLogpushZoneMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<LogpushZoneResponse, CloudflareApiError | GraphQLError>
    readonly fetchMagicTransitMetrics: (
      accountID: string
    ) => Effect.Effect<MagicTransitResponse, CloudflareApiError | GraphQLError>
    readonly fetchSSLCertificates: (
      zoneID: string
    ) => Effect.Effect<readonly SSLCertificate[], CloudflareApiError>
    readonly fetchRequestMethodMetrics: (
      zoneIDs: readonly string[]
    ) => Effect.Effect<RequestMethodResponse, CloudflareApiError | GraphQLError>
  }
>() {}

// Implementation
export const CloudflareClientLive = Layer.effect(
  CloudflareClient,
  Effect.gen(function* () {
    const config = yield* ExporterConfig

    const getAuthHeaders = (): Record<string, string> => {
      if (config.apiToken) {
        return { Authorization: `Bearer ${config.apiToken}` }
      }
      if (config.apiKey && config.apiEmail) {
        return {
          "X-AUTH-EMAIL": config.apiEmail,
          "X-AUTH-KEY": config.apiKey,
        }
      }
      throw new AuthenticationError({ message: "No valid authentication provided" })
    }

    const getTimeRange = (): { mintime: string; maxtime: string } => {
      const now = new Date()
      now.setSeconds(0, 0)
      now.setTime(now.getTime() - config.scrapeDelay * 1000)
      const maxtime = now.toISOString()
      // 5 minute window for more data
      now.setTime(now.getTime() - 5 * 60 * 1000)
      const mintime = now.toISOString()
      return { mintime, maxtime }
    }

    const graphql = <T>(
      query: string,
      variables: Record<string, unknown>
    ): Effect.Effect<T, CloudflareApiError | GraphQLError> =>
      Effect.tryPromise({
        try: async () => {
          const response = await fetch(CF_GRAPHQL_ENDPOINT, {
            method: "POST",
            headers: {
              ...getAuthHeaders(),
              "Content-Type": "application/json",
            },
            body: JSON.stringify({ query, variables }),
          })

          if (!response.ok) {
            throw new CloudflareApiError({
              message: `GraphQL request failed: ${response.status} ${response.statusText}`,
              statusCode: response.status,
            })
          }

          const result = (await response.json()) as GraphQLResponse<T>
          if (result.errors && result.errors.length > 0) {
            throw new GraphQLError({
              messages: result.errors.map((e) => e.message),
            })
          }

          return result.data
        },
        catch: (error) => {
          if (error instanceof CloudflareApiError || error instanceof GraphQLError) {
            return error
          }
          return new CloudflareApiError({
            message: error instanceof Error ? error.message : String(error),
          })
        },
      })

    const restApi = <T>(path: string): Effect.Effect<T, CloudflareApiError> =>
      Effect.tryPromise({
        try: async () => {
          const response = await fetch(`https://api.cloudflare.com/client/v4${path}`, {
            headers: {
              ...getAuthHeaders(),
              "Content-Type": "application/json",
            },
          })

          if (!response.ok) {
            throw new CloudflareApiError({
              message: `REST API request failed: ${response.status} ${response.statusText}`,
              statusCode: response.status,
            })
          }

          return response.json() as Promise<T>
        },
        catch: (error) => {
          if (error instanceof CloudflareApiError) {
            return error
          }
          return new CloudflareApiError({
            message: error instanceof Error ? error.message : String(error),
          })
        },
      })

    return {
      fetchZones: () =>
        restApi<{ result: Zone[] }>("/zones?per_page=50").pipe(Effect.map((r) => r.result)),

      fetchAccounts: () =>
        restApi<{ result: Account[] }>("/accounts?per_page=100").pipe(Effect.map((r) => r.result)),

      fetchHTTPMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                httpRequests1mGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
                  uniq { uniques }
                  sum {
                    browserMap { pageViews uaBrowserFamily }
                    bytes cachedBytes cachedRequests
                    contentTypeMap { bytes requests edgeResponseContentTypeName }
                    countryMap { bytes clientCountryName requests threats }
                    encryptedBytes encryptedRequests pageViews requests
                    responseStatusMap { edgeResponseStatus requests }
                    threatPathingMap { requests threatPathingName }
                    threats
                    clientHTTPVersionMap { clientHTTPProtocol requests }
                    clientSSLMap { clientSSLProtocol requests }
                    ipClassMap { ipType requests }
                  }
                  dimensions { datetime }
                }
                firewallEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
                  count
                  dimensions { action source ruleId clientRequestHTTPHost clientCountryName }
                }
              }
            }
          }
        `
        return graphql<HTTPGroupsResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchFirewallMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                firewallEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
                  count
                  dimensions { action source ruleId clientRequestHTTPHost clientCountryName }
                }
              }
            }
          }
        `
        return graphql<FirewallGroupsResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchHealthCheckMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                healthCheckEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
                  count
                  dimensions { healthStatus originIP region fqdn }
                }
              }
            }
          }
        `
        return graphql<HealthCheckGroupsResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchAdaptiveMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                httpRequestsAdaptiveGroups(
                  limit: $limit,
                  filter: {
                    datetime_geq: $mintime,
                    datetime_lt: $maxtime,
                    cacheStatus_notin: ["hit"],
                    originResponseStatus_in: [400, 404, 500, 502, 503, 504, 522, 523, 524]
                  }
                ) {
                  count
                  dimensions { originResponseStatus clientCountryName clientRequestHTTPHost }
                  avg { originResponseDurationMs }
                }
              }
            }
          }
        `
        return graphql<AdaptiveGroupsResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchEdgeCountryMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                httpRequestsEdgeCountryHost: httpRequestsAdaptiveGroups(
                  limit: $limit,
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
                ) {
                  count
                  dimensions { edgeResponseStatus clientCountryName clientRequestHTTPHost }
                }
              }
            }
          }
        `
        return graphql<HTTPEdgeCountryResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchColoMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                httpRequestsAdaptiveGroups(
                  limit: $limit,
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
                ) {
                  count
                  dimensions { clientRequestHTTPHost coloCode datetime originResponseStatus }
                  sum { edgeResponseBytes visits }
                }
              }
            }
          }
        `
        return graphql<ColoGroupsResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchWorkerTotals: (accountID) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($accountID: String!, $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              accounts(filter: { accountTag: $accountID }) {
                workersInvocationsAdaptive(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
                  dimensions { scriptName status }
                  sum { requests errors duration }
                  quantiles {
                    cpuTimeP50 cpuTimeP75 cpuTimeP99 cpuTimeP999
                    durationP50 durationP75 durationP99 durationP999
                  }
                }
              }
            }
          }
        `
        return graphql<WorkerTotalsResponse>(query, {
          accountID,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchLoadBalancerMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                loadBalancingRequestsAdaptiveGroups(
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime },
                  limit: $limit
                ) {
                  count
                  dimensions { lbName selectedPoolName selectedOriginName }
                }
                loadBalancingRequestsAdaptive(
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime },
                  limit: $limit
                ) {
                  lbName
                  pools { poolName healthy }
                }
              }
            }
          }
        `
        return graphql<LoadBalancerResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchLogpushAccountMetrics: (accountID) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($accountID: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
            viewer {
              accounts(filter: { accountTag: $accountID }) {
                logpushHealthAdaptiveGroups(
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime, status_neq: 200 },
                  limit: $limit
                ) {
                  count
                  dimensions { jobId status destinationType datetime final }
                }
              }
            }
          }
        `
        return graphql<LogpushAccountResponse>(query, {
          accountID,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchLogpushZoneMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $limit: Int!, $mintime: Time!, $maxtime: Time!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                logpushHealthAdaptiveGroups(
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime, status_neq: 200 },
                  limit: $limit
                ) {
                  count
                  dimensions { jobId status destinationType datetime final }
                }
              }
            }
          }
        `
        return graphql<LogpushZoneResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchMagicTransitMetrics: (accountID) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($accountID: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
            viewer {
              accounts(filter: { accountTag: $accountID }) {
                magicTransitTunnelHealthChecksAdaptiveGroups(
                  limit: $limit,
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
                ) {
                  count
                  dimensions {
                    active datetime edgeColoCity edgeColoCountry edgePopName
                    remoteTunnelIPv4 resultStatus siteName tunnelName
                  }
                }
              }
            }
          }
        `
        return graphql<MagicTransitResponse>(query, {
          accountID,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },

      fetchSSLCertificates: (zoneID) =>
        restApi<SSLCertificateResponse>(`/zones/${zoneID}/ssl/certificate_packs`).pipe(
          Effect.map((r) => r.result)
        ),

      fetchRequestMethodMetrics: (zoneIDs) => {
        const { mintime, maxtime } = getTimeRange()
        const query = `
          query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
            viewer {
              zones(filter: { zoneTag_in: $zoneIDs }) {
                zoneTag
                httpRequestsAdaptiveGroups(
                  limit: $limit,
                  filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
                ) {
                  count
                  dimensions { clientRequestHTTPMethodName }
                }
              }
            }
          }
        `
        return graphql<RequestMethodResponse>(query, {
          zoneIDs,
          mintime,
          maxtime,
          limit: config.queryLimit,
        })
      },
    }
  })
)
