package scraper

import (
	"context"
	"errors"
	
	"github.com/maltedev/amazon-size-scraper/internal/models"
)

var (
	ErrInvalidURL       = errors.New("invalid Amazon URL")
	ErrProductNotFound  = errors.New("product not found")
	ErrDimensionsNotFound = errors.New("dimensions not found")
	ErrRateLimited      = errors.New("rate limited by Amazon")
	ErrBlocked          = errors.New("blocked by Amazon anti-bot")
)

type Scraper interface {
	ScrapeProduct(ctx context.Context, url string) (*models.Product, error)
	ScrapeByASIN(ctx context.Context, asin string) (*models.Product, error)
	ExtractASIN(url string) (string, error)
	Close() error
}

type Parser interface {
	ParseProductPage(html string) (*models.Product, error)
	ExtractDimensions(html string) (*models.Dimension, error)
	ExtractWeight(html string) (*models.Weight, error)
	ExtractPrice(html string) (*models.Price, error)
}

type Options struct {
	MaxRetries      int
	RetryDelay      int
	UserAgents      []string
	Proxies         []string
	ConcurrentLimit int
	RateLimit       int
}