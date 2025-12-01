package routes

import (
	"context"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/gin-gonic/gin"
	"github.com/lablabs/cloudflare-exporter/internal/handlers"
	"github.com/lablabs/cloudflare-exporter/internal/metrics"
	"github.com/lablabs/cloudflare-exporter/internal/middlewares"
	logging "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// RunExporter starts the metric exporter and serves metrics on the /metrics endpoint
func RunExporter() {

	// Log the beginning of the exporter setup
	logging.Info("Starting metric exporter setup version : 1.11")

	cfgMetricsPath := viper.GetString("metrics_path")

	if !(len(viper.GetString("cf_api_token")) > 0 || (len(viper.GetString("cf_api_email")) > 0 && len(viper.GetString("cf_api_key")) > 0)) {
		logging.Fatal("Please provide CF_API_KEY+CF_API_EMAIL or CF_API_TOKEN")
	}
	if viper.GetInt("cf_batch_size") < 1 || viper.GetInt("cf_batch_size") > 10 {
		logging.Fatal("CF_BATCH_SIZE must be between 1 and 10")
	}
	customFormatter := new(logging.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	logging.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	metricsDenylist := []string{}
	if len(viper.GetString("metrics_denylist")) > 0 {
		metricsDenylist = strings.Split(viper.GetString("metrics_denylist"), ",")
	}
	deniedMetricsSet, err := metrics.BuildDeniedMetricsSet(metricsDenylist)
	if err != nil {
		logging.Fatal("Error building denied metrics set", map[string]interface{}{"error": err.Error()})
	}
	metrics.MustRegisterMetrics(deniedMetricsSet)
	logging.Info("Metrics registered successfully", map[string]interface{}{"metricsDenylist": metricsDenylist})

	// Initialize Gin
	r := gin.Default()

	r.Use(middlewares.CORS())      // For handling CORS requests
	r.Use(handlers.ErrorHandler()) // for hanfling error

	// Define /metrics route
	r.GET(cfgMetricsPath, metrics.Handler)

	logging.Info("Metrics endpoint registered at ", cfgMetricsPath)

	// Use the HealthCheck function for the health endpoint
	r.GET("/health", handlers.HealthCheck)
	logging.Info("Health check endpoint registered at /health")

	// Start the improved periodic metric fetcher
	go startMetricsExporter()

	// Start the Gin server
	logging.Info("Beginning to serve metrics on ", viper.GetString("listen"))
	if err := r.Run(viper.GetString("listen")); err != nil {
		logging.Fatal("Error starting server: ", map[string]interface{}{"error": err.Error()})
	}
}

func startMetricsExporter() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Worker pool reused across scrapes
	pool := workerpool.New(20)
	defer pool.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go func() {
				// Wrap existing FetchMetrics with context
				err := metrics.FetchMetrics(ctx, pool)
				if err != nil {
					logging.Error("Fetch failed", err)
				}
			}()
		}
	}
}
