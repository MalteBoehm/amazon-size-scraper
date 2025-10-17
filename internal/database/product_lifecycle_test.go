package database

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductLifecycleMethods(t *testing.T) {
	// Skip tests if no database is available
	t.Skip("Test database not configured")
	
	t.Run("InsertProductLifecycle", func(t *testing.T) {
		ctx := context.Background()
		// db := setupTestDB(t)
		// defer db.Close()
		var db *DB

		sizeTable := &SizeTable{
			Sizes: []string{"S", "M", "L", "XL"},
			Measurements: map[string]map[string]float64{
				"S": {"chest": 96, "length": 70, "width": 52},
				"M": {"chest": 100, "length": 72, "width": 54},
				"L": {"chest": 104, "length": 74, "width": 56},
				"XL": {"chest": 108, "length": 76, "width": 58},
			},
			Unit: "cm",
		}

		sizeTableJSON, err := json.Marshal(sizeTable)
		require.NoError(t, err)

		product := &ProductLifecycle{
			ASIN:          "B08N5WRWNW",
			Title:         "Echo Show 8",
			Brand:         "Amazon",
			DetailPageURL: "https://www.amazon.de/dp/B08N5WRWNW",
			ImageURLs:     json.RawMessage(`["https://example.com/image1.jpg"]`),
			Features:      json.RawMessage(`["Feature 1", "Feature 2"]`),
			CurrentPrice:  floatPtr(99.99),
			Currency:      "EUR",
			Rating:        floatPtr(4.5),
			ReviewCount:   intPtr(100),
			Status:        "PENDING",
			Category:      "Electronics",
			AvailableSizes: json.RawMessage(`["S", "M", "L", "XL"]`),
			SizeTable:     sizeTableJSON,
		}

		err = db.InsertProductLifecycle(ctx, product)
		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, product.ID)
		assert.NotZero(t, product.CreatedAt)
		assert.NotZero(t, product.UpdatedAt)
	})

	t.Run("GetProductLifecycleByASIN", func(t *testing.T) {
		ctx := context.Background()
		db := setupTestDB(t)
		defer db.Close()

		// Insert test product
		product := &ProductLifecycle{
			ASIN:          "B08N5WRWNW",
			Title:         "Test Product",
			Brand:         "Test Brand",
			DetailPageURL: "https://www.amazon.de/dp/B08N5WRWNW",
			Status:        "PENDING",
		}
		
		err := db.InsertProductLifecycle(ctx, product)
		require.NoError(t, err)

		// Retrieve product
		retrieved, err := db.GetProductLifecycleByASIN(ctx, "B08N5WRWNW")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, product.ASIN, retrieved.ASIN)
		assert.Equal(t, product.Title, retrieved.Title)
		assert.Equal(t, product.Brand, retrieved.Brand)
	})

	t.Run("UpdateProductLifecycleSizeTable", func(t *testing.T) {
		ctx := context.Background()
		db := setupTestDB(t)
		defer db.Close()

		// Insert test product
		product := &ProductLifecycle{
			ASIN:          "B08N5WRWNW",
			Title:         "Test Product",
			Brand:         "Test Brand",
			DetailPageURL: "https://www.amazon.de/dp/B08N5WRWNW",
			Status:        "PENDING",
		}
		
		err := db.InsertProductLifecycle(ctx, product)
		require.NoError(t, err)

		// Update with size table
		sizeTable := &SizeTable{
			Sizes: []string{"M", "L"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "length": 72, "width": 54},
				"L": {"chest": 104, "length": 74, "width": 56},
			},
			Unit: "cm",
		}

		err = db.UpdateProductLifecycleSizeTable(ctx, product.ASIN, sizeTable)
		assert.NoError(t, err)

		// Verify update
		updated, err := db.GetProductLifecycleByASIN(ctx, product.ASIN)
		require.NoError(t, err)
		
		var retrievedSizeTable SizeTable
		err = json.Unmarshal(updated.SizeTable, &retrievedSizeTable)
		require.NoError(t, err)
		
		assert.Equal(t, sizeTable.Sizes, retrievedSizeTable.Sizes)
		assert.Equal(t, sizeTable.Measurements, retrievedSizeTable.Measurements)
		assert.Equal(t, "SCRAPED", updated.Status)
	})

	t.Run("ValidateSizeTableHasLengthAndWidth", func(t *testing.T) {
		// Test valid size table with length and width
		validTable := &SizeTable{
			Sizes: []string{"M", "L"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "length": 72, "width": 54},
				"L": {"chest": 104, "length": 74, "width": 56},
			},
			Unit: "cm",
		}
		assert.True(t, ValidateSizeTable(validTable))

		// Test invalid - missing length
		noLength := &SizeTable{
			Sizes: []string{"M"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "width": 54},
			},
			Unit: "cm",
		}
		assert.False(t, ValidateSizeTable(noLength))

		// Test invalid - missing width
		noWidth := &SizeTable{
			Sizes: []string{"M"},
			Measurements: map[string]map[string]float64{
				"M": {"chest": 100, "length": 72},
			},
			Unit: "cm",
		}
		assert.False(t, ValidateSizeTable(noWidth))

		// Test invalid - empty table
		emptyTable := &SizeTable{
			Sizes:        []string{},
			Measurements: make(map[string]map[string]float64),
			Unit:         "cm",
		}
		assert.False(t, ValidateSizeTable(emptyTable))

		// Test invalid - nil table
		assert.False(t, ValidateSizeTable(nil))
	})
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}