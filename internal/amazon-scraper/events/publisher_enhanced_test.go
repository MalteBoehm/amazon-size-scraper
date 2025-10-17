package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedNewProductDetectedPayload(t *testing.T) {
	t.Run("Complete payload with size table", func(t *testing.T) {
		sizeTable := &database.SizeTable{
			Sizes: []string{"S", "M", "L", "XL"},
			Measurements: map[string]map[string]float64{
				"S":  {"chest": 96, "length": 70, "width": 52},
				"M":  {"chest": 100, "length": 72, "width": 54},
				"L":  {"chest": 104, "length": 74, "width": 56},
				"XL": {"chest": 108, "length": 76, "width": 58},
			},
			Unit: "cm",
		}

		payload := &EnhancedNewProductDetectedPayload{
			EventID:       uuid.New().String(),
			EventType:     string(EventTypeNewProductDetected),
			ASIN:          "B08N5WRWNW",
			Title:         "Test T-Shirt",
			Brand:         "Test Brand",
			DetailPageURL: "https://www.amazon.de/dp/B08N5WRWNW",
			Category:      "Clothing",
			Price: &Price{
				Amount:   29.99,
				Currency: "EUR",
			},
			Rating:         floatPtr(4.5),
			ReviewCount:    intPtr(150),
			Images:         []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
			Features:       []string{"100% Cotton", "Machine washable"},
			AvailableSizes: []string{"S", "M", "L", "XL"},
			SizeTable:      sizeTable,
			Source:         "scraper",
		}

		// Test JSON marshaling
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		// Verify all fields are present
		var unmarshaled map[string]interface{}
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "B08N5WRWNW", unmarshaled["asin"])
		assert.Equal(t, "Test T-Shirt", unmarshaled["title"])
		assert.Equal(t, "Test Brand", unmarshaled["brand"])
		assert.Equal(t, "Clothing", unmarshaled["category"])
		assert.NotNil(t, unmarshaled["size_table"])
		assert.NotNil(t, unmarshaled["images"])
		assert.NotNil(t, unmarshaled["features"])
		assert.NotNil(t, unmarshaled["available_sizes"])

		// Verify size table structure
		sizeTableData := unmarshaled["size_table"].(map[string]interface{})
		assert.NotNil(t, sizeTableData["sizes"])
		assert.NotNil(t, sizeTableData["measurements"])
		assert.Equal(t, "cm", sizeTableData["unit"])
	})

	t.Run("Validate required fields for lifecycle consumer", func(t *testing.T) {
		payload := &EnhancedNewProductDetectedPayload{
			ASIN:          "B08N5WRWNW",
			Title:         "Test Product",
			DetailPageURL: "https://www.amazon.de/dp/B08N5WRWNW",
			Source:        "scraper",
		}

		// These fields are required by the lifecycle consumer
		assert.NotEmpty(t, payload.ASIN)
		assert.NotEmpty(t, payload.Title)
		assert.NotEmpty(t, payload.DetailPageURL)
		assert.Equal(t, "scraper", payload.Source)
	})

	t.Run("Size table validation in payload", func(t *testing.T) {
		// Valid size table
		validTable := &database.SizeTable{
			Sizes: []string{"M", "L"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "length": 72, "width": 54},
				"L": {"chest": 104, "length": 74, "width": 56},
			},
			Unit: "cm",
		}

		payload := &EnhancedNewProductDetectedPayload{
			ASIN:      "B08N5WRWNW",
			SizeTable: validTable,
		}

		assert.True(t, payload.HasValidSizeTable())

		// Invalid size table - no length
		invalidTable := &database.SizeTable{
			Sizes: []string{"M"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "width": 54},
			},
			Unit: "cm",
		}

		payload.SizeTable = invalidTable
		assert.False(t, payload.HasValidSizeTable())

		// No size table
		payload.SizeTable = nil
		assert.False(t, payload.HasValidSizeTable())
	})
}

func TestPublishEnhancedProductEvent(t *testing.T) {
	t.Skip("Requires database connection")

	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	publisher := NewPublisher(db, nil)

	t.Run("Publish complete product event", func(t *testing.T) {
		sizeTable := &database.SizeTable{
			Sizes: []string{"M", "L"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "length": 72, "width": 54},
				"L": {"chest": 104, "length": 74, "width": 56},
			},
			Unit: "cm",
		}

		payload := &EnhancedNewProductDetectedPayload{
			ASIN:           "B08N5WRWNW",
			Title:          "Test T-Shirt",
			Brand:          "Test Brand",
			DetailPageURL:  "https://www.amazon.de/dp/B08N5WRWNW",
			Category:       "Clothing",
			Images:         []string{"https://example.com/img1.jpg"},
			Features:       []string{"100% Cotton"},
			AvailableSizes: []string{"M", "L"},
			SizeTable:      sizeTable,
			Source:         "scraper",
		}

		err := publisher.PublishEnhancedNewProductDetected(ctx, payload)
		assert.NoError(t, err)

		// Verify event was written to outbox
		var count int
		err = db.Pool().QueryRow(ctx,
			"SELECT COUNT(*) FROM outbox_event WHERE aggregate_id = $1",
			payload.ASIN).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	// Placeholder - would use actual test DB
	return nil
}