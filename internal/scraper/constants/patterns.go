package constants

// GetMaterialKeywords returns keywords for material detection
func GetMaterialKeywords() []string {
	return []string{
		// German materials
		"baumwolle", "polyester", "elasthan", "wolle", "seide", 
		"leinen", "viskose", "modal", "lyocell", "nylon",
		"acryl", "polyamid", "kaschmir", "merinowolle", "bambus",
		
		// English materials
		"cotton", "spandex", "wool", "silk", "linen",
		"viscose", "rayon", "cashmere", "merino", "bamboo",
		"acrylic", "polyamide", "elastane", "lycra",
		
		// Material composition indicators
		"material:", "obermaterial:", "zusammensetzung:", "stoff:",
		"fabric:", "composition:", "made of", "hergestellt aus",
	}
}

// GetSizeTablePatterns returns patterns for size table detection
func GetSizeTablePatterns() []string {
	return []string{
		"Größentabelle",
		"Size Chart",
		"Größenratgeber",
		"size-chart",
		"size_chart",
	}
}

// GetFeatureExclusionPatterns returns patterns to exclude from features
func GetFeatureExclusionPatterns() []string {
	return []string{
		"Weitere Informationen",
		"P.when",
		"function(",
	}
}

// GetColorPrefixPatterns returns patterns for color code prefixes to remove
func GetColorPrefixPatterns() []string {
	return []string{
		`^X\d+\s+`,      // X363 Army Green -> Army Green
		`^\d{1,5}\s+`,   // 123 Blue -> Blue
		`^[A-Z]\d+\s+`,  // A12 Red -> Red
	}
}

// GetTitlePrefixPatterns returns patterns to clean from titles
func GetTitlePrefixPatterns() []string {
	return []string{
		"Wähle ",
		" durch Klicken aus",
		"Klicken Sie, um ",
		"Click to select ",
		" auszuwählen",
	}
}

// GetDefaultOptionValues returns values that indicate default/placeholder options
func GetDefaultOptionValues() []string {
	return []string{
		"-1",
		"Auswählen",
		"Select",
		"Choose",
		"Wählen",
	}
}

// GetCurrencySymbols returns currency symbols and codes
func GetCurrencySymbols() []string {
	return []string{
		"€",
		"EUR",
		"$",
		"USD",
		"£",
		"GBP",
	}
}

// GetImageSizeReplacements returns image URL replacements for full-size images
func GetImageSizeReplacements() map[string]string {
	return map[string]string{
		"_AC_US40_":     "_AC_SL1500_",
		"_AC_SR38,50_":  "_AC_SL1500_",
		"_AC_UL160_":    "_AC_SL1500_",
		"_AC_UL320_":    "_AC_SL1500_",
		"_AC_UY218_":    "_AC_SL1500_",
		"_AC_UY500_":    "_AC_SL1500_",
	}
}

// GetSizeMeasurementKeys returns keys for size measurements
func GetSizeMeasurementKeys() map[string]string {
	return map[string]string{
		// German to standard
		"brust":    "chest",
		"länge":    "length",
		"schulter": "shoulder",
		"ärmel":    "sleeve",
		"taille":   "waist",
		"hüfte":    "hip",
		"breite":   "width",
		
		// English to standard
		"chest":    "chest",
		"length":   "length",
		"shoulder": "shoulder",
		"sleeve":   "sleeve",
		"waist":    "waist",
		"hip":      "hip",
		"width":    "width",
	}
}