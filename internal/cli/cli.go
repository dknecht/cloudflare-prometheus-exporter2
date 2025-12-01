package cli

import (
	"github.com/lablabs/cloudflare-exporter/internal/routes"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute initializes and runs the Cobra CLI
func Execute() error {

	var cmd = &cobra.Command{
		Use:   "viper-test",
		Short: "testing viper",
		Run: func(_ *cobra.Command, _ []string) {
			routes.RunExporter()
		},
	}

	viper.AutomaticEnv()

	flags := cmd.Flags()

	flags.String("listen", ":8080", "listen on addr:port ( default :8080), omit addr to listen on all interfaces")
	viper.BindEnv("listen")
	viper.SetDefault("listen", ":8080")

	flags.String("metrics_path", "/metrics", "path for metrics, default /metrics")
	viper.BindEnv("metrics_path")
	viper.SetDefault("metrics_path", "/metrics")

	flags.String("cf_api_key", "", "cloudflare api key, works with api_email flag")
	viper.BindEnv("cf_api_key")

	flags.String("cf_api_email", "", "cloudflare api email, works with api_key flag")
	viper.BindEnv("cf_api_email")

	flags.String("cf_api_token", "", "cloudflare api token (preferred)")
	viper.BindEnv("cf_api_token")

	flags.String("cf_zones", "", "cloudflare zones to export, comma delimited list")
	viper.BindEnv("cf_zones")
	viper.SetDefault("cf_zones", "")

	flags.String("cf_exclude_zones", "", "cloudflare zones to exclude, comma delimited list")
	viper.BindEnv("cf_exclude_zones")
	viper.SetDefault("cf_exclude_zones", "")

	flags.Int("scrape_delay", 300, "scrape delay in seconds, defaults to 300")
	viper.BindEnv("scrape_delay")
	viper.SetDefault("scrape_delay", 300)

	flags.Int("cf_batch_size", 10, "cloudflare zones batch size (1-10), defaults to 10")
	viper.BindEnv("cf_batch_size")
	viper.SetDefault("cf_batch_size", 10)

	flags.Bool("free_tier", false, "scrape only metrics included in free plan")
	viper.BindEnv("free_tier")
	viper.SetDefault("free_tier", false)

	flags.String("metrics_denylist", "", "metrics to not expose, comma delimited list")
	viper.BindEnv("metrics_denylist")
	viper.SetDefault("metrics_denylist", "")

	flags.Bool("exclude_host", true, "metrics data without host when exclude")
	viper.BindEnv("exclude_host")
	viper.SetDefault("exclude_host", true)

	flags.Int("cf_query_limit", 1000, "query limit for cloudflare API")
	viper.BindEnv("cf_query_limit")
	viper.SetDefault("cf_query_limit", 1000)

	flags.Bool("cf_http_status_group", false, "query limit for cloudflare API")
	viper.BindEnv("cf_http_status_group")
	viper.SetDefault("cf_http_status_group", false)

	viper.BindPFlags(flags)
	return cmd.Execute()
}
