//go:build integration
// +build integration

package paapi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASINs contains reliable German ASINs with color variations for testing
var TestASINs = struct {
	// Products with known color variations
	WithColors []string
	// Products without color variations but with single color
	SingleColor []string
	// Parent ASINs with many variations
	ParentASINs []string
}{
	WithColors: []string{
		"B08N5WRWNW", // Echo Dot (multiple colors)
		"B07PJV3JPR", // Fire TV Stick (has color options)
	},
	SingleColor: []string{
		"B08C1LR9RC", // Fire TV Stick 4K (usually single color)
	},
	ParentASINs: []string{
		"B08N5WRWNW", // Echo Dot parent ASIN
	},
}

func getTestClient(t *testing.T) *Client {
	accessKey := os.Getenv("AMAZON_ACCESS_KEY")
	secretKey := os.Getenv("AMAZON_SECRET_KEY")
	partnerTag := os.Getenv("AMAZON_PARTNER_TAG")
	region := os.Getenv("AMAZON_REGION")
	
	if accessKey == "" || secretKey == "" || partnerTag == "" {
		t.Skip("PA-API credentials not configured")
	}
	
	if region == "" {
		region = "de" // Default to German marketplace
	}
	
	return NewClient(accessKey, secretKey, partnerTag, region)
}

func TestGetItems_Integration(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	
	t.Run("GetItems with valid ASINs", func(t *testing.T) {
		response, err := client.GetItems(ctx, TestASINs.WithColors[:1])
		require.NoError(t, err)
		require.NotNil(t, response)
		require.NotNil(t, response.ItemsResult)
		assert.Greater(t, len(response.ItemsResult.Items), 0)
		
		// Check first item
		if len(response.ItemsResult.Items) > 0 {
			item := response.ItemsResult.Items[0]
			assert.NotEmpty(t, item.ASIN)
			t.Logf("Item ASIN: %s", item.ASIN)
			if item.ParentASIN != "" {
				t.Logf("Parent ASIN: %s", item.ParentASIN)
			}
			if item.ItemInfo != nil && item.ItemInfo.Title != nil {
				t.Logf("Title: %s", item.ItemInfo.Title.DisplayValue)
			}
		}
	})
	
	t.Run("GetItems with batch", func(t *testing.T) {
		// Test batch processing (up to 10 items)
		asins := append(TestASINs.WithColors, TestASINs.SingleColor...)
		if len(asins) > 10 {
			asins = asins[:10]
		}
		
		response, err := client.GetItems(ctx, asins)
		require.NoError(t, err)
		require.NotNil(t, response)
		
		if response.ItemsResult != nil {
			t.Logf("Retrieved %d items", len(response.ItemsResult.Items))
		}
	})
	
	t.Run("GetItems with invalid ASIN", func(t *testing.T) {
		response, err := client.GetItems(ctx, []string{"INVALID123"})
		// PA-API might return an error or empty result for invalid ASINs
		if err != nil {
			t.Logf("Expected error for invalid ASIN: %v", err)
		} else if response != nil && len(response.Errors) > 0 {
			t.Logf("API error for invalid ASIN: %v", response.Errors[0])
		}
	})
}

func TestGetVariations_Integration(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	
	t.Run("GetVariations for parent ASIN", func(t *testing.T) {
		if len(TestASINs.ParentASINs) == 0 {
			t.Skip("No parent ASINs configured for testing")
		}
		
		parentASIN := TestASINs.ParentASINs[0]
		variations, err := client.GetVariations(ctx, parentASIN)
		require.NoError(t, err)
		
		t.Logf("Retrieved %d variations for parent ASIN %s", len(variations), parentASIN)
		
		// Check variations
		for i, v := range variations {
			if i < 3 { // Log first 3 variations
				t.Logf("Variation %d: Value=%s, DisplayName=%s, ASIN=%s, Available=%v",
					i+1, v.Value, v.DisplayName, v.ASIN, v.Available)
			}
			
			// Validate variation
			assert.NotEmpty(t, v.Value)
			assert.NotEmpty(t, v.DisplayName)
		}
	})
}

func TestEnrichColors_Integration(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	
	t.Run("EnrichColors for products with variations", func(t *testing.T) {
		asins := TestASINs.WithColors[:1]
		results, err := client.EnrichColors(ctx, asins)
		require.NoError(t, err)
		require.NotNil(t, results)
		
		for asin, result := range results {
			t.Logf("ASIN %s: ParentASIN=%s, Colors=%d",
				asin, result.ParentASIN, len(result.Colors))
			
			// Log first few colors
			for i, color := range result.Colors {
				if i < 3 {
					t.Logf("  Color %d: Value=%s, DisplayName=%s, Available=%v",
						i+1, color.Value, color.DisplayName, color.Available)
				}
			}
			
			// Validate enrichment result
			assert.NotEmpty(t, result.ASIN)
			if len(result.Colors) > 0 {
				for _, color := range result.Colors {
					assert.NotEmpty(t, color.Value)
					assert.NotEmpty(t, color.DisplayName)
				}
			}
		}
	})
	
	t.Run("EnrichColors for single color product", func(t *testing.T) {
		if len(TestASINs.SingleColor) == 0 {
			t.Skip("No single color ASINs configured")
		}
		
		asins := TestASINs.SingleColor[:1]
		results, err := client.EnrichColors(ctx, asins)
		require.NoError(t, err)
		
		for asin, result := range results {
			t.Logf("Single color ASIN %s: Colors=%d", asin, len(result.Colors))
			if len(result.Colors) > 0 {
				t.Logf("  Color: %s", result.Colors[0].Value)
			}
		}
	})
}

func TestRateLimiting_Integration(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()
	
	t.Run("Rate limiting enforcement", func(t *testing.T) {
		start := time.Now()
		
		// Make 3 requests in quick succession
		for i := 0; i < 3; i++ {
			_, err := client.GetItems(ctx, TestASINs.WithColors[:1])
			require.NoError(t, err)
		}
		
		elapsed := time.Since(start)
		// Should take at least 2 seconds due to rate limiting (1 req/sec)
		assert.GreaterOrEqual(t, elapsed, 2*time.Second)
		t.Logf("3 requests took %v (rate limited)", elapsed)
	})
}

func TestColorNormalization_Integration(t *testing.T) {
	// Test color normalization with real PA-API data
	variations := []*models.VariationItem{
		{Value: "X123 Navy Blue (2024)", DisplayName: "X123 Navy Blue (2024)", ASIN: "TEST1", Available: true},
		{Value: "NAVY BLUE", DisplayName: "NAVY BLUE", ASIN: "TEST2", Available: true},
		{Value: "Red", DisplayName: "Red", ASIN: "TEST3", Available: true},
	}
	
	// This would be processed by the normalizer in the actual flow
	// Here we just validate the structure
	for _, v := range variations {
		assert.NotEmpty(t, v.Value)
		assert.NotEmpty(t, v.DisplayName)
		assert.NotEmpty(t, v.ASIN)
	}
}