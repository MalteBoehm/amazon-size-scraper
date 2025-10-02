// pkg/constants/constants.go
package constants

// Stream names (Unterstriche gemäß event.md)
const (
	StreamContentGeneration = "stream:content_generation"
	StreamProductLifecycle  = "stream:product_lifecycle"
	// StreamBrowseNodes removed - use StreamProductLifecycle for enrichment
	StreamPriceTracking     = "stream:price_tracking"
)

// Consumer group names (Unterstriche gemäß event.md)
const (
	GroupProductLifecycle  = "group:product_lifecycle"
	GroupContentGeneration = "group:content_generation"
	// GroupBrowseNodes removed
	GroupPriceTracking     = "group:price_tracking"
)

// Worker names (Unterstriche gemäß event.md)
const (
	WorkerProductLifecycle  = "product_lifecycle_worker"
	WorkerContentGeneration = "content_generation_worker"
	// WorkerBrowseNodes removed
	WorkerPriceTracking     = "price_tracking_worker"
)

// Database table names
const (
	TableProduct      = "product"
	TableOutbox       = "outbox"
	TablePriceHistory = "price_history"
	// TableBrowseNode removed
)
