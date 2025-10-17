package parser

import (
	"github.com/maltedev/amazon-size-scraper/internal/models"
)

type Parser interface {
	ParseProductPage(html string, asin string) (*models.Product, error)
	ExtractDimensions(html string) (*models.Dimension, error)
	ExtractWeight(html string) (*models.Weight, error)
	ExtractPrice(html string) (*models.Price, error)
	ExtractMaterial(html string) (string, error)
}