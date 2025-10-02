package paapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/metrics"
	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/maltedev/amazon-size-scraper/internal/util/colors"
	"log/slog"
)

// Client represents the PA-API v5 client
type Client struct {
	accessKey    string
	secretKey    string
	partnerTag   string
	host         string
	region       string
	logger       *slog.Logger
	rateLimiter  *RateLimiter
	retryConfig  RetryConfig
	mu           sync.Mutex
}

// RetryConfig defines retry behavior for API calls
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens    int
	maxTokens int
	refillRate time.Duration
	lastRefill time.Time
	mu        sync.Mutex
}

// NewClient creates a new PA-API client
// Expected environment variables:
// - AMAZON_ACCESS_KEY: Your PA-API access key
// - AMAZON_SECRET_KEY: Your PA-API secret key
// - AMAZON_PARTNER_TAG: Your Amazon partner/associate tag
// - AMAZON_REGION: Region code (de, us, uk, fr, etc.)
func NewClient(accessKey, secretKey, partnerTag, region string) *Client {
	host := fmt.Sprintf("webservices.amazon.%s", getRegionDomain(region))
	
	return &Client{
		accessKey:   accessKey,
		secretKey:   secretKey,
		partnerTag:  partnerTag,
		host:        host,
		region:      region,
		logger:      slog.Default().With("component", "paapi_client"),
		rateLimiter: NewRateLimiter(1, time.Second), // 1 request per second
		retryConfig: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}
}

// signRequest signs a request using AWS Signature Version 4
func (c *Client) signRequest(method, path string, headers map[string]string, payload []byte) map[string]string {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	dateTime := now.Format("20060102T150405Z")
	
	// Add required headers
	headers["host"] = c.host
	headers["x-amz-date"] = dateTime
	// Fix x-amz-target with proper casing
	operation := ""
	if strings.Contains(path, "getitems") {
		operation = "GetItems"
	} else if strings.Contains(path, "getvariations") {
		operation = "GetVariations"
	}
	headers["x-amz-target"] = "com.amazon.paapi5.v1.ProductAdvertisingAPIv1." + operation
	headers["content-encoding"] = "amz-1.0"
	headers["content-type"] = "application/json; charset=utf-8"
	
	// Create canonical headers
	var headerKeys []string
	for k := range headers {
		headerKeys = append(headerKeys, strings.ToLower(k))
	}
	sort.Strings(headerKeys)
	
	var canonicalHeaders strings.Builder
	var signedHeaders []string
	for _, k := range headerKeys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(strings.TrimSpace(headers[k]))
		canonicalHeaders.WriteString("\n")
		signedHeaders = append(signedHeaders, k)
	}
	
	// Create payload hash
	payloadHash := sha256.Sum256(payload)
	payloadHashHex := hex.EncodeToString(payloadHash[:])
	
	// Create canonical request
	canonicalRequest := strings.Join([]string{
		method,
		path,
		"", // Query string (empty for PA-API)
		canonicalHeaders.String(),
		strings.Join(signedHeaders, ";"),
		payloadHashHex,
	}, "\n")
	
	// Create string to sign
	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))
	canonicalRequestHashHex := hex.EncodeToString(canonicalRequestHash[:])
	
	credentialScope := fmt.Sprintf("%s/%s/ProductAdvertisingAPI/aws4_request", dateStamp, c.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		dateTime,
		credentialScope,
		canonicalRequestHashHex,
	}, "\n")
	
	// Calculate signature
	signingKey := getSignatureKey(c.secretKey, dateStamp, c.region, "ProductAdvertisingAPI")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	
	// Create authorization header
	authorization := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey,
		credentialScope,
		strings.Join(signedHeaders, ";"),
		signature,
	)
	
	headers["Authorization"] = authorization
	return headers
}

// hmacSHA256 generates HMAC-SHA256 hash
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// getSignatureKey derives the signing key
func getSignatureKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxTokens int, refillRate time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)
	
	if tokensToAdd > 0 {
		rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
		rl.lastRefill = now
	}

	// If no tokens available, wait for next refill
	if rl.tokens <= 0 {
		waitTime := rl.refillRate - now.Sub(rl.lastRefill)
		rl.mu.Unlock()
		
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return ctx.Err()
		}
		
		rl.mu.Lock()
		rl.tokens = 1
		rl.lastRefill = time.Now()
	}

	rl.tokens--
	return nil
}

// GetItemsRequest represents a GetItems API request
type GetItemsRequest struct {
	ItemIds        []string
	Resources      []string
	PartnerTag     string
	PartnerType    string
	Marketplace    string
	LanguagesOfPreference []string
}

// GetItemsResponse represents the API response
type GetItemsResponse struct {
	ItemsResult *ItemsResult    `json:"ItemsResult,omitempty"`
	Errors      []ErrorResponse `json:"Errors,omitempty"`
}

// ItemsResult contains the items from the response
type ItemsResult struct {
	Items []Item `json:"Items,omitempty"`
}

// GetVariationsRequest represents a GetVariations API request
type GetVariationsRequest struct {
	ASIN                  string   `json:"ASIN"`
	Resources             []string `json:"Resources"`
	PartnerTag            string   `json:"PartnerTag"`
	PartnerType           string   `json:"PartnerType"`
	Marketplace           string   `json:"Marketplace"`
	VariationPage         int      `json:"VariationPage,omitempty"`
	VariationCount        int      `json:"VariationCount,omitempty"`
	LanguagesOfPreference []string `json:"LanguagesOfPreference,omitempty"`
}

// GetVariationsResponse represents the GetVariations API response
type GetVariationsResponse struct {
	VariationsResult *VariationsResult `json:"VariationsResult,omitempty"`
	Errors           []ErrorResponse   `json:"Errors,omitempty"`
}

// VariationsResult contains variation information
type VariationsResult struct {
	Items             []Item                  `json:"Items,omitempty"`
	VariationSummary  *VariationSummary       `json:"VariationSummary,omitempty"`
	VariationDimensions []VariationDimension  `json:"VariationDimensions,omitempty"`
}

// Item represents a product item from PA-API
type Item struct {
	ASIN             string                      `json:"ASIN"`
	ParentASIN       string                      `json:"ParentASIN,omitempty"`
	VariationAttributes map[string]interface{}   `json:"VariationAttributes,omitempty"`
	ItemInfo         *ItemInfo                   `json:"ItemInfo,omitempty"`
	Offers           *Offers                     `json:"Offers,omitempty"`
	VariationSummary *VariationSummary           `json:"VariationSummary,omitempty"`
}

// VariationSummary contains variation summary information
type VariationSummary struct {
	VariationDimensions []VariationDimension `json:"VariationDimension,omitempty"`
}

// VariationDimension represents a variation dimension like Color or Size
type VariationDimension struct {
	Name        string   `json:"Name,omitempty"`
	Values      []string `json:"Values,omitempty"`
	DisplayName string   `json:"DisplayName,omitempty"`
}

// Offers contains offer information
type Offers struct {
	Listings []Listing `json:"Listings,omitempty"`
}

// Listing represents a product listing
type Listing struct {
	Availability *Availability `json:"Availability,omitempty"`
}

// Availability represents product availability
type Availability struct {
	Message string `json:"Message,omitempty"`
	Type    string `json:"Type,omitempty"`
}

// ItemInfo contains product information
type ItemInfo struct {
	Title        *Title        `json:"Title,omitempty"`
	Features     *Features     `json:"Features,omitempty"`
	ProductInfo  *ProductInfo  `json:"ProductInfo,omitempty"`
}

// ProductInfo contains product-specific information
type ProductInfo struct {
	Color *Color `json:"Color,omitempty"`
}

// Color represents color information
type Color struct {
	DisplayValue string `json:"DisplayValue,omitempty"`
	Label        string `json:"Label,omitempty"`
}

// Title represents the product title
type Title struct {
	DisplayValue string `json:"DisplayValue"`
}

// Features represents product features
type Features struct {
	DisplayValues []string `json:"DisplayValues,omitempty"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

// GetItems retrieves items by ASIN with retries and rate limiting
func (c *Client) GetItems(ctx context.Context, asins []string) (*GetItemsResponse, error) {
	// Apply rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		metrics.RecordRateLimit()
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Build request
	request := GetItemsRequest{
		ItemIds:     asins,
		PartnerTag:  c.partnerTag,
		PartnerType: "Associates",
		Marketplace: fmt.Sprintf("www.amazon.%s", getRegionDomain(c.region)),
		Resources: []string{
			"ItemInfo.Title",
			"ItemInfo.Features",
			"ItemInfo.ProductInfo",
			"VariationSummary.VariationDimension",
			"Offers.Listings.Availability",
		},
	}

	// Execute with retries
	var response *GetItemsResponse
	var lastErr error
	
	backoff := c.retryConfig.InitialBackoff
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			metrics.RecordRetry("api_error")
			c.logger.Info("retrying API call", "attempt", attempt, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * c.retryConfig.BackoffFactor)
			if backoff > c.retryConfig.MaxBackoff {
				backoff = c.retryConfig.MaxBackoff
			}
		}

		response, lastErr = c.executeGetItems(ctx, request)
		if lastErr == nil {
			return response, nil
		}

		// Check if error is retryable
		if !isRetryableError(lastErr) {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// executeGetItems performs the actual API call
func (c *Client) executeGetItems(ctx context.Context, request GetItemsRequest) (*GetItemsResponse, error) {
	start := time.Now()
	// Marshal request to JSON
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	c.logger.Debug("executing GetItems request", "asins", request.ItemIds, "resources", request.Resources)
	
	// Create HTTP request
	url := fmt.Sprintf("https://%s/paapi5/getitems", c.host)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Sign the request
	headers := make(map[string]string)
	headers = c.signRequest("POST", "/paapi5/getitems", headers, payload)
	
	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	
	// Execute HTTP request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Log response for debugging
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("PA-API error response", 
			"status", resp.StatusCode,
			"body", string(body),
		)
		if resp.StatusCode == http.StatusTooManyRequests {
			metrics.RecordPAAPIError("RateLimitExceeded", "GetItems")
			metrics.RecordRateLimit()
			return nil, fmt.Errorf("rate limit exceeded")
		}
		if resp.StatusCode >= 500 {
			metrics.RecordPAAPIError(fmt.Sprintf("ServerError_%d", resp.StatusCode), "GetItems")
			return nil, fmt.Errorf("server error: %d", resp.StatusCode)
		}
	}
	
	// Parse response
	var response GetItemsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Check for API errors
	if len(response.Errors) > 0 {
		c.logger.Error("PA-API returned errors", "errors", response.Errors)
		metrics.RecordPAAPIError(response.Errors[0].Code, "GetItems")
		metrics.RecordPAAPIRequest("GetItems", false, time.Since(start).Seconds())
		return nil, fmt.Errorf("PA-API error: %s - %s", response.Errors[0].Code, response.Errors[0].Message)
	}
	
	// Record success metrics
	metrics.RecordPAAPIRequest("GetItems", true, time.Since(start).Seconds())
	return &response, nil
}

// GetVariations retrieves all variations for a parent ASIN with pagination
func (c *Client) GetVariations(ctx context.Context, parentASIN string) ([]*models.VariationItem, error) {
	var allVariations []*models.VariationItem
	page := 1
	maxPages := 10 // Safety limit
	
	for page <= maxPages {
		// Apply rate limiting
		if err := c.rateLimiter.Wait(ctx); err != nil {
			metrics.RecordRateLimit()
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}
		
		c.logger.Info("getting variations page", "parentASIN", parentASIN, "page", page)
		
		// Build request
		request := GetVariationsRequest{
			ASIN:         parentASIN,
			PartnerTag:   c.partnerTag,
			PartnerType:  "Associates",
			Marketplace:  fmt.Sprintf("www.amazon.%s", getRegionDomain(c.region)),
			VariationPage: page,
			VariationCount: 10, // Max allowed per page
			Resources: []string{
				"ItemInfo.Title",
				"ItemInfo.Features",
				"ItemInfo.ProductInfo",
				"VariationSummary.VariationDimension",
				"Offers.Listings.Availability",
			},
		}
		
		// Execute request with retries
		response, err := c.executeGetVariations(ctx, request)
		if err != nil {
			c.logger.Error("failed to get variations", "error", err, "parentASIN", parentASIN, "page", page)
			return allVariations, err // Return what we have so far
		}
		
		// Extract color variations from response
		if response.VariationsResult != nil {
			for _, item := range response.VariationsResult.Items {
				colorVariations := c.extractColorFromItem(item)
				allVariations = append(allVariations, colorVariations...)
			}
			
			// Check if there are more pages
			if len(response.VariationsResult.Items) < 10 {
				break // No more pages
			}
		} else {
			break // No results
		}
		
		page++
	}
	
	c.logger.Info("retrieved all variations", "parentASIN", parentASIN, "totalVariations", len(allVariations))
	return allVariations, nil
}

// executeGetVariations performs the GetVariations API call
func (c *Client) executeGetVariations(ctx context.Context, request GetVariationsRequest) (*GetVariationsResponse, error) {
	start := time.Now()
	// Marshal request to JSON
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	c.logger.Debug("executing GetVariations request", "asin", request.ASIN, "page", request.VariationPage)
	
	// Create HTTP request
	url := fmt.Sprintf("https://%s/paapi5/getvariations", c.host)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Sign the request
	headers := make(map[string]string)
	headers = c.signRequest("POST", "/paapi5/getvariations", headers, payload)
	
	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	
	// Execute HTTP request with retries
	var response *GetVariationsResponse
	var lastErr error
	
	backoff := c.retryConfig.InitialBackoff
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			metrics.RecordRetry("api_error")
			c.logger.Info("retrying GetVariations", "attempt", attempt, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * c.retryConfig.BackoffFactor)
			if backoff > c.retryConfig.MaxBackoff {
				backoff = c.retryConfig.MaxBackoff
			}
			
			// Re-create request for each retry to avoid body consumption issue
			req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			// Re-set headers
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
		
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}
		
		if resp.StatusCode == http.StatusTooManyRequests {
			metrics.RecordPAAPIError("RateLimitExceeded", "GetVariations")
			metrics.RecordRateLimit()
			lastErr = fmt.Errorf("rate limit exceeded")
			continue // Retry with backoff
		}
		
		if resp.StatusCode >= 500 {
			metrics.RecordPAAPIError(fmt.Sprintf("ServerError_%d", resp.StatusCode), "GetVariations")
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue // Retry with backoff
		}
		
		var variationsResp GetVariationsResponse
		if err := json.Unmarshal(body, &variationsResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		
		response = &variationsResp
		break
	}
	
	if response == nil {
		metrics.RecordPAAPIRequest("GetVariations", false, time.Since(start).Seconds())
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	
	// Check for errors in response
	if len(response.Errors) > 0 {
		metrics.RecordPAAPIError(response.Errors[0].Code, "GetVariations")
		metrics.RecordPAAPIRequest("GetVariations", false, time.Since(start).Seconds())
		return response, fmt.Errorf("PA-API error: %s", response.Errors[0].Message)
	}
	
	metrics.RecordPAAPIRequest("GetVariations", true, time.Since(start).Seconds())
	return response, nil
}

// extractColorFromItem extracts color variations from an item
func (c *Client) extractColorFromItem(item Item) []*models.VariationItem {
	var variations []*models.VariationItem
	
	// Check VariationAttributes for Color/Colour
	if item.VariationAttributes != nil {
		for key, value := range item.VariationAttributes {
			if strings.EqualFold(key, "Color") || strings.EqualFold(key, "Colour") {
				// Handle different value types
				switch v := value.(type) {
				case string:
					variations = append(variations, &models.VariationItem{
						Value:       v,
						DisplayName: v,
						ASIN:        item.ASIN,
						Available:   c.checkAvailability(item),
					})
				case []interface{}:
					for _, color := range v {
						if colorStr, ok := color.(string); ok {
							variations = append(variations, &models.VariationItem{
								Value:       colorStr,
								DisplayName: colorStr,
								ASIN:        item.ASIN,
								Available:   c.checkAvailability(item),
							})
						}
					}
				}
			}
		}
	}
	
	// Fallback to ItemInfo.ProductInfo.Color if no variations
	if len(variations) == 0 && item.ItemInfo != nil && item.ItemInfo.ProductInfo != nil && item.ItemInfo.ProductInfo.Color != nil {
		variations = append(variations, &models.VariationItem{
			Value:       item.ItemInfo.ProductInfo.Color.DisplayValue,
			DisplayName: item.ItemInfo.ProductInfo.Color.DisplayValue,
			ASIN:        item.ASIN,
			Available:   c.checkAvailability(item),
		})
	}
	
	return variations
}

// checkAvailability checks if an item is available
func (c *Client) checkAvailability(item Item) bool {
	if item.Offers != nil && len(item.Offers.Listings) > 0 {
		for _, listing := range item.Offers.Listings {
			if listing.Availability != nil {
				// Check if available based on Type
				if listing.Availability.Type == "Now" || listing.Availability.Type == "Available" {
					return true
				}
			}
		}
		return false
	}
	// Conservative default: assume available if no offer info
	return true
}

// EnrichColors enriches product colors using PA-API with full variation support
func (c *Client) EnrichColors(ctx context.Context, asins []string) (map[string]*ColorEnrichmentResult, error) {
	result := make(map[string]*ColorEnrichmentResult)
	normalizer := colors.NewColorNormalizer()
	
	// Process in batches of 10 (PA-API limit)
	batchSize := 10
	for i := 0; i < len(asins); i += batchSize {
		end := i + batchSize
		if end > len(asins) {
			end = len(asins)
		}
		
		batch := asins[i:end]
		metrics.RecordBatchSize(len(batch))
		response, err := c.GetItems(ctx, batch)
		if err != nil {
			c.logger.Error("failed to get items", "error", err, "batch", batch)
			continue
		}
		
		// Process each item in the response
		if response.ItemsResult != nil {
			for _, item := range response.ItemsResult.Items {
				enrichmentResult := &ColorEnrichmentResult{
					ASIN:       item.ASIN,
					ParentASIN: item.ParentASIN,
				}
				
				// If no parent ASIN, use current ASIN as parent
				if enrichmentResult.ParentASIN == "" {
					enrichmentResult.ParentASIN = item.ASIN
				}
				
				// Check if this item has color variations
				hasColorVariations := false
				if item.VariationSummary != nil {
					for _, dim := range item.VariationSummary.VariationDimensions {
						if strings.EqualFold(dim.Name, "Color") || strings.EqualFold(dim.Name, "Colour") {
							hasColorVariations = true
							break
						}
					}
				}
				
				if hasColorVariations && item.ParentASIN != "" {
					// Get all variations for parent ASIN
					variations, err := c.GetVariations(ctx, item.ParentASIN)
					if err != nil {
						c.logger.Error("failed to get variations", "error", err, "parentASIN", item.ParentASIN)
						// Fall back to single item color
						variations = c.extractColorFromItem(item)
					}
					// Normalize and deduplicate
					enrichmentResult.Colors = normalizer.ProcessVariations(variations, 100)
					if len(enrichmentResult.Colors) > 0 {
						metrics.RecordColorEnrichment(true, "variations_found", len(enrichmentResult.Colors))
					} else {
						metrics.RecordColorEnrichment(false, "no_variations", 0)
					}
				} else {
					// Extract color from single item
					variations := c.extractColorFromItem(item)
					// Normalize and deduplicate
					enrichmentResult.Colors = normalizer.ProcessVariations(variations, 100)
					if len(enrichmentResult.Colors) > 0 {
						metrics.RecordColorEnrichment(true, "variations_found", len(enrichmentResult.Colors))
					} else {
						metrics.RecordColorEnrichment(false, "no_variations", 0)
					}
				}
				
				result[item.ASIN] = enrichmentResult
			}
		}
	}
	
	return result, nil
}

// ColorEnrichmentResult contains the enriched color data for a product
type ColorEnrichmentResult struct {
	ASIN       string
	ParentASIN string
	Colors     []*models.VariationItem
}

// extractVariationsFromItem extracts color variations from an API item
func extractVariationsFromItem(item Item) []*models.VariationItem {
	var variations []*models.VariationItem

	// Check for variation attributes
	if item.VariationAttributes != nil {
		for key, value := range item.VariationAttributes {
			if strings.EqualFold(key, "Color") || strings.EqualFold(key, "Colour") {
				switch v := value.(type) {
				case string:
					variations = append(variations, &models.VariationItem{
						Value:       v,
						DisplayName: v,
						ASIN:        item.ASIN,
						Available:   true,
					})
				case []interface{}:
					for _, color := range v {
						if colorStr, ok := color.(string); ok {
							variations = append(variations, &models.VariationItem{
								Value:       colorStr,
								DisplayName: colorStr,
								ASIN:        item.ASIN,
								Available:   true,
							})
						}
					}
				}
			}
		}
	}

	// Fallback to ItemInfo.ProductInfo.Color if no variations
	if len(variations) == 0 && item.ItemInfo != nil && item.ItemInfo.ProductInfo != nil && 
		item.ItemInfo.ProductInfo.Color != nil && item.ItemInfo.ProductInfo.Color.DisplayValue != "" {
		variations = append(variations, &models.VariationItem{
			Value:       item.ItemInfo.ProductInfo.Color.DisplayValue,
			DisplayName: item.ItemInfo.ProductInfo.Color.DisplayValue,
			ASIN:        item.ASIN,
			Available:   true,
		})
	}

	return variations
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Retry on rate limits
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		return true
	}
	
	// Retry on server errors
	if strings.Contains(errStr, "server error") || strings.Contains(errStr, "5") {
		return true
	}
	
	// Retry on network errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		return true
	}
	
	// Don't retry on client errors (4xx except 429)
	if strings.Contains(errStr, "4") && !strings.Contains(errStr, "429") {
		return false
	}
	
	return false
}

// getRegionDomain returns the domain for a given region
func getRegionDomain(region string) string {
	domains := map[string]string{
		"us": "com",
		"uk": "co.uk",
		"de": "de",
		"fr": "fr",
		"jp": "co.jp",
		"ca": "ca",
		"it": "it",
		"es": "es",
	}

	if domain, ok := domains[region]; ok {
		return domain
	}
	return "com" // Default to .com
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}