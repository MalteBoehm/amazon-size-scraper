package constants

// GetPrimeEligibilitySelectors returns CSS selectors for Prime eligibility detection
func GetPrimeEligibilitySelectors() []string {
	return []string{
		"i.a-icon-prime",
		"span.a-icon-alt:has-text('Prime')",
		"div#apex_desktop_delivery_feature_div i.a-icon-prime",
		"span.a-color-base:has-text('Prime')",
	}
}

// GetStockStatusSelectors returns CSS selectors for stock status detection
func GetStockStatusSelectors() []string {
	return []string{
		"div#availability span",
		"div.a-section.a-spacing-base span.a-size-medium",
		"span.a-declarative[data-action='availability-message-show-text']",
	}
}

// GetOutOfStockKeywords returns keywords indicating out of stock status
func GetOutOfStockKeywords() []string {
	return []string{
		"nicht auf lager",
		"nicht verfügbar",
		"derzeit nicht verfügbar",
		"out of stock",
		"unavailable",
	}
}

// GetColorSwatchSelectors returns CSS selectors for color swatches
func GetColorSwatchSelectors() []string {
	return []string{
		// Support both legacy (data-defaultasin) and new (data-asin)
		"div#variation_color_name li[data-asin]",
		"div#variation_color_name li[data-defaultasin]",
		"select#native_dropdown_selected_color_name option",
		"div#variation_color_name span.selection",
	}
}

// GetSizeDropdownSelectors returns CSS selectors for size dropdowns
func GetSizeDropdownSelectors() []string {
	return []string{
		"select#native_dropdown_selected_size_name option",
		"select[name='dropdown_selected_size_name'] option",
	}
}

// GetSizeButtonSelectors returns CSS selectors for size buttons
func GetSizeButtonSelectors() []string {
	return []string{
		// Keep legacy selector; some pages might expose data-asin as well
		"div#variation_size_name li[data-asin]",
		"div#variation_size_name li[data-defaultasin]",
	}
}

// GetFeatureSelectors returns CSS selectors for product features
func GetFeatureSelectors() []string {
	return []string{
		"#feature-bullets ul.a-unordered-list span.a-list-item",
		"#feature-bullets li span",
		"div.a-section.feature span.a-list-item",
		"ul.a-unordered-list.a-vertical span.a-list-item",
		"div#feature-bullets_feature_div span.a-list-item",
	}
}

// GetPriceSelectors returns CSS selectors for price detection
func GetPriceSelectors() []string {
	return []string{
		"span.a-price-whole",
		"span#priceblock_dealprice",
		"span#priceblock_ourprice",
		"span.a-price.a-text-price.a-size-medium.apexPriceToPay",
		"span.a-price-range",
	}
}

// GetBrandSelectors returns CSS selectors for brand detection
func GetBrandSelectors() []string {
	return []string{
		"a#bylineInfo",
		"span.a-size-base.po-break-word",
		"div.a-section.a-spacing-none span.a-size-base",
	}
}

// GetImageSelectors returns CSS selectors for image extraction
func GetImageSelectors() []string {
	return []string{
		"div#altImages img",
		"#landingImage",
	}
}

// GetBreadcrumbSelectors returns CSS selectors for breadcrumb navigation
func GetBreadcrumbSelectors() []string {
	return []string{
		"div#wayfinding-breadcrumbs_feature_div a",
		"div#wayfinding-breadcrumbs_feature_div a.a-link-normal",
	}
}

// GetMaterialDetailSelectors returns CSS selectors for material details
func GetMaterialDetailSelectors() []string {
	return []string{
		"div#productDetails_feature_div tr",
		"div#detailBullets_feature_div li",
		"table.prodDetTable tr",
	}
}
