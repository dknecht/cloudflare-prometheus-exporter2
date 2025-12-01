package metrics

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/gammazero/workerpool"
	cloudflareAPI "github.com/lablabs/cloudflare-exporter/internal/cloudflare"
	limiter "github.com/lablabs/cloudflare-exporter/internal/limiter"
	"github.com/lablabs/cloudflare-exporter/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	logging "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// MetricName represent metric name
type MetricName string

func (mn MetricName) String() string {
	return string(mn)
}

const (
	zoneRequestTotalMetricName                   MetricName = "cloudflare_zone_requests_total"
	zoneRequestCachedMetricName                  MetricName = "cloudflare_zone_requests_cached"
	zoneRequestSSLEncryptedMetricName            MetricName = "cloudflare_zone_requests_ssl_encrypted"
	zoneRequestContentTypeMetricName             MetricName = "cloudflare_zone_requests_content_type"
	zoneRequestCountryMetricName                 MetricName = "cloudflare_zone_requests_country"
	zoneRequestHTTPStatusMetricName              MetricName = "cloudflare_zone_requests_status"
	zoneRequestBrowserMapMetricName              MetricName = "cloudflare_zone_requests_browser_map_page_views_count"
	zoneRequestOriginStatusCountryHostMetricName MetricName = "cloudflare_zone_requests_origin_status_country_host" //host
	zoneRequestStatusCountryHostMetricName       MetricName = "cloudflare_zone_requests_status_country_host"        //host
	zoneBandwidthTotalMetricName                 MetricName = "cloudflare_zone_bandwidth_total"
	zoneBandwidthCachedMetricName                MetricName = "cloudflare_zone_bandwidth_cached"
	zoneBandwidthSSLEncryptedMetricName          MetricName = "cloudflare_zone_bandwidth_ssl_encrypted"
	zoneBandwidthContentTypeMetricName           MetricName = "cloudflare_zone_bandwidth_content_type"
	zoneBandwidthCountryMetricName               MetricName = "cloudflare_zone_bandwidth_country"
	zoneThreatsTotalMetricName                   MetricName = "cloudflare_zone_threats_total"
	zoneThreatsCountryMetricName                 MetricName = "cloudflare_zone_threats_country"
	zoneThreatsTypeMetricName                    MetricName = "cloudflare_zone_threats_type"
	zonePageviewsTotalMetricName                 MetricName = "cloudflare_zone_pageviews_total"
	zoneUniquesTotalMetricName                   MetricName = "cloudflare_zone_uniques_total"
	zoneColocationVisitsMetricName               MetricName = "cloudflare_zone_colocation_visits"              //host
	zoneColocationEdgeResponseBytesMetricName    MetricName = "cloudflare_zone_colocation_edge_response_bytes" //host
	zoneColocationRequestsTotalMetricName        MetricName = "cloudflare_zone_colocation_requests_total"      //host
	zoneFirewallEventsCountMetricName            MetricName = "cloudflare_zone_firewall_events_count"
	zoneHealthCheckEventsOriginCountMetricName   MetricName = "cloudflare_zone_health_check_events_origin_count"
	workerRequestsMetricName                     MetricName = "cloudflare_worker_requests_count"
	workerErrorsMetricName                       MetricName = "cloudflare_worker_errors_count"
	workerCPUTimeMetricName                      MetricName = "cloudflare_worker_cpu_time"
	workerDurationMetricName                     MetricName = "cloudflare_worker_duration"
	poolHealthStatusMetricName                   MetricName = "cloudflare_zone_pool_health_status"
	poolRequestsTotalMetricName                  MetricName = "cloudflare_zone_pool_requests_total"
	logpushFailedJobsAccountMetricName           MetricName = "cloudflare_logpush_failed_jobs_account_count"
	logpushFailedJobsZoneMetricName              MetricName = "cloudflare_logpush_failed_jobs_zone_count"
	// new added
	zoneCustomerError4xxRate               MetricName = "cloudflare_zone_customer_error_4xx_rate" //host
	zoneCustomerError5xxRate               MetricName = "cloudflare_zone_customer_error_5xx_rate" //host
	zoneEdgeErrorRate                      MetricName = "cloudflare_zone_edge_error_rate"         //host
	zoneOriginErrorRate                    MetricName = "cloudflare_zone_origin_error_rate"       //host
	zoneBotRequestsByCountry               MetricName = "cloudflare_zone_bot_request_by_country"  //host
	zoneCacheHitRatio                      MetricName = "cloudflare_zone_cache_hit_ratio"
	zoneHealthCheckEventsAdaptiveGroupsAvg MetricName = "cloudflare_zone_health_check_events_avg"
	zoneFirewallBotsDetectedSource         MetricName = "cloudflare_zone_firewall_bots_detected" //host
	zoneFirewallRequestAction              MetricName = "cloudflare_zone_firewall_request_action"
	zoneRequestMethodCount                 MetricName = "cloudflare_zone_request_method_count"
	magicTransitActiveTunnels              MetricName = "cloudflare_magic_transit_active_tunnels"
	magicTransitHealthyTunnels             MetricName = "cloudflare_magic_transit_healthy_tunnels"
	magicTransitTunnelFailures             MetricName = "cloudflare_magic_transit_tunnel_failures"
	magicTransitEdgeColoCount              MetricName = "cloudflare_magic_transit_edge_colo_count"
	zoneCertificateValidationStatus        MetricName = "cloudflare_zone_certificate_validation_status"
	// other new
	zoneOriginResponseDurationMsMetricName         MetricName = "cloudflare_zone_origin_response_duration_ms"
	zoneColocationVisitsErrorMetricName            MetricName = "cloudflare_zone_colocation_visits_error"              //host
	zoneColocationEdgeResponseBytesErrorMetricName MetricName = "cloudflare_zone_colocation_edge_response_bytes_error" //host
	zoneColocationRequestsTotalErrorMetricName     MetricName = "cloudflare_zone_colocation_requests_total_error"      //host
)

// Set map to check metric name availability.
type Set map[MetricName]struct{}

// Has function check and return bool for metric availability.
func (ms Set) Has(mn MetricName) bool {
	_, exists := ms[mn]
	return exists
}

// Add function add metric name.
func (ms Set) Add(mn MetricName) {
	ms[mn] = struct{}{}
}

var (
	// Requests
	zoneRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestTotalMetricName.String(),
		Help: "Number of requests for zone",
	}, []string{"zone", "account"},
	)

	zoneRequestCached = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: zoneRequestCachedMetricName.String(),
		Help: "Number of cached requests for zone",
	}, []string{"zone", "account"},
	)

	zoneRequestSSLEncrypted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestSSLEncryptedMetricName.String(),
		Help: "Number of encrypted requests for zone",
	}, []string{"zone", "account"},
	)

	zoneRequestContentType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestContentTypeMetricName.String(),
		Help: "Number of request for zone per content type",
	}, []string{"zone", "account", "content_type"},
	)

	zoneRequestCountry = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestCountryMetricName.String(),
		Help: "Number of request for zone per country",
	}, []string{"zone", "account", "country"},
	)

	zoneRequestHTTPStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestHTTPStatusMetricName.String(),
		Help: "Number of request for zone per HTTP status",
	}, []string{"zone", "account", "status"},
	)

	zoneRequestBrowserMap = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestBrowserMapMetricName.String(),
		Help: "Number of successful requests for HTML pages per zone",
	}, []string{"zone", "account", "family"},
	)

	zoneBandwidthTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneBandwidthTotalMetricName.String(),
		Help: "Total bandwidth per zone in bytes",
	}, []string{"zone", "account"},
	)

	zoneBandwidthCached = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneBandwidthCachedMetricName.String(),
		Help: "Cached bandwidth per zone in bytes",
	}, []string{"zone", "account"},
	)

	zoneBandwidthSSLEncrypted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneBandwidthSSLEncryptedMetricName.String(),
		Help: "Encrypted bandwidth per zone in bytes",
	}, []string{"zone", "account"},
	)

	zoneBandwidthContentType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneBandwidthContentTypeMetricName.String(),
		Help: "Bandwidth per zone per content type",
	}, []string{"zone", "account", "content_type"},
	)

	zoneBandwidthCountry = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneBandwidthCountryMetricName.String(),
		Help: "Bandwidth per country per zone",
	}, []string{"zone", "account", "country"},
	)

	zoneThreatsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneThreatsTotalMetricName.String(),
		Help: "Threats per zone",
	}, []string{"zone", "account"},
	)

	zoneThreatsCountry = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneThreatsCountryMetricName.String(),
		Help: "Threats per zone per country",
	}, []string{"zone", "account", "country"},
	)

	zoneThreatsType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneThreatsTypeMetricName.String(),
		Help: "Threats per zone per type",
	}, []string{"zone", "account", "type"},
	)

	zonePageviewsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zonePageviewsTotalMetricName.String(),
		Help: "Pageviews per zone",
	}, []string{"zone", "account"},
	)

	zoneUniquesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneUniquesTotalMetricName.String(),
		Help: "Uniques per zone",
	}, []string{"zone", "account"},
	)

	zoneFirewallEventsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneFirewallEventsCountMetricName.String(),
		Help: "Count of Firewall events",
	}, []string{"zone", "account"},
	)

	zoneHealthCheckEventsOriginCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneHealthCheckEventsOriginCountMetricName.String(),
		Help: "Number of Heath check events per region per origin",
	}, []string{"zone", "account", "health_status", "origin_ip", "fqdn"},
	)

	workerRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: workerRequestsMetricName.String(),
		Help: "Number of requests sent to worker by script name",
	}, []string{"script_name", "account"},
	)

	workerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: workerErrorsMetricName.String(),
		Help: "Number of errors by script name",
	}, []string{"script_name", "account"},
	)

	workerCPUTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: workerCPUTimeMetricName.String(),
		Help: "CPU time quantiles by script name",
	}, []string{"script_name", "account", "quantile"},
	)

	workerDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: workerDurationMetricName.String(),
		Help: "Duration quantiles by script name (GB*s)",
	}, []string{"script_name", "account", "quantile"},
	)

	poolHealthStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: poolHealthStatusMetricName.String(),
		Help: "Reports the health of a pool, 1 for healthy, 0 for unhealthy.",
	},
		[]string{"zone", "account", "load_balancer_name", "pool_name"},
	)

	poolRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: poolRequestsTotalMetricName.String(),
		Help: "Requests per pool",
	},
		[]string{"zone", "account", "load_balancer_name", "pool_name", "origin_name"},
	)

	logpushFailedJobsAccount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cloudflare_logpush_failed_jobs_account_count",
		Help: "Number of failed logpush jobs on the account level",
	},
		[]string{"account", "account_type", "destination", "job_id", "final"},
	)

	logpushFailedJobsZone = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: logpushFailedJobsZoneMetricName.String(),
		Help: "Number of failed logpush jobs on the zone level",
	},
		[]string{"destination", "job_id", "final"},
	)

	zoneCacheHit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: zoneCacheHitRatio.String(),
		Help: "Number fo cache hit ratio",
	}, []string{"zone", "account", "cachedRequests", "requests"},
	)

	zoneHealthCheckEventsAvg = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: zoneHealthCheckEventsAdaptiveGroupsAvg.String(),
		Help: "Number fo cache hit ratio",
	}, []string{"zone", "account"},
	)

	zoneFirewallAction = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneFirewallRequestAction.String(),
		Help: "Number of Firewall events",
	}, []string{"zone", "account", "action"},
	)

	zoneRequestMethod = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: zoneRequestMethodCount.String(),
		Help: "Number of zone request method",
	}, []string{"zone", "account", "method"},
	)
	magicTransitActiveTunnel = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: magicTransitActiveTunnels.String(),
			Help: "Number of active Magic Transit tunnels",
		},
		[]string{"account", "account_type"},
	)
	magicTransitHealthyTunnel = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: magicTransitHealthyTunnels.String(),
			Help: "Number of healthy Magic Transit tunnels",
		},
		[]string{"account", "account_type"},
	)
	magicTransitTunnelFailure = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: magicTransitTunnelFailures.String(),
			Help: "Number of failed Magic Transit tunnels",
		},
		[]string{"account", "account_type"},
	)
	magicTransitEdgeColo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: magicTransitEdgeColoCount.String(),
			Help: "Number of edge colocation sites involved in Magic Transit tunnels",
		},
		[]string{"account", "account_type"},
	)

	zoneCertificateValidation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: zoneCertificateValidationStatus.String(),
			Help: "SSL certificate status for a given zone",
		},
		[]string{"zone_id", "zone_name", "status", "issuer"},
	)
)

func getLabels(baseLabels prometheus.Labels, hostValue string) prometheus.Labels {

	exclude_host := viper.GetBool("exclude_host")

	// Check if "exclude_host" is false and add "host" dynamically
	if !exclude_host {
		baseLabels["host"] = hostValue
	}

	return baseLabels
}

// BuildAllMetricsSet helps to build all metric and return as Set.
func BuildAllMetricsSet() Set {
	allMetricsSet := Set{}
	allMetricsSet.Add(zoneRequestTotalMetricName)
	allMetricsSet.Add(zoneRequestCachedMetricName)
	allMetricsSet.Add(zoneRequestSSLEncryptedMetricName)
	allMetricsSet.Add(zoneRequestContentTypeMetricName)
	allMetricsSet.Add(zoneRequestCountryMetricName)
	allMetricsSet.Add(zoneRequestHTTPStatusMetricName)
	allMetricsSet.Add(zoneRequestBrowserMapMetricName)
	allMetricsSet.Add(zoneRequestOriginStatusCountryHostMetricName)
	allMetricsSet.Add(zoneRequestStatusCountryHostMetricName)
	allMetricsSet.Add(zoneBandwidthTotalMetricName)
	allMetricsSet.Add(zoneBandwidthCachedMetricName)
	allMetricsSet.Add(zoneBandwidthSSLEncryptedMetricName)
	allMetricsSet.Add(zoneBandwidthContentTypeMetricName)
	allMetricsSet.Add(zoneBandwidthCountryMetricName)
	allMetricsSet.Add(zoneThreatsTotalMetricName)
	allMetricsSet.Add(zoneThreatsCountryMetricName)
	allMetricsSet.Add(zoneThreatsTypeMetricName)
	allMetricsSet.Add(zonePageviewsTotalMetricName)
	allMetricsSet.Add(zoneUniquesTotalMetricName)
	allMetricsSet.Add(zoneColocationVisitsMetricName)
	allMetricsSet.Add(zoneColocationEdgeResponseBytesMetricName)
	allMetricsSet.Add(zoneColocationRequestsTotalMetricName)
	allMetricsSet.Add(zoneFirewallEventsCountMetricName)
	allMetricsSet.Add(zoneHealthCheckEventsOriginCountMetricName)
	allMetricsSet.Add(workerRequestsMetricName)
	allMetricsSet.Add(workerErrorsMetricName)
	allMetricsSet.Add(workerCPUTimeMetricName)
	allMetricsSet.Add(workerDurationMetricName)
	allMetricsSet.Add(poolHealthStatusMetricName)
	allMetricsSet.Add(poolRequestsTotalMetricName)
	allMetricsSet.Add(logpushFailedJobsAccountMetricName)
	allMetricsSet.Add(logpushFailedJobsZoneMetricName)
	// new
	allMetricsSet.Add(zoneCustomerError4xxRate)
	allMetricsSet.Add(zoneCustomerError5xxRate)
	allMetricsSet.Add(zoneEdgeErrorRate)
	allMetricsSet.Add(zoneOriginErrorRate)
	allMetricsSet.Add(zoneBotRequestsByCountry)
	allMetricsSet.Add(zoneHealthCheckEventsAdaptiveGroupsAvg)
	allMetricsSet.Add(zoneFirewallBotsDetectedSource)
	allMetricsSet.Add(zoneFirewallRequestAction)
	allMetricsSet.Add(zoneRequestMethodCount)
	allMetricsSet.Add(magicTransitActiveTunnels)
	allMetricsSet.Add(magicTransitEdgeColoCount)
	allMetricsSet.Add(magicTransitHealthyTunnels)
	allMetricsSet.Add(magicTransitTunnelFailures)
	allMetricsSet.Add(zoneCertificateValidationStatus)
	// other new
	allMetricsSet.Add(zoneOriginResponseDurationMsMetricName)
	allMetricsSet.Add(zoneColocationVisitsErrorMetricName)
	allMetricsSet.Add(zoneColocationEdgeResponseBytesErrorMetricName)
	allMetricsSet.Add(zoneColocationRequestsTotalErrorMetricName)

	return allMetricsSet
}

// BuildDeniedMetricsSet returns Set and error.
func BuildDeniedMetricsSet(metricsDenylist []string) (Set, error) {
	deniedMetricsSet := Set{}
	allMetricsSet := BuildAllMetricsSet()
	for _, metric := range metricsDenylist {
		if !allMetricsSet.Has(MetricName(metric)) {
			return nil, fmt.Errorf("metric %s doesn't exists", metric)
		}
		deniedMetricsSet.Add(MetricName(metric))
	}
	return deniedMetricsSet, nil
}

var zoneRequestOriginStatusCountryHost *prometheus.CounterVec
var zoneRequestStatusCountryHost *prometheus.CounterVec
var zoneColocationVisits *prometheus.CounterVec
var zoneColocationEdgeResponseBytes *prometheus.CounterVec
var zoneColocationRequestsTotal *prometheus.CounterVec
var zoneCustomerError4xx *prometheus.CounterVec
var zoneCustomerError5xx *prometheus.CounterVec
var zoneEdgeError *prometheus.GaugeVec
var zoneOriginError *prometheus.CounterVec
var zoneFirewallBotsDetected *prometheus.CounterVec
var zoneBotRequests *prometheus.CounterVec

// other new added
var zoneOriginResponseDuration *prometheus.GaugeVec
var zoneColocationVisitsError *prometheus.CounterVec
var zoneColocationEdgeResponseBytesError *prometheus.CounterVec
var zoneColocationRequestsTotalError *prometheus.CounterVec

// MustRegisterMetrics register the metrics.
func MustRegisterMetrics(deniedMetrics Set) {
	if !deniedMetrics.Has(zoneRequestTotalMetricName) {
		prometheus.MustRegister(zoneRequestTotal)
	}
	if !deniedMetrics.Has(zoneRequestCachedMetricName) {
		prometheus.MustRegister(zoneRequestCached)
	}
	if !deniedMetrics.Has(zoneRequestSSLEncryptedMetricName) {
		prometheus.MustRegister(zoneRequestSSLEncrypted)
	}
	if !deniedMetrics.Has(zoneRequestContentTypeMetricName) {
		prometheus.MustRegister(zoneRequestContentType)
	}
	if !deniedMetrics.Has(zoneRequestCountryMetricName) {
		prometheus.MustRegister(zoneRequestCountry)
	}
	if !deniedMetrics.Has(zoneRequestHTTPStatusMetricName) {
		prometheus.MustRegister(zoneRequestHTTPStatus)
	}
	if !deniedMetrics.Has(zoneRequestBrowserMapMetricName) {
		prometheus.MustRegister(zoneRequestBrowserMap)
	}
	if !deniedMetrics.Has(zoneRequestOriginStatusCountryHostMetricName) {
		if zoneRequestOriginStatusCountryHost == nil { // Ensure it is not nil before registration
			metricLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneRequestOriginStatusCountryHost = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneRequestOriginStatusCountryHostMetricName.String(),
					Help: "Count of not cached requests for zone per origin HTTP status per country per host",
				},
				metricLabels,
			)

			prometheus.MustRegister(zoneRequestOriginStatusCountryHost)
		}
	}
	if !deniedMetrics.Has(zoneRequestStatusCountryHostMetricName) {
		if zoneRequestStatusCountryHost == nil { // Ensure it is not nil before registration
			metricLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneRequestStatusCountryHost = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneRequestStatusCountryHostMetricName.String(),
					Help: "Count of requests for zone per edge HTTP status per country per host",
				},
				metricLabels,
			)

			prometheus.MustRegister(zoneRequestStatusCountryHost)
		}
	}
	if !deniedMetrics.Has(zoneBandwidthTotalMetricName) {
		prometheus.MustRegister(zoneBandwidthTotal)
	}
	if !deniedMetrics.Has(zoneBandwidthCachedMetricName) {
		prometheus.MustRegister(zoneBandwidthCached)
	}
	if !deniedMetrics.Has(zoneBandwidthSSLEncryptedMetricName) {
		prometheus.MustRegister(zoneBandwidthSSLEncrypted)
	}
	if !deniedMetrics.Has(zoneBandwidthContentTypeMetricName) {
		prometheus.MustRegister(zoneBandwidthContentType)
	}
	if !deniedMetrics.Has(zoneBandwidthCountryMetricName) {
		prometheus.MustRegister(zoneBandwidthCountry)
	}
	if !deniedMetrics.Has(zoneThreatsTotalMetricName) {
		prometheus.MustRegister(zoneThreatsTotal)
	}
	if !deniedMetrics.Has(zoneThreatsCountryMetricName) {
		prometheus.MustRegister(zoneThreatsCountry)
	}
	if !deniedMetrics.Has(zoneThreatsTypeMetricName) {
		prometheus.MustRegister(zoneThreatsType)
	}
	if !deniedMetrics.Has(zonePageviewsTotalMetricName) {
		prometheus.MustRegister(zonePageviewsTotal)
	}
	if !deniedMetrics.Has(zoneUniquesTotalMetricName) {
		prometheus.MustRegister(zoneUniquesTotal)
	}
	if !deniedMetrics.Has(zoneColocationVisitsMetricName) {
		if zoneColocationVisits == nil { // Ensure it is not nil before registration
			metricLabels1 := []string{"zone", "account", "colocation"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels1 = append(metricLabels1, "host") // Conditionally add "host"
			}

			zoneColocationVisits = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationVisitsMetricName.String(),
					Help: "Total visits per colocation",
				},
				metricLabels1,
			)

			prometheus.MustRegister(zoneColocationVisits)
		}
	}
	if !deniedMetrics.Has(zoneColocationEdgeResponseBytesMetricName) {
		if zoneColocationEdgeResponseBytes == nil { // Ensure it is not nil before registration
			metricLabels2 := []string{"zone", "account", "colocation"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels2 = append(metricLabels2, "host") // Conditionally add "host"
			}

			zoneColocationEdgeResponseBytes = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationEdgeResponseBytesMetricName.String(),
					Help: "Edge response bytes per colocation",
				},
				metricLabels2,
			)

			prometheus.MustRegister(zoneColocationEdgeResponseBytes)
		}
	}
	if !deniedMetrics.Has(zoneColocationRequestsTotalMetricName) {
		if zoneColocationRequestsTotal == nil { // Ensure it is not nil before registration
			metricLabels3 := []string{"zone", "account", "colocation"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels3 = append(metricLabels3, "host") // Conditionally add "host"
			}

			zoneColocationRequestsTotal = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationRequestsTotalMetricName.String(),
					Help: "Total requests per colocation",
				},
				metricLabels3,
			)

			prometheus.MustRegister(zoneColocationRequestsTotal)
		}
	}
	if !deniedMetrics.Has(zoneFirewallEventsCountMetricName) {
		prometheus.MustRegister(zoneFirewallEventsCount)
	}
	if !deniedMetrics.Has(zoneHealthCheckEventsOriginCountMetricName) {
		prometheus.MustRegister(zoneHealthCheckEventsOriginCount)
	}
	if !deniedMetrics.Has(workerRequestsMetricName) {
		prometheus.MustRegister(workerRequests)
	}
	if !deniedMetrics.Has(workerErrorsMetricName) {
		prometheus.MustRegister(workerErrors)
	}
	if !deniedMetrics.Has(workerCPUTimeMetricName) {
		prometheus.MustRegister(workerCPUTime)
	}
	if !deniedMetrics.Has(workerDurationMetricName) {
		prometheus.MustRegister(workerDuration)
	}
	if !deniedMetrics.Has(poolHealthStatusMetricName) {
		prometheus.MustRegister(poolHealthStatus)
	}
	if !deniedMetrics.Has(poolRequestsTotalMetricName) {
		prometheus.MustRegister(poolRequestsTotal)
	}
	if !deniedMetrics.Has(logpushFailedJobsAccountMetricName) {
		prometheus.MustRegister(logpushFailedJobsAccount)
	}
	if !deniedMetrics.Has(logpushFailedJobsZoneMetricName) {
		prometheus.MustRegister(logpushFailedJobsZone)
	}
	// new
	if !deniedMetrics.Has(zoneCustomerError4xxRate) {
		if zoneCustomerError4xx == nil { // Ensure it is not nil before registration
			metricLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneCustomerError4xx = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneCustomerError4xxRate.String(),
					Help: "Number of error rates of 4xx",
				},
				metricLabels,
			)

			prometheus.MustRegister(zoneCustomerError4xx)
		}
	}
	if !deniedMetrics.Has(zoneCustomerError5xxRate) {
		if zoneCustomerError5xx == nil { // Ensure it is not nil before registration
			metricLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneCustomerError5xx = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneCustomerError5xxRate.String(),
					Help: "Number of error rates of 5xx",
				},
				metricLabels,
			)

			prometheus.MustRegister(zoneCustomerError5xx)
		}
	}
	if !deniedMetrics.Has(zoneEdgeErrorRate) {
		if zoneEdgeError == nil { // Ensure it is not nil before registration
			var metricLabels = []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneEdgeError = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: zoneEdgeErrorRate.String(),
					Help: "Number of error rate of 4xx and 5xx",
				},
				metricLabels, // Correctly pass the label slice
			)

			prometheus.MustRegister(zoneEdgeError)
		}
	}
	if !deniedMetrics.Has(zoneOriginErrorRate) {
		if zoneOriginError == nil { // Ensure it is not nil before registration
			metricLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabels = append(metricLabels, "host") // Conditionally add "host"
			}

			zoneOriginError = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneOriginErrorRate.String(),
					Help: "Number of error rates of 4xx and 5xx in HTTP requests",
				},
				metricLabels,
			)

			prometheus.MustRegister(zoneOriginError)
		}
	}
	if !deniedMetrics.Has(zoneBotRequestsByCountry) {
		if zoneBotRequests == nil { // Ensure it is not nil before registration
			zoneBotRequestsMetricLabels := []string{"zone", "account", "country", "action"}

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				zoneBotRequestsMetricLabels = append(zoneBotRequestsMetricLabels, "host")
			}

			zoneBotRequests = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "cloudflare_zone_bot_request_by_country",
					Help: "Number of bot requests over country",
				},
				zoneBotRequestsMetricLabels,
			)

			prometheus.MustRegister(zoneBotRequests)
		}
	}
	if !deniedMetrics.Has(zoneCacheHitRatio) {
		prometheus.MustRegister(zoneCacheHit)
	}
	if !deniedMetrics.Has(zoneHealthCheckEventsAdaptiveGroupsAvg) {
		prometheus.MustRegister(zoneHealthCheckEventsAvg)
	}
	if !deniedMetrics.Has(zoneFirewallBotsDetectedSource) {
		if zoneFirewallBotsDetected == nil { // Ensure it is not nil before registration
			zoneFirewallBotsDetectedLabels := []string{"zone", "account", "source", "action"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				zoneFirewallBotsDetectedLabels = append(zoneFirewallBotsDetectedLabels, "host") // Conditionally add "host"
			}

			zoneFirewallBotsDetected = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneFirewallBotsDetectedSource.String(),
					Help: "Number of bot requests over country",
				},
				zoneFirewallBotsDetectedLabels,
			)

			prometheus.MustRegister(zoneFirewallBotsDetected)
		}
	}
	if !deniedMetrics.Has(zoneFirewallRequestAction) {
		prometheus.MustRegister(zoneFirewallAction)
	}
	if !deniedMetrics.Has(zoneRequestMethodCount) {
		prometheus.MustRegister(zoneRequestMethod)
	}
	if !deniedMetrics.Has(magicTransitActiveTunnels) {
		prometheus.MustRegister(magicTransitActiveTunnel)
	}
	if !deniedMetrics.Has(magicTransitEdgeColoCount) {
		prometheus.MustRegister(magicTransitEdgeColo)
	}
	if !deniedMetrics.Has(magicTransitHealthyTunnels) {
		prometheus.MustRegister(magicTransitHealthyTunnel)
	}
	if !deniedMetrics.Has(magicTransitTunnelFailures) {
		prometheus.MustRegister(magicTransitTunnelFailure)
	}
	if !deniedMetrics.Has(zoneCertificateValidationStatus) {
		prometheus.MustRegister(zoneCertificateValidation)
	}
	if !deniedMetrics.Has(zoneOriginResponseDurationMsMetricName) {
		if zoneOriginResponseDuration == nil { // Ensure it is not nil before registration
			zoneOriginResponseDurationMsLabels := []string{"zone", "account", "status", "country"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				zoneOriginResponseDurationMsLabels = append(zoneOriginResponseDurationMsLabels, "host") // Conditionally add "host"
			}

			zoneOriginResponseDuration = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: zoneOriginResponseDurationMsMetricName.String(),
					Help: "Zone Origin Response Time MS",
				},
				zoneOriginResponseDurationMsLabels, // Correctly pass the label slice
			)

			prometheus.MustRegister(zoneOriginResponseDuration)
		}
	}
	if !deniedMetrics.Has(zoneColocationVisitsErrorMetricName) {
		if zoneColocationVisitsError == nil { // Ensure it is not nil before registration
			metricLabelsError1 := []string{"zone", "account", "colocation", "status"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabelsError1 = append(metricLabelsError1, "host") // Conditionally add "host"
			}

			zoneColocationVisitsError = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationVisitsErrorMetricName.String(),
					Help: "Total visits per colocation with error code",
				},
				metricLabelsError1,
			)

			prometheus.MustRegister(zoneColocationVisitsError)
		}
	}
	if !deniedMetrics.Has(zoneColocationEdgeResponseBytesErrorMetricName) {
		if zoneColocationEdgeResponseBytesError == nil { // Ensure it is not nil before registration
			metricLabelsError2 := []string{"zone", "account", "colocation", "status"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabelsError2 = append(metricLabelsError2, "host") // Conditionally add "host"
			}

			zoneColocationEdgeResponseBytesError = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationEdgeResponseBytesErrorMetricName.String(),
					Help: "Edge response bytes per colocation with error code",
				},
				metricLabelsError2,
			)

			prometheus.MustRegister(zoneColocationEdgeResponseBytesError)
		}
	}
	if !deniedMetrics.Has(zoneColocationRequestsTotalErrorMetricName) {
		if zoneColocationRequestsTotalError == nil { // Ensure it is not nil before registration
			metricLabelsError3 := []string{"zone", "account", "colocation", "status"} // Base labels

			exclude_host := viper.GetBool("exclude_host")

			if !exclude_host {
				metricLabelsError3 = append(metricLabelsError3, "host") // Conditionally add "host"
			}

			zoneColocationRequestsTotalError = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: zoneColocationRequestsTotalErrorMetricName.String(),
					Help: "Total requests per colocation with error code",
				},
				metricLabelsError3,
			)

			prometheus.MustRegister(zoneColocationRequestsTotalError)
		}
	}

}

// FetchWorkerAnalytics handles cloudflare account and expose metrics like requests, error, Worker CPUTime and Duration.
func FetchWorkerAnalytics(account cloudflare.Account) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in FetchWorkerAnalytics", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	// Replace spaces with hyphens and convert to lowercase
	accountName := strings.ToLower(strings.ReplaceAll(account.Name, " ", "-"))

	r, err := cloudflareAPI.FetchWorkerTotals(account.ID)
	if err != nil {
		// Return early if API call fails, keeping default metrics
		logging.Error("FetchWorkerAnalytics: Failed to fetch worker totals", map[string]interface{}{
			"accountID": account.ID,
			"error":     err.Error(),
		})
		return
	}

	for _, a := range r.Viewer.Accounts {
		if len(a.WorkersInvocationsAdaptive) == 0 {
			// Ensure metrics for "unknown" are set when no worker data is present
			// initializeDefaultMetrics(accountName, "unknown")
			continue
		}

		for _, w := range a.WorkersInvocationsAdaptive {
			// Add actual metrics
			workerRequests.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName}).Add(float64(w.Sum.Requests))
			workerErrors.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName}).Add(float64(w.Sum.Errors))
			workerCPUTime.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P50"}).Set(float64(w.Quantiles.CPUTimeP50))
			workerCPUTime.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P75"}).Set(float64(w.Quantiles.CPUTimeP75))
			workerCPUTime.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P99"}).Set(float64(w.Quantiles.CPUTimeP99))
			workerCPUTime.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P999"}).Set(float64(w.Quantiles.CPUTimeP999))
			workerDuration.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P50"}).Set(math.Round(float64(w.Quantiles.DurationP50)*1000) / 1000)
			workerDuration.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P75"}).Set(math.Round(float64(w.Quantiles.DurationP75)*1000) / 1000)
			workerDuration.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P99"}).Set(math.Round(float64(w.Quantiles.DurationP99)*1000) / 1000)
			workerDuration.With(prometheus.Labels{"script_name": w.Dimensions.ScriptName, "account": accountName, "quantile": "P999"}).Set(math.Round(float64(w.Quantiles.DurationP999)*1000) / 1000)
		}
	}
}

// filterZones helper function to filter the zones.
func filterZones(all []cloudflare.Zone, target []string) []cloudflare.Zone {
	var filtered []cloudflare.Zone

	if (len(target)) == 0 {
		return all
	}

	for _, tz := range target {
		for _, z := range all {
			if tz == z.ID {
				filtered = append(filtered, z)
				logging.Info("Filtering zone: ", z.ID, " ", z.Name)
			}
		}
	}
	return filtered
}

// getTargetZones helper function to get targeted zones.
func getTargetZones() []string {
	var zoneIDs []string
	if len(viper.GetString("cf_zones")) > 0 {
		zoneIDs = strings.Split(viper.GetString("cf_zones"), ",")
	} else {
		// deprecated
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "ZONE_") {
				split := strings.SplitN(e, "=", 2)
				zoneIDs = append(zoneIDs, split[1])
			}
		}
	}
	return zoneIDs
}

// getExcludedZones returns array of excluded zones.
func getExcludedZones() []string {
	var zoneIDs []string

	if len(viper.GetString("cf_exclude_zones")) > 0 {
		zoneIDs = strings.Split(viper.GetString("cf_exclude_zones"), ",")
	}
	return zoneIDs
}

func allZonesAreEmpty(account []models.LogpushResponse) bool {
	// Check if all zones are empty
	for _, zone := range account {
		if len(zone.LogpushHealthAdaptiveGroups) > 0 {
			return false
		}
	}
	return true // All zones are empty
}

// fetchLogpushAnalyticsForAccount expose metrics related to logpush.
func fetchLogpushAnalyticsForAccount(account cloudflare.Account) {
	defer func() { // Panic Recovery
		if r := recover(); r != nil {
			logging.Error("Recovered from panic in fetchLogpushAnalyticsForAccount", map[string]interface{}{
				"accountID": account.ID,
				"panic":     r,
			})
		}
	}()

	r, err := cloudflareAPI.FetchLogpushAccount(account.ID)
	if err != nil {
		logging.Error("Failed to fetch logpush health data", map[string]interface{}{
			"accountID": account.ID,
			"error":     err.Error(),
		})

		return
	}

	if r == nil || r.Viewer.Accounts == nil {
		return
	}

	// Check if the API response is empty and handle accordingly
	if len(r.Viewer.Accounts) == 0 || allZonesAreEmpty(r.Viewer.Accounts) {

		return
	}

	// Process metrics from the API response
	for _, acc := range r.Viewer.Accounts {
		for _, LogpushHealthAdaptiveGroup := range acc.LogpushHealthAdaptiveGroups {
			logpushFailedJobsAccount.With(prometheus.Labels{
				"account":      account.Name,
				"account_type": account.Type,
				"destination":  LogpushHealthAdaptiveGroup.Dimensions.DestinationType,
				"job_id":       strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.JobID),
				"final":        strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.Final),
			}).Add(float64(LogpushHealthAdaptiveGroup.Count))
		}
	}
}

func fetchMagicTransitHealth(account cloudflare.Account) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchMagicTransitHealth", map[string]interface{}{
				"accountID": account.ID,
				"panic":     r,
			})
			// Optionally, set default metrics here as well
		}
	}()

	// Fetch data from the Magic Transit API
	r, err := cloudflareAPI.MagicTransitTunnelHealthChecksAdaptiveGroups(account.ID)
	if err != nil {
		logging.Error("Failed to fetch Magic Transit data", map[string]interface{}{
			"accountID": account.ID,
			"error":     err.Error(),
		})
		return
	}

	// Check if the API response is empty and handle accordingly
	if r == nil || len(r.Viewer.Accounts) == 0 {
		return
	}

	// Initialize metrics
	var activeTunnels, healthyTunnels, tunnelFailures, edgeColoCount float64

	// Process metrics from the API response
	for _, acc := range r.Viewer.Accounts {
		fmt.Println(":::::::::::::::::::::::::::::", acc.MagicTransitTunnelHealthChecksAdaptiveGroups)
		for _, group := range acc.MagicTransitTunnelHealthChecksAdaptiveGroups {
			if group.Dimensions.Active == 1 {
				activeTunnels++
			}
			if group.Dimensions.ResultStatus == "healthy" {
				healthyTunnels++
			} else {
				tunnelFailures++
			}
			if group.Dimensions.EdgePopName != "" {
				edgeColoCount++
			}
		}
	}

	// Set Prometheus metrics
	magicTransitActiveTunnel.With(prometheus.Labels{"account": account.Name, "account_type": account.Type}).Set(activeTunnels)
	magicTransitHealthyTunnel.With(prometheus.Labels{"account": account.Name, "account_type": account.Type}).Set(healthyTunnels)
	magicTransitTunnelFailure.With(prometheus.Labels{"account": account.Name, "account_type": account.Type}).Set(tunnelFailures)
	magicTransitEdgeColo.With(prometheus.Labels{"account": account.Name, "account_type": account.Type}).Set(edgeColoCount)
}

func filterNonFreePlanZones(zones []cloudflare.Zone) (filteredZones []cloudflare.Zone) {

	for _, z := range zones {
		if z.Plan.ZonePlanCommon.ID != "0feeeeeeeeeeeeeeeeeeeeeeeeeeeeee" {
			filteredZones = append(filteredZones, z)
		}
	}

	return
}

func findZoneAccountName(zones []cloudflare.Zone, ID string) (string, string) {

	for _, z := range zones {

		if strings.TrimSpace(z.ID) == strings.TrimSpace(ID) {

			return z.Name, strings.ToLower(strings.ReplaceAll(z.Account.Name, " ", "-"))
		}
	}

	return "", ""
}

func fetchZoneAnalytics(ctx context.Context, zones []cloudflare.Zone) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchZoneAnalytics", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	// None of the below referenced metrics are available in the free tier
	if viper.GetBool("free_tier") {
		return
	}

	zoneIDs := cloudflareAPI.ExtractZoneIDs(filterNonFreePlanZones(zones))
	if len(zoneIDs) == 0 {
		return
	}

	batchSize := 5 // Process 5 zones at a time

	for i := 0; i < len(zoneIDs); i += batchSize {
		batch := zoneIDs[i:min(i+batchSize, len(zoneIDs))]

		// Parallel fetch per metric type
		httpData, err := cloudflareAPI.FetchHTTPMetrics(ctx, batch)
		if err != nil {
			logging.Error("Failed to fetch HTTP metrics", err)
			continue
		}

		firewallData, err := cloudflareAPI.FetchFirewallMetrics(ctx, batch)
		if err != nil {
			logging.Error("Failed to fetch firewallData", err)
			continue
		}

		healthCheckEventsAdaptiveData, err := cloudflareAPI.HealthCheckEventsAdaptiveMetrics(ctx, batch)
		if err != nil {
			logging.Error("Failed to fetch healthCheckEventsAdaptiveData", err)
			continue
		}

		httpRequestsAdaptiveGroupsData, err := cloudflareAPI.HTTPRequestsAdaptiveMetrics(ctx, batch)
		if err != nil {
			logging.Error("Failed to fetch httpRequestsAdaptiveGroupsData", err)
			continue
		}

		httpRequestsEdgeCountryHostData, err := cloudflareAPI.HTTPRequestsEdgeCountryMetrics(ctx, batch)
		if err != nil {
			logging.Error("Failed to fetch httpRequestsEdgeCountryHostData", err)
			continue
		}

		for _, z := range httpData.Viewer.Zones {
			name, account := findZoneAccountName(zones, z.ZoneTag)
			currentZone := z
			addHTTPGroups(&currentZone, name, account)
		}
		for _, z := range firewallData.Viewer.Zones {
			name, account := findZoneAccountName(zones, z.ZoneTag)
			currentZone := z
			addFirewallGroups(&currentZone, name, account)
		}
		for _, z := range healthCheckEventsAdaptiveData.Viewer.Zones {
			name, account := findZoneAccountName(zones, z.ZoneTag)
			currentZone := z
			addHealthCheckGroups(&currentZone, name, account)
		}
		for _, z := range httpRequestsAdaptiveGroupsData.Viewer.Zones {
			name, account := findZoneAccountName(zones, z.ZoneTag)
			currentZone := z
			addHTTPAdaptiveGroups(&currentZone, name, account)
		}
		for _, z := range httpRequestsEdgeCountryHostData.Viewer.Zones {
			name, account := findZoneAccountName(zones, z.ZoneTag)
			currentZone := z
			addHTTPRequestsEdgeCountryHost(&currentZone, name, account)
		}
	}
}

func addHTTPGroups(z *models.ZoneRespHTTPGroups, name string, account string) {

	if z == nil {
		logging.Error("Received nil zone response in addHTTPGroups", nil)
		return
	}

	// Nothing to do if HTTP1mGroups is empty
	if len(z.HTTP1mGroups) == 0 {
		return
	}

	zt := z.HTTP1mGroups[0]

	// Update metrics with actual data
	zoneRequestTotal.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.Requests))
	zoneRequestCached.With(prometheus.Labels{"zone": name, "account": account}).Set(float64(zt.Sum.CachedRequests))
	zoneRequestSSLEncrypted.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.EncryptedRequests))

	for _, ct := range zt.Sum.ContentType {
		zoneRequestContentType.With(prometheus.Labels{"zone": name, "account": account, "content_type": ct.EdgeResponseContentType}).Add(float64(ct.Requests))
		zoneBandwidthContentType.With(prometheus.Labels{"zone": name, "account": account, "content_type": ct.EdgeResponseContentType}).Add(float64(ct.Bytes))
	}

	for _, country := range zt.Sum.Country {

		zoneRequestCountry.With(prometheus.Labels{"zone": name, "account": account, "country": country.ClientCountryName}).Add(float64(country.Requests))
		zoneBandwidthCountry.With(prometheus.Labels{"zone": name, "account": account, "country": country.ClientCountryName}).Add(float64(country.Bytes))
		zoneThreatsCountry.With(prometheus.Labels{"zone": name, "account": account, "country": country.ClientCountryName}).Add(float64(country.Threats))
	}

	groupStatus := viper.GetBool("cf_http_status_group")

	if groupStatus {
		// Grouped: 2xx, 4xx, etc.
		statusGroups := map[string]uint64{
			"1xx": 0,
			"2xx": 0,
			"3xx": 0,
			"4xx": 0,
			"5xx": 0,
		}

		for _, status := range zt.Sum.ResponseStatus {
			code := status.EdgeResponseStatus
			switch {
			case code < 200:
				statusGroups["1xx"] += status.Requests
			case code < 300:
				statusGroups["2xx"] += status.Requests
			case code < 400:
				statusGroups["3xx"] += status.Requests
			case code < 500:
				statusGroups["4xx"] += status.Requests
			default:
				statusGroups["5xx"] += status.Requests
			}
		}

		for group, count := range statusGroups {
			zoneRequestHTTPStatus.With(prometheus.Labels{
				"zone":    name,
				"account": account,
				"status":  group,
			}).Add(float64(count))
		}
	} else {
		// Individual: 200, 401, 503, etc.
		for _, status := range zt.Sum.ResponseStatus {
			codeStr := strconv.Itoa(status.EdgeResponseStatus)
			zoneRequestHTTPStatus.With(prometheus.Labels{
				"zone":    name,
				"account": account,
				"status":  codeStr,
			}).Add(float64(status.Requests))
		}
	}

	for _, browser := range zt.Sum.BrowserMap {
		zoneRequestBrowserMap.With(prometheus.Labels{"zone": name, "account": account, "family": browser.UaBrowserFamily}).Add(float64(browser.PageViews))
	}

	zoneBandwidthTotal.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.Bytes))
	zoneBandwidthCached.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.CachedBytes))
	zoneBandwidthSSLEncrypted.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.EncryptedBytes))

	zoneThreatsTotal.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.Threats))

	for _, t := range zt.Sum.ThreatPathing {
		zoneThreatsType.With(prometheus.Labels{"zone": name, "account": account, "type": t.Name}).Add(float64(t.Requests))
	}

	zonePageviewsTotal.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Sum.PageViews))

	// Uniques
	zoneUniquesTotal.With(prometheus.Labels{"zone": name, "account": account}).Add(float64(zt.Unique.Uniques))

	zoneCacheHit.With(
		prometheus.Labels{
			"zone":           name,
			"account":        account,
			"requests":       strconv.FormatUint(zt.Sum.Requests, 10),
			"cachedRequests": strconv.FormatUint(zt.Sum.CachedRequests, 10),
		}).Set(float64(zt.Sum.CachedRequests) / float64(zt.Sum.Requests))

	// Map to track HTTP method counts
	methodCounts := make(map[string]float64)

	// Loop through firewall events
	for _, g := range z.FirewallEventsAdaptiveGroups {
		// Extract ClientRequestHTTPHost or other dimensions
		httpMethod := g.Dimensions.ClientRequestHTTPHost // Adjust based on available data

		// Increment the count for this HTTP method
		methodCounts[httpMethod] += float64(g.Count)
	}

	// Push metrics to Prometheus
	for method, count := range methodCounts {
		zoneRequestMethod.With(prometheus.Labels{
			"zone":    name,
			"account": account,
			"method":  method, // The HTTP method dimension
		}).Add(count)
	}
}

func addFirewallGroups(z *models.ZoneRespFirewallGroups, name string, account string) {

	if z == nil {
		logging.Error("Received nil zone response in Firewall group", nil)
		return
	}

	// Nothing to do if there are no FirewallEventsAdaptiveGroups
	if len(z.FirewallEventsAdaptiveGroups) == 0 {
		return
	}

	// Fetch firewall rules map
	// rulesMap := cloudflareAPI.FetchFirewallRules(z.ZoneTag)

	// Process each firewall event group
	for _, g := range z.FirewallEventsAdaptiveGroups {
		zoneFirewallEventsCount.With(
			prometheus.Labels{
				"zone":    name,
				"account": account,
			}).Add(float64(g.Count))

		zoneFirewallAction.With(
			prometheus.Labels{
				"zone":    name,
				"account": account,
				"action":  g.Dimensions.Action,
			}).Add(float64(g.Count))

		// Generate labels dynamically using getLabels()
		zoneBotRequestsLabels := getLabels(prometheus.Labels{
			"zone":    name,
			"account": account,
			"country": g.Dimensions.ClientCountryName, // Keep dynamic values
			"action":  g.Dimensions.Action,
			// "rule":    normalizeRuleName(rulesMap[g.Dimensions.RuleID]),
		}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

		if zoneBotRequests != nil {
			// Use generated labels with Prometheus metric
			zoneBotRequests.With(zoneBotRequestsLabels).Add(float64(g.Count))
		}

		// Generate labels dynamically using getLabels()
		labels := getLabels(prometheus.Labels{
			"zone":    name,
			"account": account,
			"source":  g.Dimensions.Source,
			"action":  g.Dimensions.Action,
			// "rule":    normalizeRuleName(rulesMap[g.Dimensions.RuleID]),
		}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

		// Use the dynamically generated labels with Prometheus metric
		// zoneFirewallBotsDetected.With(labels).Add(float64(g.Count))
		if zoneFirewallBotsDetected != nil { //  Prevents nil pointer error
			zoneFirewallBotsDetected.With(labels).Add(float64(g.Count))
		}

	}

}

func addHealthCheckGroups(z *models.ZoneRespHealthCheckGroups, name string, account string) {

	if z == nil {
		logging.Error("Received nil zone response in Health check group", nil)
		return
	}

	// Nothing to do if there are no HealthCheckEventsAdaptiveGroups
	if len(z.HealthCheckEventsAdaptiveGroups) == 0 {
		return
	}

	var totalEvents uint64
	var totalCount int

	// Process each health check event group
	for _, g := range z.HealthCheckEventsAdaptiveGroups {
		// Add the count of events to the total
		totalEvents += g.Count
		totalCount++

		zoneHealthCheckEventsOriginCount.With(
			prometheus.Labels{
				"zone":          name,
				"account":       account,
				"health_status": g.Dimensions.HealthStatus,
				"origin_ip":     g.Dimensions.OriginIP,
				// "region":        g.Dimensions.Region,
				"fqdn": g.Dimensions.Fqdn,
			}).Add(float64(g.Count))
	}

	// Calculate the average health check events
	var avgHealthCheckEvents float64
	if totalCount > 0 {
		avgHealthCheckEvents = float64(totalEvents) / float64(totalCount)
	}

	zoneHealthCheckEventsAvg.With(
		prometheus.Labels{
			"zone":    name,
			"account": account,
		}).Set(avgHealthCheckEvents)
}

func addHTTPAdaptiveGroups(z *models.ZoneRespAdaptiveGroups, name string, account string) {

	if z == nil {
		logging.Error("Received nil zone response in HTTP Adaptive Group", nil)
		return
	}

	// Process `HTTPRequestsAdaptiveGroups`
	for _, g := range z.HTTPRequestsAdaptiveGroups {
		labels := getLabels(prometheus.Labels{
			"zone":    name,
			"account": account,
			"status":  strconv.Itoa(int(g.Dimensions.OriginResponseStatus)),
			"country": g.Dimensions.ClientCountryName,
		}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

		if zoneRequestOriginStatusCountryHost != nil {
			zoneRequestOriginStatusCountryHost.With(labels).Add(float64(g.Count))
		}

	}

	// Process `HTTPRequestsAdaptiveGroups`
	for _, g := range z.HTTPRequestsAdaptiveGroups {
		labels := getLabels(prometheus.Labels{
			"zone":    name,
			"account": account,
			"status":  strconv.Itoa(int(g.Dimensions.OriginResponseStatus)),
			"country": g.Dimensions.ClientCountryName,
		}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

		if zoneOriginResponseDuration != nil {
			zoneOriginResponseDuration.With(labels).Set(g.Avg.OriginResponseDurationMs)
		}

	}

	// Process `` and EdgeResponseStatus for 4xx
	for _, g := range z.HTTPRequestsAdaptiveGroups {
		statusCode := g.Dimensions.OriginResponseStatus

		// Check if OriginResponseStatus is zero (default value) to skip invalid groups
		if statusCode == 0 {
			logging.Debug("Skipping group without valid origin response status", map[string]interface{}{
				"zone":          name,
				"account":       account,
				"clientHost":    g.Dimensions.ClientRequestHTTPHost,
				"clientCountry": g.Dimensions.ClientCountryName,
			})
			continue
		}

		// Check if the status code is a 4xx error
		if statusCode >= 400 && statusCode < 500 {
			// Exclude edge-specific errors like 499 (Client Disconnect)
			if statusCode == 499 {
				logging.Debug("Skipping edge error (499 - Client Disconnect)", map[string]interface{}{
					"zone":          name,
					"account":       account,
					"clientHost":    g.Dimensions.ClientRequestHTTPHost,
					"clientCountry": g.Dimensions.ClientCountryName,
				})
				continue
			}
			// Generate labels dynamically using getLabels()
			labels := getLabels(prometheus.Labels{
				"zone":    name,
				"account": account,
				"status":  strconv.Itoa(int(g.Dimensions.OriginResponseStatus)),
				"country": g.Dimensions.ClientCountryName,
			}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

			if zoneCustomerError4xx != nil {
				// Increment the Prometheus metric for 4xx errors
				zoneCustomerError4xx.With(labels).Add(float64(g.Count))
			}
		}
	}

	// Process `` and EdgeResponseStatus for 5xx
	for _, g := range z.HTTPRequestsAdaptiveGroups {

		// Check if OriginResponseStatus is zero (default value) to skip invalid groups
		if g.Dimensions.OriginResponseStatus == 0 {
			logging.Debug("Skipping group without valid origin response status", map[string]interface{}{
				"zone":          name,
				"account":       account,
				"clientHost":    g.Dimensions.ClientRequestHTTPHost,
				"clientCountry": g.Dimensions.ClientCountryName,
			})
			continue
		}

		statusCode := g.Dimensions.OriginResponseStatus

		// Check if the status code is a 5xx error
		if statusCode >= 500 {
			// Generate labels dynamically using getLabels()
			labels := getLabels(prometheus.Labels{
				"zone":    name,
				"account": account,
				"status":  strconv.Itoa(int(g.Dimensions.OriginResponseStatus)),
				"country": g.Dimensions.ClientCountryName,
			}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

			if zoneCustomerError5xx != nil {
				// Increment the Prometheus metric for 5xx errors
				zoneCustomerError5xx.With(labels).Add(float64(g.Count))
			}

		}

	}

}

func addHTTPRequestsEdgeCountryHost(z *models.ZoneRespHTTPRequestsEdge, name string, account string) {

	if z == nil {
		logging.Error("Received nil zone response in HTTP Adaptive Group", nil)
		return
	}

	// Process `HTTPRequestsEdgeCountryHost` for OriginResponseStatus
	for _, g := range z.HTTPRequestsEdgeCountryHost {
		labels := getLabels(prometheus.Labels{
			"zone":    name,
			"account": account,
			"status":  strconv.Itoa(int(g.Dimensions.EdgeResponseStatus)),
			"country": g.Dimensions.ClientCountryName,
		}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

		if zoneRequestStatusCountryHost != nil {
			zoneRequestStatusCountryHost.With(labels).Add(float64(g.Count))
		}

	}

	// Process `HTTPRequestsEdgeCountryHost` and EdgeResponseStatus for 5xx
	for _, g := range z.HTTPRequestsEdgeCountryHost {
		statusCode := g.Dimensions.EdgeResponseStatus

		// Check if the status code is a 4xx or 5xx error
		if (statusCode >= 400 && statusCode < 500) || (statusCode >= 500 && statusCode < 600) {
			// Generate labels dynamically using getLabels()
			labels := getLabels(prometheus.Labels{
				"zone":    name,
				"account": account,
				"status":  strconv.Itoa(int(g.Dimensions.EdgeResponseStatus)),
				"country": g.Dimensions.ClientCountryName,
			}, g.Dimensions.ClientRequestHTTPHost) // Pass host dynamically

			if zoneEdgeError != nil {
				// Increment the Prometheus metric for edge errors
				zoneEdgeError.With(labels).Inc()
			}

		}

	}

}

//

func fetchZoneColocationAnalytics(zones []cloudflare.Zone) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchZoneColocationAnalytics", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	// Colocation metrics are not available in non-enterprise zones
	if viper.GetBool("free_tier") {
		return
	}

	zoneIDs := cloudflareAPI.ExtractZoneIDs(filterNonFreePlanZones(zones))
	if len(zoneIDs) == 0 {
		return
	}

	r, err := cloudflareAPI.FetchColoTotals(zoneIDs)
	if err != nil {
		logging.Error("Failed to fetch Colo totals", map[string]interface{}{
			"zoneIDs": zoneIDs,
			"error":   err.Error(),
		})
		return
	}

	// Check if the response structure is valid
	if r == nil || r.Viewer.Zones == nil {
		logging.Error("Nil response received for Colo totals", map[string]interface{}{
			"zoneIDs": zoneIDs,
		})
		return
	}

	for _, z := range r.Viewer.Zones {
		cg := z.ColoGroups
		name, account := findZoneAccountName(zones, z.ZoneTag)

		for _, c := range cg {
			labels := getLabels(prometheus.Labels{
				"zone":       name,
				"account":    account,
				"colocation": c.Dimensions.ColoCode,
			}, c.Dimensions.Host) // Pass actual host dynamically

			if zoneColocationVisits != nil {
				zoneColocationVisits.With(labels).Add(float64(c.Sum.Visits))
			}
			if zoneColocationEdgeResponseBytes != nil {
				zoneColocationEdgeResponseBytes.With(labels).Add(float64(c.Sum.EdgeResponseBytes))
			}
			if zoneColocationRequestsTotal != nil {
				zoneColocationRequestsTotal.With(labels).Add(float64(c.Count))
			}

			// Only process error status codes (4xx/5xx)
			status := c.Dimensions.OriginResponseStatus

			if status >= 400 {
				// Create error-specific labels
				errorLabels := getLabels(prometheus.Labels{
					"zone":       name,
					"account":    account,
					"colocation": c.Dimensions.ColoCode,
					"status":     fmt.Sprintf("%dxx", status/100),
				}, c.Dimensions.Host) // Pass actual host dynamically

				// Error-specific metrics
				if zoneColocationVisitsError != nil {
					zoneColocationVisitsError.With(errorLabels).Add(float64(c.Sum.Visits))
				}
				if zoneColocationEdgeResponseBytesError != nil {
					zoneColocationEdgeResponseBytesError.With(errorLabels).Add(float64(c.Sum.EdgeResponseBytes))
				}
				if zoneColocationRequestsTotalError != nil {
					zoneColocationRequestsTotalError.With(errorLabels).Add(float64(c.Count))
				}
			}

		}

	}
}

func fetchLoadBalancerAnalytics(zones []cloudflare.Zone) {

	// Panic recovery to ensure one failing goroutine does not stop the service
	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchLoadBalancerAnalytics", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	// None of the below referenced metrics are available in the free tier
	if viper.GetBool("free_tier") {
		return
	}

	zoneIDs := cloudflareAPI.ExtractZoneIDs(filterNonFreePlanZones(zones))
	if len(zoneIDs) == 0 {
		return
	}

	l, err := cloudflareAPI.FetchLoadBalancerTotals(zoneIDs)
	if err != nil {
		logging.Error("Failed to fetch Load Balancer totals", map[string]interface{}{
			"zoneIDs": zoneIDs,
			"error":   err.Error(),
		})
		return
	}

	for _, lb := range l.Viewer.Zones {
		name, account := findZoneAccountName(zones, lb.ZoneTag)
		lb := lb
		addLoadBalancingRequestsAdaptive(&lb, name, account)
		addLoadBalancingRequestsAdaptiveGroups(&lb, name, account)
	}
}

func addLoadBalancingRequestsAdaptiveGroups(z *models.LbResp, name string, account string) {

	if z == nil {
		logging.Info("Received nil zone response in addLoadBalancingRequestsAdaptiveGroups", nil)
		return
	}

	if len(z.LoadBalancingRequestsAdaptiveGroups) == 0 {

		return
	}
	for _, g := range z.LoadBalancingRequestsAdaptiveGroups {
		poolRequestsTotal.With(
			prometheus.Labels{
				"zone":               name,
				"account":            account,
				"load_balancer_name": g.Dimensions.LbName,
				"pool_name":          g.Dimensions.SelectedPoolName,
				"origin_name":        g.Dimensions.SelectedOriginName,
			}).Add(float64(g.Count))
	}
}

func addLoadBalancingRequestsAdaptive(z *models.LbResp, name string, account string) {

	if z == nil {
		logging.Info("Received nil zone response in addLoadBalancingRequestsAdaptive", nil)
		return
	}

	if len(z.LoadBalancingRequestsAdaptive) == 0 {

		return
	}
	for _, g := range z.LoadBalancingRequestsAdaptive {
		for _, p := range g.Pools {
			poolHealthStatus.With(
				prometheus.Labels{
					"zone":               name,
					"account":            account,
					"load_balancer_name": g.LbName,
					"pool_name":          p.PoolName,
				}).Set(float64(p.Healthy))
		}
	}
}

func fetchLogpushAnalyticsForZone(zones []cloudflare.Zone) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchLogpushAnalyticsForZone", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	if viper.GetBool("free_tier") {
		return
	}

	zoneIDs := cloudflareAPI.ExtractZoneIDs(filterNonFreePlanZones(zones))
	if len(zoneIDs) == 0 {
		return
	}

	r, err2 := cloudflareAPI.FetchLogpushZone(zoneIDs)
	if err2 != nil {

		return
	}

	// Check if the API response is empty and handle accordingly
	if len(r.Viewer.Zones) == 0 || allZonesAreEmpty(r.Viewer.Zones) {

		return
	}

	for _, zone := range r.Viewer.Zones {
		for _, LogpushHealthAdaptiveGroup := range zone.LogpushHealthAdaptiveGroups {
			if LogpushHealthAdaptiveGroup.Count == 0 {
				// Default values in case of no data
				logpushFailedJobsZone.With(prometheus.Labels{
					"destination": LogpushHealthAdaptiveGroup.Dimensions.DestinationType,
					"job_id":      strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.JobID),
					"final":       strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.Final),
				}).Add(0)
			} else {
				logpushFailedJobsZone.With(prometheus.Labels{
					"destination": LogpushHealthAdaptiveGroup.Dimensions.DestinationType,
					"job_id":      strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.JobID),
					"final":       strconv.Itoa(LogpushHealthAdaptiveGroup.Dimensions.Final),
				}).Add(float64(LogpushHealthAdaptiveGroup.Count))
			}
		}
	}
}

func fetchSSLCertificateStatus(zones []cloudflare.Zone) {

	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in fetchSSLCertificateStatus", map[string]interface{}{
				"panic": r,
			})
		}
	}()

	if viper.GetBool("free_tier") {
		return
	}

	zoneIDs := cloudflareAPI.ExtractZoneIDs(filterNonFreePlanZones(zones))
	if len(zoneIDs) == 0 {
		return
	}
	// Fetch SSL certificate status for the zones
	r, err := cloudflareAPI.FetchSSLCertificateStatus(zoneIDs)
	if err != nil {
		logging.Error("Error fetching SSL certificate status", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	if r == nil {
		logging.Error("Received nil response from FetchSSLCertificateStatus", map[string]interface{}{
			"zoneIDs": zoneIDs,
		})
		return
	}

	// Loop through the response and create Prometheus metrics
	for _, zone := range r.Result {
		// Example: Extract certificate data
		for _, certificate := range zone.Certificates {
			// Create a label with necessary details
			certificateStatus := certificate.Status // active, expired, etc.

			// Convert the string to a time.Time object
			expiresOnTime, err := time.Parse(time.RFC3339Nano, certificate.ExpiresOn)
			if err != nil {
				logging.Warnf("Invalid time format for certificate in zone %s: %v", zone.ZoneID, err)
				continue
			}

			// Convert to Unix timestamp (float64)
			expiresOnTimestamp := float64(expiresOnTime.Unix())

			// Check for zone name
			zoneName := "unknown"
			if len(certificate.Hosts) > 0 {
				// If the first element is a wildcard and there's a second element, use that.
				if strings.HasPrefix(certificate.Hosts[0], "*.") && len(certificate.Hosts) > 1 {
					zoneName = certificate.Hosts[1]
				} else {
					zoneName = certificate.Hosts[0]
				}
			}

			// Set the value for the metric
			zoneCertificateValidation.With(prometheus.Labels{
				"zone_id":   zone.ZoneID,
				"zone_name": zoneName,
				"status":    certificateStatus,
				"issuer":    certificate.Issuer,
			}).Set(expiresOnTimestamp)
		}
	}

}

// worker pool ::::::
func FetchMetrics(ctx context.Context, pool *workerpool.WorkerPool) error {
	fmt.Println("FetchMetrics started")

	// Reuse ALL your existing processing logic
	zones, accounts, err := fetchInitialData(ctx)
	if err != nil {
		return err
	}

	filteredZones := cloudflareAPI.FilterExcludedZones(
		filterZones(zones, getTargetZones()), getExcludedZones(),
	)

	// Minimal changes below...
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	// Process accounts - NO CHANGES to your functions
	for _, account := range accounts {
		acc := account
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()

			// Add rate limiting for each API call
			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			FetchWorkerAnalytics(acc)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchLogpushAnalyticsForAccount(acc)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fmt.Println("::::::::::::::::before calling")
			fetchMagicTransitHealth(acc)
		})
	}

	// Process zones - NO CHANGES to your functions
	batchSize := viper.GetInt("cf_batch_size")
	for len(filteredZones) > 0 {
		batch := filteredZones[:min(batchSize, len(filteredZones))]
		filteredZones = filteredZones[len(batch):]

		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchZoneAnalytics(ctx, batch)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchZoneColocationAnalytics(batch)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchLoadBalancerAnalytics(batch)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchLogpushAnalyticsForZone(batch)

			if err := limiter.Wait(ctx); err != nil {
				logging.Error("Rate limit exceeded in worker", err)
				return
			}
			fetchSSLCertificateStatus(batch)
		})
	}

	// Safe wait with context
	go func() { wg.Wait(); close(errChan) }()
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Helper functions
func fetchInitialData(ctx context.Context) ([]cloudflare.Zone, []cloudflare.Account, error) {
	// / Add rate limiting before each API call
	if err := limiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("rate limit wait failed: %w", err)
	}
	zones, err := cloudflareAPI.FetchZones(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch zones: %w", err)
	}

	if err := limiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("rate limit wait failed: %w", err)
	}
	accounts, err := cloudflareAPI.FetchAccounts(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}

	return zones, accounts, nil
}
