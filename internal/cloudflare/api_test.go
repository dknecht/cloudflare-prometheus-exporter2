package cloudflare_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"

	"github.com/lablabs/cloudflare-exporter/internal/cloudflare"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestAuthHeader_WithToken(t *testing.T) {
	// Setup: mock viper values
	viper.Set("cf_api_token", "dummy-token")
	viper.Set("cf_api_email", "")
	viper.Set("cf_api_key", "")

	// Create a dummy request
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Mimic auth logic from api.go
	if token := viper.GetString("cf_api_token"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-AUTH-EMAIL", viper.GetString("cf_api_email"))
	req.Header.Set("X-AUTH-KEY", viper.GetString("cf_api_key"))

	// Assertions
	assert.Equal(t, "Bearer dummy-token", req.Header.Get("Authorization"))
	assert.Equal(t, "", req.Header.Get("X-AUTH-EMAIL"))
	assert.Equal(t, "", req.Header.Get("X-AUTH-KEY"))
}

func TestFetchZones_Mocked(t *testing.T) {
	// Setup mock HTTP
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	viper.Set("cf_api_token", "dummy-token")

	// Mock the Cloudflare zones API
	httpmock.RegisterResponder("GET", "https://api.cloudflare.com/client/v4/zones",
		httpmock.NewStringResponder(200, `{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "zone1",
					"name": "example.com",
					"status": "active"
				}
			]
		}`))

	zones, err := cloudflare.FetchZones(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, len(zones))
	assert.Equal(t, "example.com", zones[0].Name)
}

func TestFetchAccounts_WithMockedHTTP(t *testing.T) {
	// Mock env vars
	viper.Set("cf_api_token", "dummy-token")

	// Activate HTTP mock
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// This is the actual Cloudflare API mock
	httpmock.RegisterResponder("GET", "https://api.cloudflare.com/client/v4/accounts",
		httpmock.NewStringResponder(200, `{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "acc1",
					"name": "Test Account"
				}
			]
		}`))

	ctx := context.Background()
	accounts, err := cloudflare.FetchAccounts(ctx)

	assert.NoError(t, err)
	assert.Len(t, accounts, 1)
	assert.Equal(t, "Test Account", accounts[0].Name)
}
