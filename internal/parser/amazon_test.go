package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMaterial(t *testing.T) {
	parser := NewAmazonParser()

	tests := []struct {
		name     string
		html     string
		expected string
		hasError bool
	}{
		{
			name: "Material composition from structured HTML",
			html: `<div class="a-fixed-left-grid-inner" style="padding-left:140px">
						<div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
							<span style="font-weight: 600;">
								<span class="a-color-base">Materialzusammensetzung</span>
							</span>
						</div>
						<div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
							<span style="font-weight: 400;">
								<span class="a-color-base">95% Baumwolle, 5% Elasthan</span>
							</span>
						</div>
					</div>`,
			expected: "95% Baumwolle, 5% Elasthan",
			hasError: false,
		},
		{
			name:     "Material composition from regex pattern",
			html:     `<div>Materialzusammensetzung: 80% Polyester, 20% Baumwolle</div>`,
			expected: "80% Polyester, 20% Baumwolle",
			hasError: false,
		},
		{
			name:     "Material from different pattern",
			html:     `<div>Material: 100% Baumwolle</div>`,
			expected: "100% Baumwolle",
			hasError: false,
		},
		{
			name:     "No material found",
			html:     `<div>Some other content</div>`,
			expected: "",
			hasError: true,
		},
		{
			name:     "Case insensitive material extraction",
			html:     `<div>MATERIALZUSAMMENSETZUNG: 70% Cotton, 30% Polyester</div>`,
			expected: "70% Cotton, 30% Polyester",
			hasError: false,
		},
		{
			name: "Material with multiple components",
			html: `<div class="a-fixed-left-grid-inner" style="padding-left:140px">
						<div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
							<span style="font-weight: 600;">
								<span class="a-color-base">Material</span>
							</span>
						</div>
						<div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
							<span style="font-weight: 400;">
								<span class="a-color-base">65% Polyester, 30% Viskose, 5% Elasthan</span>
							</span>
						</div>
					</div>`,
			expected: "65% Polyester, 30% Viskose, 5% Elasthan",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ExtractMaterial(tt.html)

			if tt.hasError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExtractMaterialFromFullProductPage(t *testing.T) {
	parser := NewAmazonParser()

	html := `<!DOCTYPE html>
<html>
<body>
	<div id="productTitle">Test T-Shirt</div>
	<div class="a-fixed-left-grid-inner" style="padding-left:140px">
		<div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
			<span style="font-weight: 600;">
				<span class="a-color-base">Materialzusammensetzung</span>
			</span>
		</div>
		<div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
			<span style="font-weight: 400;">
				<span class="a-color-base">95% Baumwolle, 5% Elasthan</span>
			</span>
		</div>
	</div>
	<div id="feature-bullets">
		<ul>
			<li>Material: 95% Baumwolle, 5% Elasthan</li>
		</ul>
	</div>
</body>
</html>`

	result, err := parser.ExtractMaterial(html)
	require.NoError(t, err)
	assert.Equal(t, "95% Baumwolle, 5% Elasthan", result)
}

func TestExtractMaterialHandlesNotFound(t *testing.T) {
	parser := NewAmazonParser()

	html := `<!DOCTYPE html>
<html>
<body>
	<div id="productTitle">Test Product</div>
	<div id="feature-bullets">
		<ul>
			<li>Color: Blue</li>
			<li>Size: M</li>
		</ul>
	</div>
</body>
</html>`

	result, err := parser.ExtractMaterial(html)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "material not found")
	assert.Empty(t, result)
}