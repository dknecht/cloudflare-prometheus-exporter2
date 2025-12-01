/**
 * Type definitions for Cloudflare API responses
 * These match the GraphQL and REST API response structures
 */

// Zone types from REST API
export interface Zone {
  readonly id: string
  readonly name: string
  readonly status: string
  readonly plan: {
    readonly id: string
    readonly name: string
    readonly is_subscribed: boolean
  }
  readonly account: {
    readonly id: string
    readonly name: string
  }
}

export interface Account {
  readonly id: string
  readonly name: string
  readonly type: string
}

// GraphQL response wrapper
export interface GraphQLResponse<T> {
  readonly data: T
  readonly errors?: readonly { readonly message: string }[]
}

// HTTP Requests 1m Groups
export interface HTTP1mGroup {
  readonly dimensions: { readonly datetime: string }
  readonly uniq: { readonly uniques: number }
  readonly sum: {
    readonly bytes: number
    readonly cachedBytes: number
    readonly cachedRequests: number
    readonly requests: number
    readonly encryptedBytes: number
    readonly encryptedRequests: number
    readonly pageViews: number
    readonly threats: number
    readonly browserMap: readonly { readonly pageViews: number; readonly uaBrowserFamily: string }[]
    readonly contentTypeMap: readonly {
      readonly bytes: number
      readonly requests: number
      readonly edgeResponseContentTypeName: string
    }[]
    readonly countryMap: readonly {
      readonly bytes: number
      readonly clientCountryName: string
      readonly requests: number
      readonly threats: number
    }[]
    readonly responseStatusMap: readonly { readonly edgeResponseStatus: number; readonly requests: number }[]
    readonly threatPathingMap: readonly { readonly threatPathingName: string; readonly requests: number }[]
    readonly clientHTTPVersionMap: readonly { readonly clientHTTPProtocol: string; readonly requests: number }[]
    readonly clientSSLMap: readonly { readonly clientSSLProtocol: string; readonly requests: number }[]
    readonly ipClassMap: readonly { readonly ipType: string; readonly requests: number }[]
  }
}

// Firewall Events
export interface FirewallEventGroup {
  readonly count: number
  readonly dimensions: {
    readonly action: string
    readonly source: string
    readonly ruleId: string
    readonly clientRequestHTTPHost: string
    readonly clientCountryName: string
  }
}

// Health Check Events
export interface HealthCheckEventGroup {
  readonly count: number
  readonly dimensions: {
    readonly healthStatus: string
    readonly originIP: string
    readonly region: string
    readonly fqdn: string
  }
}

// HTTP Requests Adaptive Groups
export interface HTTPAdaptiveGroup {
  readonly count: number
  readonly dimensions: {
    readonly originResponseStatus: number
    readonly clientCountryName: string
    readonly clientRequestHTTPHost: string
  }
  readonly avg: {
    readonly originResponseDurationMs: number
  }
}

// HTTP Requests Edge Country Host
export interface HTTPEdgeCountryHostGroup {
  readonly count: number
  readonly dimensions: {
    readonly edgeResponseStatus: number
    readonly clientCountryName: string
    readonly clientRequestHTTPHost: string
  }
}

// Colo Groups
export interface ColoGroup {
  readonly count: number
  readonly avg?: {
    readonly sampleInterval: number
  }
  readonly dimensions: {
    readonly datetime: string
    readonly coloCode: string
    readonly clientRequestHTTPHost: string
    readonly originResponseStatus: number
  }
  readonly sum: {
    readonly edgeResponseBytes: number
    readonly visits: number
  }
}

// Colo Error Groups (for error-specific metrics)
export interface ColoErrorGroup {
  readonly count: number
  readonly dimensions: {
    readonly coloCode: string
    readonly clientRequestHTTPHost: string
    readonly edgeResponseStatus: number
  }
  readonly sum: {
    readonly edgeResponseBytes: number
    readonly visits: number
  }
}

// Worker Invocations
export interface WorkerInvocation {
  readonly dimensions: {
    readonly scriptName: string
    readonly status: string
  }
  readonly sum: {
    readonly requests: number
    readonly errors: number
    readonly duration: number
  }
  readonly quantiles: {
    readonly cpuTimeP50: number
    readonly cpuTimeP75: number
    readonly cpuTimeP99: number
    readonly cpuTimeP999: number
    readonly durationP50: number
    readonly durationP75: number
    readonly durationP99: number
    readonly durationP999: number
  }
}

// Load Balancer
export interface LoadBalancerGroup {
  readonly count: number
  readonly dimensions: {
    readonly lbName: string
    readonly selectedPoolName: string
    readonly selectedOriginName: string
    readonly region?: string
    readonly proxied?: number
    readonly selectedPoolAvgRttMs?: number
    readonly selectedPoolHealthy?: number
    readonly steeringPolicy?: string
  }
}

export interface LoadBalancerOrigin {
  readonly originName: string
  readonly healthy: number
  readonly originAddress: string
}

export interface LoadBalancerRequest {
  readonly lbName: string
  readonly pools: readonly {
    readonly poolName: string
    readonly healthy: number
    readonly origins?: readonly LoadBalancerOrigin[]
  }[]
}

// Firewall Rules
export interface FirewallRule {
  readonly id: string
  readonly description?: string
  readonly action: string
  readonly filter: {
    readonly id: string
    readonly expression: string
  }
}

export interface FirewallRuleset {
  readonly id: string
  readonly name: string
  readonly description?: string
}

export interface FirewallRulesResponse {
  readonly result: readonly FirewallRule[]
  readonly rulesets?: readonly FirewallRuleset[]
}

// Logpush Health
export interface LogpushHealthGroup {
  readonly count: number
  readonly dimensions: {
    readonly jobId: number
    readonly status: number
    readonly destinationType: string
    readonly datetime: string
    readonly final: number
  }
}

// Magic Transit
export interface MagicTransitGroup {
  readonly count: number
  readonly dimensions: {
    readonly active: number
    readonly datetime: string
    readonly edgeColoCity: string
    readonly edgeColoCountry: string
    readonly edgePopName: string
    readonly remoteTunnelIPv4: string
    readonly resultStatus: string
    readonly siteName: string
    readonly tunnelName: string
  }
}

// SSL Certificate
export interface SSLCertificate {
  readonly id: string
  readonly type: string
  readonly status: string
  readonly issuer: string
  readonly expires_on: string
  readonly hosts: readonly string[]
}

// GraphQL Response Types
export interface HTTPGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequests1mGroups: readonly HTTP1mGroup[]
      readonly firewallEventsAdaptiveGroups: readonly FirewallEventGroup[]
    }[]
  }
}

export interface FirewallGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly firewallEventsAdaptiveGroups: readonly FirewallEventGroup[]
    }[]
  }
}

export interface HealthCheckGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly healthCheckEventsAdaptiveGroups: readonly HealthCheckEventGroup[]
    }[]
  }
}

export interface AdaptiveGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequestsAdaptiveGroups: readonly HTTPAdaptiveGroup[]
    }[]
  }
}

export interface HTTPEdgeCountryResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequestsEdgeCountryHost: readonly HTTPEdgeCountryHostGroup[]
    }[]
  }
}

export interface ColoGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequestsAdaptiveGroups: readonly ColoGroup[]
    }[]
  }
}

export interface ColoErrorGroupsResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequestsAdaptiveGroups: readonly ColoErrorGroup[]
    }[]
  }
}

export interface WorkerTotalsResponse {
  readonly viewer: {
    readonly accounts: readonly {
      readonly workersInvocationsAdaptive: readonly WorkerInvocation[]
    }[]
  }
}

export interface LoadBalancerResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly loadBalancingRequestsAdaptiveGroups: readonly LoadBalancerGroup[]
      readonly loadBalancingRequestsAdaptive: readonly LoadBalancerRequest[]
    }[]
  }
}

export interface LogpushAccountResponse {
  readonly viewer: {
    readonly accounts: readonly {
      readonly logpushHealthAdaptiveGroups: readonly LogpushHealthGroup[]
    }[]
  }
}

export interface LogpushZoneResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly logpushHealthAdaptiveGroups: readonly LogpushHealthGroup[]
    }[]
  }
}

export interface MagicTransitResponse {
  readonly viewer: {
    readonly accounts: readonly {
      readonly magicTransitTunnelHealthChecksAdaptiveGroups: readonly MagicTransitGroup[]
    }[]
  }
}

export interface SSLCertificateResponse {
  readonly result: readonly SSLCertificate[]
}

// Request Method Response
export interface RequestMethodResponse {
  readonly viewer: {
    readonly zones: readonly {
      readonly zoneTag: string
      readonly httpRequestsAdaptiveGroups: readonly {
        readonly count: number
        readonly dimensions: {
          readonly clientRequestHTTPMethodName: string
        }
      }[]
    }[]
  }
}
