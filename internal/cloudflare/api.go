package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/machinebox/graphql"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"

	"github.com/lablabs/cloudflare-exporter/internal/limiter"
	"github.com/lablabs/cloudflare-exporter/internal/models"
	logging "github.com/sirupsen/logrus"

	_ "net/http/pprof"
)

func init() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}

var (
	cfGraphQLEndpoint = "https://api.cloudflare.com/client/v4/graphql/"
)

// Cloudflare's API limits: 1200 requests/5min = 4 requests/sec (with burst of 2)
var apiLimiter = rate.NewLimiter(rate.Every(250*time.Millisecond), 2) // 4 RPS, burst=2

func WaitForRateLimit(ctx context.Context) error {
	return apiLimiter.Wait(ctx) // Call this before each API request
}

func FetchZones(ctx context.Context) ([]cloudflare.Zone, error) {
	var api *cloudflare.API
	var err error

	// Initialize the API client with appropriate credentials.
	if token := viper.GetString("cf_api_token"); token != "" {
		api, err = cloudflare.NewWithAPIToken(token)
	} else {
		api, err = cloudflare.New(viper.GetString("cf_api_key"), viper.GetString("cf_api_email"))
	}
	if err != nil {
		logging.Error("Failed to initialize Cloudflare API client", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	logging.Info("Fetching zones from Cloudflare API", nil)

	// Retry mechanism with exponential backoff
	const maxRetries = 3
	var zones []cloudflare.Zone

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a new context with a 30s timeout for each attempt
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		zones, err = api.ListZones(reqCtx)
		cancel()

		if err == nil {
			logging.Info("Successfully fetched zones", map[string]interface{}{
				"zone_count": len(zones),
			})
			return zones, nil
		}

		// Handle timeout-specific errors separately
		if errors.Is(err, context.DeadlineExceeded) {
			logging.Warn("Cloudflare API request timed out", map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
		} else if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
			logging.Warn("Network timeout while fetching zones", map[string]interface{}{
				"attempt": attempt, "error": err.Error(),
			})
		} else if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "temporary") {
			logging.Warn("Possible DNS failure or no internet connection", map[string]interface{}{
				"attempt": attempt, "error": err.Error(),
			})
		} else {
			logging.Warn("Failed to fetch zones from Cloudflare API, retrying...", map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
		}

		// Exponential backoff (2s, 4s, 6s)
		time.Sleep(time.Duration(attempt*2) * time.Second)

		// Backoff with context awareness
		select {
		case <-time.After(time.Duration(attempt*2) * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err() // Respect parent cancellation
		}
	}

	// Final failure after retries
	logging.Error("Exceeded max retries for fetching zones from Cloudflare API", map[string]interface{}{
		"error": err.Error(),
	})
	return nil, err
}

// FetchAccounts function returns accounts in an array with retry logic.
func FetchAccounts(ctx context.Context) ([]cloudflare.Account, error) {
	var api *cloudflare.API
	var err error
	if len(viper.GetString("cf_api_token")) > 0 {
		api, err = cloudflare.NewWithAPIToken(viper.GetString("cf_api_token"))
	} else {
		api, err = cloudflare.New(viper.GetString("cf_api_key"), viper.GetString("cf_api_email"))
	}
	// Handle API client initialization error
	if err != nil {
		logging.Error("Failed to initialize Cloudflare API client", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Define retry parameters
	const maxRetries = 3
	var accounts []cloudflare.Account

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a context with timeout to prevent hanging requests
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		accounts, _, err = api.Accounts(reqCtx, cloudflare.AccountsListParams{
			PaginationOptions: cloudflare.PaginationOptions{PerPage: 100},
		})
		cancel()
		if err == nil {
			// Log success and return
			logging.Info("Successfully fetched accounts", map[string]interface{}{
				"account_count": len(accounts),
			})
			return accounts, nil
		}

		// Log retry attempt
		logging.Warn("Failed to fetch accounts from Cloudflare API, retrying...", map[string]interface{}{
			"attempt": attempt,
			"error":   err.Error(),
		})

		// Exponential backoff
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	// Log final failure
	logging.Error("Exceeded max retries for fetching accounts from Cloudflare API", map[string]interface{}{
		"error": err.Error(),
	})
	return nil, err
}

func FetchHTTPMetrics(ctx context.Context, zoneIDs []string) (*models.CloudflareResponseHTTPGroups, error) {
	if err := limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!)  {
			viewer {
				zones(filter: { zoneTag_in: $zoneIDs }) {
					zoneTag
					httpRequests1mGroups(limit: $limit filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
						uniq {
							uniques
						}
						sum {
							browserMap {
								pageViews
								uaBrowserFamily
							}
							bytes
							cachedBytes
							cachedRequests
							clientHTTPVersionMap {
								clientHTTPProtocol
								requests
							}
							clientSSLMap {
								clientSSLProtocol
								requests
							}
							contentTypeMap {
								bytes
								requests
								edgeResponseContentTypeName
							}
							countryMap {
								bytes
								clientCountryName
								requests
								threats
							}
							encryptedBytes
							encryptedRequests
							ipClassMap {
								ipType
								requests
							}
							pageViews
							requests
							responseStatusMap {
								edgeResponseStatus
								requests
							}
							threatPathingMap {
								requests
								threatPathingName
							}
							threats
						}
						dimensions {
							datetime
						}
					}
					firewallEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
						count
						dimensions {
						action
						source
						ruleId
						clientRequestHTTPHost
						clientCountryName
						}
					}
				}
			}
		}
		`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)

	// Log the query parameters for debugging
	logging.Info("Fetching FetchHTTPMetrics from Cloudflare API", map[string]interface{}{
		"zoneIDs":    zoneIDs,
		"limit":      viper.GetInt("cf_query_limit"),
		"maxtime":    now,
		"mintime":    now1mAgo,
		"time_range": fmt.Sprintf("%s - %s", now1mAgo, now),
	})

	var resp models.CloudflareResponseHTTPGroups
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to FetchHTTPMetrics", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully FetchHTTPMetrics", map[string]interface{}{
		"zone_count": len(resp.Viewer.Zones),
	})

	return &resp, nil
}

func FetchFirewallMetrics(ctx context.Context, zoneIDs []string) (*models.CloudflareResponseFirewallGroups, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!)  {
			viewer {
				zones(filter: { zoneTag_in: $zoneIDs }) {
					zoneTag
					firewallEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
						count
						dimensions {
						action
						source
						ruleId
						clientRequestHTTPHost
						clientCountryName
						}
					}
				}
			}
		}
		`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)

	// Log the query parameters for debugging
	logging.Info("Fetching FetchFirewallMetrics from Cloudflare API", map[string]interface{}{
		"zoneIDs":    zoneIDs,
		"limit":      viper.GetInt("cf_query_limit"),
		"maxtime":    now,
		"mintime":    now1mAgo,
		"time_range": fmt.Sprintf("%s - %s", now1mAgo, now),
	})

	var resp models.CloudflareResponseFirewallGroups
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to FetchFirewallMetrics totals", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully fetched zone totals", map[string]interface{}{
		"zone_count": len(resp.Viewer.Zones),
	})

	return &resp, nil
}

func HealthCheckEventsAdaptiveMetrics(ctx context.Context, zoneIDs []string) (*models.CloudflareResponseHealthCheckGroups, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!)  {
			viewer {
				zones(filter: { zoneTag_in: $zoneIDs }) {
					zoneTag
					healthCheckEventsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
						count
						dimensions {
							healthStatus
							originIP
							region
							fqdn
						}
					}
				}
			}
		}
		`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)

	// Log the query parameters for debugging
	logging.Info("Fetching HealthCheckGroupMetrics from Cloudflare API", map[string]interface{}{
		"zoneIDs":    zoneIDs,
		"limit":      viper.GetInt("cf_query_limit"),
		"maxtime":    now,
		"mintime":    now1mAgo,
		"time_range": fmt.Sprintf("%s - %s", now1mAgo, now),
	})

	var resp models.CloudflareResponseHealthCheckGroups
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to HealthCheckEventsAdaptiveMetrics", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully fetched HealthCheckEventsAdaptiveMetrics", map[string]interface{}{
		"zone_count": len(resp.Viewer.Zones),
	})

	return &resp, nil
}

func HTTPRequestsAdaptiveMetrics(ctx context.Context, zoneIDs []string) (*models.CloudflareResponseAdaptiveGroups, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!)  {
			viewer {
				zones(filter: { zoneTag_in: $zoneIDs }) {
					zoneTag
					httpRequestsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime, cacheStatus_notin: ["hit"], originResponseStatus_in: [400, 404, 500, 502, 503, 504, 522, 523, 524] }) {
						count
						dimensions {
							originResponseStatus
							clientCountryName
							clientRequestHTTPHost
						}
						avg {
          					originResponseDurationMs
        				}
					}
				}
			}
		}
		`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)

	// Log the query parameters for debugging
	logging.Info("Fetching zone totals from Cloudflare API", map[string]interface{}{
		"zoneIDs":    zoneIDs,
		"limit":      viper.GetInt("cf_query_limit"),
		"maxtime":    now,
		"mintime":    now1mAgo,
		"time_range": fmt.Sprintf("%s - %s", now1mAgo, now),
	})

	var resp models.CloudflareResponseAdaptiveGroups
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to HTTPRequestsAdaptiveMetrics totals", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully HTTPRequestsAdaptiveMetrics totals", map[string]interface{}{
		"zone_count": len(resp.Viewer.Zones),
	})

	return &resp, nil
}

func HTTPRequestsEdgeCountryMetrics(ctx context.Context, zoneIDs []string) (*models.CloudflareResponseHTTPRequestsEdge, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!)  {
			viewer {
				zones(filter: { zoneTag_in: $zoneIDs }) {
					zoneTag
					httpRequestsEdgeCountryHost: httpRequestsAdaptiveGroups(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime }) {
						count
						dimensions {
							edgeResponseStatus
							clientCountryName
							clientRequestHTTPHost
						}
					}
				}
			}
		}
		`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)

	// Log the query parameters for debugging
	logging.Info("Fetching zone totals from Cloudflare API", map[string]interface{}{
		"zoneIDs":    zoneIDs,
		"limit":      viper.GetInt("cf_query_limit"),
		"maxtime":    now,
		"mintime":    now1mAgo,
		"time_range": fmt.Sprintf("%s - %s", now1mAgo, now),
	})

	var resp models.CloudflareResponseHTTPRequestsEdge
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to HTTPRequestsAdaptiveMetrics totals", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully HTTPRequestsEdgeCountryMetrics totals", map[string]interface{}{
		"zone_count": len(resp.Viewer.Zones),
	})

	return &resp, nil
}

//

// FetchWorkerTotals function query workersInvocationsAdaptive
func FetchWorkerTotals(accountID string) (*models.CloudflareResponseAccts, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
		query ($accountID: String!, $mintime: Time!, $maxtime: Time!, $limit: Int!) {
			viewer {
				accounts(filter: {accountTag: $accountID} ) {
					workersInvocationsAdaptive(limit: $limit, filter: { datetime_geq: $mintime, datetime_lt: $maxtime}) {
						dimensions {
							scriptName
							status
							datetime
						}

						sum {
							requests
							errors
							duration
						}

						quantiles {
							cpuTimeP50
							cpuTimeP75
							cpuTimeP99
							cpuTimeP999
							durationP50
							durationP75
							durationP99
							durationP999
						}
					}
				}
			}
		}
	`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("accountID", accountID)

	// Log the query parameters for debugging
	logging.Info("Fetching worker totals for Cloudflare account", map[string]interface{}{
		"accountID":         accountID,
		"limit":             viper.GetInt("cf_query_limit"),
		"maxtime":           now,
		"mintime":           now1mAgo,
		"cfGraphQLEndpoint": cfGraphQLEndpoint,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseAccts
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to fetch worker totals", map[string]interface{}{
			"accountID": accountID,
			"error":     err.Error(),
		})
		return nil, err
	}

	// Log the successful response
	logging.Info("Successfully fetched worker totals", map[string]interface{}{
		"worker_count": len(resp.Viewer.Accounts),
		"accountID":    accountID,
	})

	return &resp, nil
}

// FetchLogpushAccount queries logpushHealthAdaptiveGroups and returns CloudflareResponseLogpushAccount.
func FetchLogpushAccount(accountID string) (*models.CloudflareResponseLogpushAccount, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`query($accountID: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
			viewer {
			accounts(filter: {accountTag : $accountID }) {
				logpushHealthAdaptiveGroups(
				filter: {
					datetime_geq: $mintime
					datetime_lt: $maxtime
					status_neq: 200
				}
				limit: $limit
				) {
				count
				dimensions {
					jobId
					status
					destinationType
					datetime
					final
				}
				}
			}
			}
		}`)

	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}

	request.Var("accountID", accountID)
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)

	// Log the query parameters for debugging
	logging.Info("Fetching logpush health data for Cloudflare account", map[string]interface{}{
		"accountID":         accountID,
		"limit":             viper.GetInt("cf_query_limit"),
		"maxtime":           now,
		"mintime":           now1mAgo,
		"cfGraphQLEndpoint": cfGraphQLEndpoint,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseLogpushAccount
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to fetch logpush health data", map[string]interface{}{
			"accountID": accountID,
			"error":     err.Error(),
		})
		return nil, err
	}

	// Ensure the response is not nil
	if resp.Viewer.Accounts == nil {
		logging.Error("Received nil Accounts from Cloudflare API", map[string]interface{}{
			"accountID": accountID,
		})
		return nil, fmt.Errorf("Cloudflare API returned nil accounts")
	}

	// Ensure accounts slice is not empty before accessing index 0
	if len(resp.Viewer.Accounts) == 0 {
		logging.Error("Received empty accounts list from Cloudflare API", map[string]interface{}{
			"accountID": accountID,
		})
		return nil, fmt.Errorf("Cloudflare API returned empty accounts list")
	}

	// Ensure LogpushHealthAdaptiveGroups is not nil before accessing it
	if resp.Viewer.Accounts[0].LogpushHealthAdaptiveGroups == nil {
		logging.Error("Received nil LogpushHealthAdaptiveGroups from Cloudflare API", map[string]interface{}{
			"accountID": accountID,
		})
		return nil, fmt.Errorf("Cloudflare API returned nil LogpushHealthAdaptiveGroups")
	}

	// Log the successful response
	logging.Info("Successfully fetched logpush health data", map[string]interface{}{
		"logpush_count": len(resp.Viewer.Accounts[0].LogpushHealthAdaptiveGroups),
		"accountID":     accountID,
	})

	return &resp, nil
}

// ExtractZoneIDs extracts zone Ids from zones and return array of zone ids.
func ExtractZoneIDs(zones []cloudflare.Zone) []string {
	var IDs []string
	for _, z := range zones {
		IDs = append(IDs, z.ID)
	}
	return IDs
}

// contains helper function
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// FilterExcludedZones excludes zones and return array of non excludes zones
func FilterExcludedZones(all []cloudflare.Zone, exclude []string) []cloudflare.Zone {
	var filtered []cloudflare.Zone

	if (len(exclude)) == 0 {
		logging.Info("No zones to exclude. Returning all zones.", nil)
		return all
	}

	for _, z := range all {
		if contains(exclude, z.ID) {
			// Log zones that are excluded
			logging.Info("Excluding zone", map[string]interface{}{
				"zoneID":   z.ID,
				"zoneName": z.Name,
			})
		} else {
			filtered = append(filtered, z)
		}
	}

	// Log the number of zones returned after filtering
	logging.Info("Filtered zones count", map[string]interface{}{
		"totalExcluded": len(all) - len(filtered),
		"totalReturned": len(filtered),
	})

	return filtered
}

// FetchFirewallRules queries firewall rules.
func FetchFirewallRules(zoneID string) map[string]string {

	var api *cloudflare.API
	var err error
	if len(viper.GetString("cf_api_token")) > 0 {
		api, err = cloudflare.NewWithAPIToken(viper.GetString("cf_api_token"))
	} else {
		api, err = cloudflare.New(viper.GetString("cf_api_key"), viper.GetString("cf_api_email"))
	}
	if err != nil {
		logging.Error("Failed to initialize Cloudflare API client", map[string]interface{}{"error": err.Error()})
		return map[string]string{}
	}

	// Log the start of the firewall rules fetch
	logging.Info("Fetching firewall rules for zone", map[string]interface{}{
		"zoneID": zoneID,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	listOfRules, _, err := api.FirewallRules(ctx,
		cloudflare.ZoneIdentifier(zoneID),
		cloudflare.FirewallRuleListParams{})
	if err != nil {
		logging.Error(err)
	}
	firewallRulesMap := make(map[string]string)

	for _, rule := range listOfRules {
		firewallRulesMap[rule.ID] = rule.Description
	}

	listOfRulesets, err := api.ListRulesets(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListRulesetsParams{})
	if err != nil {
		logging.Error(err)
	}

	logging.Info("Fetched rulesets", map[string]interface{}{
		"zoneID":          zoneID,
		"rulesetsFetched": len(listOfRulesets),
	})

	for _, rulesetDesc := range listOfRulesets {
		if rulesetDesc.Phase == "http_request_firewall_managed" {
			ruleset, err := api.GetRuleset(ctx, cloudflare.ZoneIdentifier(zoneID), rulesetDesc.ID)
			if err != nil {
				logging.Info("Fetched managed ruleset", map[string]interface{}{
					"zoneID":       zoneID,
					"rulesetID":    rulesetDesc.ID,
					"rulesetName":  rulesetDesc.Name,
					"rulesFetched": len(ruleset.Rules),
				})
			}
			for _, rule := range ruleset.Rules {
				firewallRulesMap[rule.ID] = rule.Description
			}
		}
	}

	// Log the number of rules collected
	logging.Info("Total firewall rules collected", map[string]interface{}{
		"zoneID":     zoneID,
		"totalRules": len(firewallRulesMap),
	})

	return firewallRulesMap
}

// FetchColoTotals returns queries httpRequestsAdaptiveGroups.
func FetchColoTotals(zoneIDs []string) (*models.CloudflareResponseColo, error) {

	// Log the start of the process
	logging.Info("Fetching Colo totals for zoneIDs", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
	query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
		viewer {
			zones(filter: { zoneTag_in: $zoneIDs }) {
				zoneTag
				httpRequestsAdaptiveGroups(
					limit: $limit
					filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
					) {
						count
						avg {
							sampleInterval
						}
						dimensions {
							clientRequestHTTPHost
							coloCode
							datetime
							originResponseStatus
						}
						sum {
							edgeResponseBytes
							visits
						}
					}
				}
			}
		}
`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	// Log request variables
	logging.Info("GraphQL request variables", map[string]interface{}{
		"limit":   viper.GetInt("cf_query_limit"),
		"maxtime": now,
		"mintime": now1mAgo,
		"zoneIDs": zoneIDs,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseColo
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		// Log the error if request fails
		logging.Error("Failed to fetch Colo totals", map[string]interface{}{
			"error": err,
		})
		return nil, err
	}

	// Log success after receiving response
	logging.Info("Successfully fetched Colo totals", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	return &resp, nil
}

// FetchLoadBalancerTotals returns data by querying loadBalancingRequestsAdaptiveGroups and loadBalancingRequestsAdaptive.
func FetchLoadBalancerTotals(zoneIDs []string) (*models.CloudflareResponseLb, error) {
	// Log the start of the process
	logging.Info("Fetching Load Balancer totals for zoneIDs", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`
	query ($zoneIDs: [String!], $mintime: Time!, $maxtime: Time!, $limit: Int!) {
		viewer {
			zones(filter: { zoneTag_in: $zoneIDs }) {
				zoneTag
				loadBalancingRequestsAdaptiveGroups(
					filter: { datetime_geq: $mintime, datetime_lt: $maxtime},
					limit: $limit) {
					count
					dimensions {
						region
						lbName
						selectedPoolName
						proxied
						selectedOriginName
						selectedPoolAvgRttMs
						selectedPoolHealthy
						steeringPolicy
					}
				}
				loadBalancingRequestsAdaptive(
					filter: { datetime_geq: $mintime, datetime_lt: $maxtime},
					limit: $limit) {
					lbName
					proxied
					region
					selectedPoolHealthy
					selectedPoolId
					selectedPoolName
					sessionAffinityStatus
					steeringPolicy
					selectedPoolAvgRttMs
					pools {
						id
						poolName
						healthy
						avgRttMs
					}
					origins {
						originName
						health
						ipv4
						selected
					}
				}
			}
		}
	}
`)
	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)
	request.Var("zoneIDs", zoneIDs)

	// Log request variables
	logging.Info("GraphQL request variables", map[string]interface{}{
		"limit":   viper.GetInt("cf_query_limit"),
		"maxtime": now,
		"mintime": now1mAgo,
		"zoneIDs": zoneIDs,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseLb
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		// Log the error if request fails
		logging.Error("Failed to fetch Load Balancer totals", map[string]interface{}{
			"error": err,
		})
		return nil, err
	}

	// Log success after receiving response
	logging.Info("Successfully fetched Load Balancer totals", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	return &resp, nil
}

// FetchLogpushZone query logpushHealthAdaptiveGroups and return CloudflareResponseLogpushZone
func FetchLogpushZone(zoneIDs []string) (*models.CloudflareResponseLogpushZone, error) {
	// Log the start of the process
	logging.Info("Fetching Logpush zone for zoneIDs", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`query($zoneIDs: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
		viewer {
			zones(filter: {zoneTag_in : $zoneIDs }) {
			logpushHealthAdaptiveGroups(
			  filter: {
				datetime_geq: $mintime
				datetime_lt: $maxtime
				status_neq: 200
			  }
			  limit: $limit
			) {
			  count
			  dimensions {
				jobId
				status
				destinationType
				datetime
				final
			  }
			}
		  }
		}
	  }`)

	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}

	request.Var("zoneIDs", zoneIDs)
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)

	// Log request variables
	logging.Info("FetchLogpushZone GraphQL request variables", map[string]interface{}{
		"zoneIDs": zoneIDs,
		"limit":   viper.GetInt("cf_query_limit"),
		"maxtime": now,
		"mintime": now1mAgo,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseLogpushZone
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error(err)
		return nil, err
	}

	// Log success after receiving response
	logging.Info("Successfully fetched Logpush zone data", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	return &resp, nil
}

// FetchFirewallEventsAllowedDenied queries logpushHealthAdaptiveGroups.
func FetchFirewallEventsAllowedDenied(zoneIDs []string) (*models.CloudflareResponseLogpushZone, error) {
	// Log the start of the process
	logging.Info("Fetching firewall events for allowed/denied status", map[string]interface{}{
		"zoneIDs": zoneIDs,
	})

	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	request := graphql.NewRequest(`query($zoneIDs: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
		viewer {
			zones(filter: {zoneTag_in : $zoneIDs }) {
			logpushHealthAdaptiveGroups(
			  filter: {
				datetime_geq: $mintime
				datetime_lt: $maxtime
				status_neq: 200
			  }
			  limit: $limit
			) {
			  count
			  dimensions {
				jobId
				status
				destinationType
				datetime
				final
			  }
			}
		  }
		}
	  }`)

	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}

	request.Var("zoneIDs", zoneIDs)
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)

	// Log request variables
	logging.Info("GraphQL request variables", map[string]interface{}{
		"zoneIDs": zoneIDs,
		"limit":   viper.GetInt("cf_query_limit"),
		"maxtime": now,
		"mintime": now1mAgo,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseLogpushZone
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		// Log the error if request fails
		logging.Error("Failed to fetch firewall events", map[string]interface{}{
			"error": err,
		})
		return nil, err
	}

	// Log success after receiving response
	logging.Info("Successfully fetched firewall events for allowed/denied status", map[string]interface{}{
		"zoneIDs":  zoneIDs,
		"response": resp,
	})

	return &resp, nil
}

// MagicTransitTunnelHealthChecksAdaptiveGroups query magicTransitTunnelHealthChecksAdaptiveGroups.
func MagicTransitTunnelHealthChecksAdaptiveGroups(accountID string) (*models.CloudflareResponseMagicTransit, error) {
	now := time.Now().Add(-time.Duration(viper.GetInt("scrape_delay")) * time.Second).UTC()
	s := 60 * time.Second
	now = now.Truncate(s)
	now1mAgo := now.Add(-60 * time.Second)

	// Log the computed time range
	logging.Info("Computed time range for Magic Transit query", map[string]interface{}{
		"now":         now,
		"now1mAgo":    now1mAgo,
		"scrapeDelay": viper.GetInt("scrape_delay"),
	})

	request := graphql.NewRequest(`query($accountID: String!, $limit: Int!, $mintime: Time!, $maxtime: Time!) {
		viewer {
			accounts(filter: {accountTag : $accountID }) {
				magicTransitTunnelHealthChecksAdaptiveGroups(
					limit: $limit
					filter: { datetime_geq: $mintime, datetime_lt: $maxtime }
				) {
					count
					dimensions {
						active
						datetime
						edgeColoCity
						edgeColoCountry
						edgePopName
						remoteTunnelIPv4
						resultStatus
						siteName
						tunnelName
					}
				}
			}
		}
	}`)

	if len(viper.GetString("cf_api_token")) > 0 {
		request.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		request.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		request.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}

	request.Var("accountID", accountID)
	request.Var("limit", viper.GetInt("cf_query_limit"))
	request.Var("maxtime", now)
	request.Var("mintime", now1mAgo)

	// Log the request headers and variables before sending the request
	logging.Info("GraphQL request details", map[string]interface{}{
		"accountID": accountID,
		"limit":     viper.GetInt("cf_query_limit"),
		"maxtime":   now,
		"mintime":   now1mAgo,
	})

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
	defer cancel()

	graphqlClient := graphql.NewClient(cfGraphQLEndpoint)
	var resp models.CloudflareResponseMagicTransit
	if err := graphqlClient.Run(ctx, request, &resp); err != nil {
		logging.Error("Failed to execute GraphQL query", map[string]interface{}{
			"error":     err.Error(),
			"accountID": accountID,
			"endpoint":  cfGraphQLEndpoint,
		})
		return nil, err
	}

	// Log successful response
	logging.Info("Successfully fetched Magic Transit data", map[string]interface{}{
		"accountID": accountID,
		"count":     len(resp.Viewer.Accounts),
	})

	return &resp, nil
}

// HTTP client with timeout
var httpClient = &http.Client{
	Timeout: 10 * time.Second, // Set a per-request timeout
}

// FetchSSLCertificateStatus fetches SSL certificate status for multiple zones concurrently
func FetchSSLCertificateStatus(zoneIDs []string) (*models.SSLResponse, error) {
	var combinedResponse models.SSLResponse
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Use a buffered channel to limit concurrency (avoid hitting rate limits)
	maxConcurrentRequests := 5
	sem := make(chan struct{}, maxConcurrentRequests)

	for _, zoneID := range zoneIDs {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot

		go func(zoneID string) {
			defer wg.Done()
			defer func() { <-sem }() // Release slot

			sslResponse, err := fetchSSLForZone(zoneID)
			if err != nil {
				logging.Error("Failed to fetch SSL data", map[string]interface{}{
					"zone_id": zoneID,
					"error":   err.Error(),
				})
				return
			}

			mu.Lock()
			combinedResponse.Result = append(combinedResponse.Result, sslResponse.Result...)
			mu.Unlock()
		}(zoneID)
	}

	// 🛠 **Fix: Wait for all goroutines to complete before returning**
	wg.Wait()

	return &combinedResponse, nil
}

// fetchSSLForZone fetches SSL certificate data for a single zone with retry logic
func fetchSSLForZone(zoneID string) (*models.SSLResponse, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/ssl/certificate_packs", zoneID)
	logging.Info("Fetching SSL certificate status", map[string]interface{}{
		"zone_id":  zoneID,
		"endpoint": url,
	})

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication headers
	if len(viper.GetString("cf_api_token")) > 0 {
		req.Header.Set("Authorization", "Bearer "+viper.GetString("cf_api_token"))
	} else {
		req.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
		req.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))
	}
	req.Header.Set("Content-Type", "application/json")

	// Implement retry with exponential backoff
	maxRetries := 3
	var body []byte

	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set 10s timeout
		defer cancel()

		req = req.WithContext(ctx)

		resp, err := httpClient.Do(req)
		if err != nil {
			logging.Warn("API request failed, retrying...", map[string]interface{}{
				"zone_id": zoneID,
				"attempt": attempt,
				"error":   err.Error(),
			})
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		defer resp.Body.Close()

		// Handle rate limit (429)
		if resp.StatusCode == 429 {
			logging.Warn("Rate limited, waiting before retry...", map[string]interface{}{
				"zone_id":  zoneID,
				"attempt":  attempt,
				"response": resp.Status,
			})
			time.Sleep(time.Duration(attempt*3) * time.Second)
			continue
		}

		// Read body
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch SSL data, status: %d, response: %s", resp.StatusCode, string(body))
		}

		logging.Info("API response received", map[string]interface{}{
			"zone_id":       zoneID,
			"status_code":   resp.StatusCode,
			"response_time": resp.Header.Get("Date"),
		})

		break // Success, exit retry loop
	}

	// Parse response
	var sslResponse models.SSLResponse
	if err := json.Unmarshal(body, &sslResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Assign ZoneID to results
	for i := range sslResponse.Result {
		sslResponse.Result[i].ZoneID = zoneID
	}

	logging.Info("SSL certificate data fetched successfully", map[string]interface{}{
		"zone_id":    zoneID,
		"cert_count": len(sslResponse.Result),
	})

	return &sslResponse, nil
}
