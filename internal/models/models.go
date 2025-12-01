package models

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponse struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneResp `json:"zones"`
	} `json:"viewer"`
}

// CloudflareResponseAccts represents the Cloudflare API response for accounts.
type CloudflareResponseAccts struct {
	Viewer struct {
		// Accounts holds the list of AccountResponse data.
		Accounts []AccountResp `json:"accounts"`
	} `json:"viewer"`
}

// CloudflareResponseColo represents the Cloudflare API response for colo groups.
type CloudflareResponseColo struct {
	Viewer struct {
		Zones []ZoneRespColo `json:"zones"`
	} `json:"viewer"`
}

// CloudflareResponseLb represents the Cloudflare API response for load balancing zones.
type CloudflareResponseLb struct {
	Viewer struct {
		Zones []LbResp `json:"zones"`
	} `json:"viewer"`
}

// CloudflareResponseLogpushAccount represents the Cloudflare API response for logpush accounts.
type CloudflareResponseLogpushAccount struct {
	Viewer struct {
		Accounts []LogpushResponse `json:"accounts"`
	} `json:"viewer"`
}

// CloudflareResponseLogpushZone represents the Cloudflare API response for logpush zones.
type CloudflareResponseLogpushZone struct {
	Viewer struct {
		Zones []LogpushResponse `json:"zones"`
	} `json:"viewer"`
}

// LogpushResponse contains the data for logpush health checks.
type LogpushResponse struct {
	LogpushHealthAdaptiveGroups []struct {
		Count uint64 `json:"count"`

		Dimensions struct {
			Datetime        string `json:"datetime"`
			DestinationType string `json:"destinationType"`
			JobID           int    `json:"jobId"`
			Status          int    `json:"status"`
			Final           int    `json:"final"`
		}
	} `json:"logpushHealthAdaptiveGroups"`
}

// AccountResp represents an account's invocations and statistics.
type AccountResp struct {
	WorkersInvocationsAdaptive []struct {
		Dimensions struct {
			ScriptName string `json:"scriptName"`
			Status     string `json:"status"`
		}

		Sum struct {
			Requests uint64  `json:"requests"`
			Errors   uint64  `json:"errors"`
			Duration float64 `json:"duration"`
		} `json:"sum"`

		Quantiles struct {
			CPUTimeP50   float32 `json:"cpuTimeP50"`
			CPUTimeP75   float32 `json:"cpuTimeP75"`
			CPUTimeP99   float32 `json:"cpuTimeP99"`
			CPUTimeP999  float32 `json:"cpuTimeP999"`
			DurationP50  float32 `json:"durationP50"`
			DurationP75  float32 `json:"durationP75"`
			DurationP99  float32 `json:"durationP99"`
			DurationP999 float32 `json:"durationP999"`
		} `json:"quantiles"`
	} `json:"workersInvocationsAdaptive"`
}

// ZoneRespColo represents a zone's data for colo groups.
type ZoneRespColo struct {
	ColoGroups []struct {
		Dimensions struct {
			Datetime             string `json:"datetime"`
			ColoCode             string `json:"coloCode"`
			Host                 string `json:"clientRequestHTTPHost"`
			OriginResponseStatus int    `json:"originResponseStatus"`
		} `json:"dimensions"`
		Count uint64 `json:"count"`
		Sum   struct {
			EdgeResponseBytes uint64 `json:"edgeResponseBytes"`
			Visits            uint64 `json:"visits"`
		} `json:"sum"`
		Avg struct {
			SampleInterval float64 `json:"sampleInterval"`
		} `json:"avg"`
	} `json:"httpRequestsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneResp struct {
	HTTP1mGroups []struct {
		Dimensions struct {
			Datetime string `json:"datetime"`
		} `json:"dimensions"`
		Unique struct {
			Uniques uint64 `json:"uniques"`
		} `json:"uniq"`
		Sum struct {
			Bytes          uint64 `json:"bytes"`
			CachedBytes    uint64 `json:"cachedBytes"`
			CachedRequests uint64 `json:"cachedRequests"`
			Requests       uint64 `json:"requests"`
			BrowserMap     []struct {
				PageViews       uint64 `json:"pageViews"`
				UaBrowserFamily string `json:"uaBrowserFamily"`
			} `json:"browserMap"`
			ClientHTTPVersion []struct {
				Protocol string `json:"clientHTTPProtocol"`
				Requests uint64 `json:"requests"`
			} `json:"clientHTTPVersionMap"`
			ClientSSL []struct {
				Protocol string `json:"clientSSLProtocol"`
			} `json:"clientSSLMap"`
			ContentType []struct {
				Bytes                   uint64 `json:"bytes"`
				Requests                uint64 `json:"requests"`
				EdgeResponseContentType string `json:"edgeResponseContentTypeName"`
			} `json:"contentTypeMap"`
			Country []struct {
				Bytes             uint64 `json:"bytes"`
				ClientCountryName string `json:"clientCountryName"`
				Requests          uint64 `json:"requests"`
				Threats           uint64 `json:"threats"`
			} `json:"countryMap"`
			EncryptedBytes    uint64 `json:"encryptedBytes"`
			EncryptedRequests uint64 `json:"encryptedRequests"`
			IPClass           []struct {
				Type     string `json:"ipType"`
				Requests uint64 `json:"requests"`
			} `json:"ipClassMap"`
			PageViews      uint64 `json:"pageViews"`
			ResponseStatus []struct {
				EdgeResponseStatus int    `json:"edgeResponseStatus"`
				Requests           uint64 `json:"requests"`
			} `json:"responseStatusMap"`
			ThreatPathing []struct {
				Name     string `json:"threatPathingName"`
				Requests uint64 `json:"requests"`
			} `json:"threatPathingMap"`
			Threats uint64 `json:"threats"`
		} `json:"sum"`
	} `json:"httpRequests1mGroups"`

	FirewallEventsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			Action                string `json:"action"`
			Source                string `json:"source"`
			RuleID                string `json:"ruleId"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"firewallEventsAdaptiveGroups"`

	HTTPRequestsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			OriginResponseStatus  uint16 `json:"originResponseStatus"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"httpRequestsAdaptiveGroups"`

	HTTPRequestsEdgeCountryHost []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			EdgeResponseStatus    uint16 `json:"edgeResponseStatus"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"httpRequestsEdgeCountryHost"`

	HealthCheckEventsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			HealthStatus  string `json:"healthStatus"`
			OriginIP      string `json:"originIP"`
			FailureReason string `json:"failureReason"`
			Region        string `json:"region"`
			Fqdn          string `json:"fqdn"`
		} `json:"dimensions"`
	} `json:"healthCheckEventsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}

// LbResp represents LoadBalancingRequestsAdaptiveGroups and LoadBalancingRequestsAdaptive.
type LbResp struct {
	LoadBalancingRequestsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			LbName               string `json:"lbName"`
			Proxied              uint8  `json:"proxied"`
			Region               string `json:"region"`
			SelectedOriginName   string `json:"selectedOriginName"`
			SelectedPoolAvgRttMs uint64 `json:"selectedPoolAvgRttMs"`
			SelectedPoolHealthy  uint8  `json:"selectedPoolHealthy"`
			SelectedPoolName     string `json:"selectedPoolName"`
			SteeringPolicy       string `json:"steeringPolicy"`
		} `json:"dimensions"`
	} `json:"loadBalancingRequestsAdaptiveGroups"`

	LoadBalancingRequestsAdaptive []struct {
		LbName                string `json:"lbName"`
		Proxied               uint8  `json:"proxied"`
		Region                string `json:"region"`
		SelectedPoolHealthy   uint8  `json:"selectedPoolHealthy"`
		SelectedPoolID        string `json:"selectedPoolID"`
		SelectedPoolName      string `json:"selectedPoolName"`
		SessionAffinityStatus string `json:"sessionAffinityStatus"`
		SteeringPolicy        string `json:"steeringPolicy"`
		SelectedPoolAvgRttMs  uint64 `json:"selectedPoolAvgRttMs"`
		Pools                 []struct {
			AvgRttMs uint64 `json:"avgRttMs"`
			Healthy  uint8  `json:"healthy"`
			ID       string `json:"id"`
			PoolName string `json:"poolName"`
		} `json:"pools"`
		Origins []struct {
			OriginName string `json:"originName"`
			Health     uint8  `json:"health"`
			IPv4       string `json:"ipv4"`
			Selected   uint8  `json:"selected"`
		} `json:"origins"`
	} `json:"loadBalancingRequestsAdaptive"`

	ZoneTag string `json:"zoneTag"`
}

// CloudflareResponseMagicTransit represents MagicTransitAccount
type CloudflareResponseMagicTransit struct {
	Viewer struct {
		Accounts []MagicTransitAccount `json:"accounts"`
	} `json:"viewer"`
}

// MagicTransitAccount represents MagicTransitTunnelHealthChecksAdaptiveGroups
type MagicTransitAccount struct {
	MagicTransitTunnelHealthChecksAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			Active           uint8  `json:"active"`           // 1 if the colo had traffic in the last 6 hours
			Datetime         string `json:"datetime"`         // Timestamp of the health check
			EdgeColoCity     string `json:"edgeColoCity"`     // City of the Cloudflare datacenter
			EdgeColoCountry  string `json:"edgeColoCountry"`  // Country of the Cloudflare datacenter
			EdgePopName      string `json:"edgePopName"`      // Name of the Cloudflare POP
			RemoteTunnelIPv4 string `json:"remoteTunnelIPv4"` // IPv4 address of the remote tunnel endpoint
			ResultStatus     string `json:"resultStatus"`     // Status of the health check
			SiteName         string `json:"siteName"`         // Human-friendly name of the site
			TunnelName       string `json:"tunnelName"`       // Human-friendly name of the tunnel
		} `json:"dimensions"`
	} `json:"magicTransitTunnelHealthChecksAdaptiveGroups"`
}

// Certificate to handle SSL
type Certificate struct {
	ID                   string   `json:"id"`
	Type                 string   `json:"type"`
	Status               string   `json:"status"`
	Issuer               string   `json:"issuer"`
	UploadedOn           string   `json:"uploaded_on"`
	ExpiresOn            string   `json:"expires_on"`
	ValidityDays         int      `json:"validity_days"`
	ValidationMethod     string   `json:"validation_method"`
	CertificateAuthority string   `json:"certificate_authority"`
	Hosts                []string `json:"hosts"`
}

// Zone represents ZoneId and Certificate.
type Zone struct {
	ZoneID       string        `json:"zone_id"`
	ZoneName     string        `json:"zone_name"`
	AccountName  string        `json:"account_name"`
	Certificates []Certificate `json:"certificates"`
}

// SSLResponse represents array of Zones.
type SSLResponse struct {
	Result []Zone `json:"result"`
}

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponseHTTPGroups struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneRespHTTPGroups `json:"zones"`
	} `json:"viewer"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneRespHTTPGroups struct {
	HTTP1mGroups []struct {
		Dimensions struct {
			Datetime string `json:"datetime"`
		} `json:"dimensions"`
		Unique struct {
			Uniques uint64 `json:"uniques"`
		} `json:"uniq"`
		Sum struct {
			Bytes          uint64 `json:"bytes"`
			CachedBytes    uint64 `json:"cachedBytes"`
			CachedRequests uint64 `json:"cachedRequests"`
			Requests       uint64 `json:"requests"`
			BrowserMap     []struct {
				PageViews       uint64 `json:"pageViews"`
				UaBrowserFamily string `json:"uaBrowserFamily"`
			} `json:"browserMap"`
			ClientHTTPVersion []struct {
				Protocol string `json:"clientHTTPProtocol"`
				Requests uint64 `json:"requests"`
			} `json:"clientHTTPVersionMap"`
			ClientSSL []struct {
				Protocol string `json:"clientSSLProtocol"`
			} `json:"clientSSLMap"`
			ContentType []struct {
				Bytes                   uint64 `json:"bytes"`
				Requests                uint64 `json:"requests"`
				EdgeResponseContentType string `json:"edgeResponseContentTypeName"`
			} `json:"contentTypeMap"`
			Country []struct {
				Bytes             uint64 `json:"bytes"`
				ClientCountryName string `json:"clientCountryName"`
				Requests          uint64 `json:"requests"`
				Threats           uint64 `json:"threats"`
			} `json:"countryMap"`
			EncryptedBytes    uint64 `json:"encryptedBytes"`
			EncryptedRequests uint64 `json:"encryptedRequests"`
			IPClass           []struct {
				Type     string `json:"ipType"`
				Requests uint64 `json:"requests"`
			} `json:"ipClassMap"`
			PageViews      uint64 `json:"pageViews"`
			ResponseStatus []struct {
				EdgeResponseStatus int    `json:"edgeResponseStatus"`
				Requests           uint64 `json:"requests"`
			} `json:"responseStatusMap"`
			ThreatPathing []struct {
				Name     string `json:"threatPathingName"`
				Requests uint64 `json:"requests"`
			} `json:"threatPathingMap"`
			Threats uint64 `json:"threats"`
		} `json:"sum"`
	} `json:"httpRequests1mGroups"`
	FirewallEventsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			Action                string `json:"action"`
			Source                string `json:"source"`
			RuleID                string `json:"ruleId"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"firewallEventsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponseFirewallGroups struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneRespFirewallGroups `json:"zones"`
	} `json:"viewer"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneRespFirewallGroups struct {
	FirewallEventsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			Action                string `json:"action"`
			Source                string `json:"source"`
			RuleID                string `json:"ruleId"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"firewallEventsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponseHealthCheckGroups struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneRespHealthCheckGroups `json:"zones"`
	} `json:"viewer"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneRespHealthCheckGroups struct {
	HealthCheckEventsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			HealthStatus  string `json:"healthStatus"`
			OriginIP      string `json:"originIP"`
			FailureReason string `json:"failureReason"`
			Region        string `json:"region"`
			Fqdn          string `json:"fqdn"`
		} `json:"dimensions"`
	} `json:"healthCheckEventsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponseHTTPRequestsEdge struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneRespHTTPRequestsEdge `json:"zones"`
	} `json:"viewer"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneRespHTTPRequestsEdge struct {
	HTTPRequestsEdgeCountryHost []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			EdgeResponseStatus    uint16 `json:"edgeResponseStatus"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
	} `json:"httpRequestsEdgeCountryHost"`

	ZoneTag string `json:"zoneTag"`
}

// CloudflareResponse represents the Cloudflare API response for zones.
type CloudflareResponseAdaptiveGroups struct {
	// Viewer contains the list of zones.
	Viewer struct {
		// Zones holds the list of ZoneResponse data.
		Zones []ZoneRespAdaptiveGroups `json:"zones"`
	} `json:"viewer"`
}

// ZoneResp represents a zone's data for HTTP requests, firewall events, and other metrics.
type ZoneRespAdaptiveGroups struct {
	HTTPRequestsAdaptiveGroups []struct {
		Count      uint64 `json:"count"`
		Dimensions struct {
			OriginResponseStatus  uint16 `json:"originResponseStatus"`
			ClientCountryName     string `json:"clientCountryName"`
			ClientRequestHTTPHost string `json:"clientRequestHTTPHost"`
		} `json:"dimensions"`
		Avg struct {
			OriginResponseDurationMs float64 `json:"originResponseDurationMs"`
		}
	} `json:"httpRequestsAdaptiveGroups"`

	ZoneTag string `json:"zoneTag"`
}
