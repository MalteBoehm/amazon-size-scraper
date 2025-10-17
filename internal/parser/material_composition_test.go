package parser

import (
	"testing"

	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMaterialComposition(t *testing.T) {
	parser := NewAmazonParser()

	tests := []struct {
		name                       string
		html                       string
		expectedComposition        *models.MaterialComposition
		expectedFullTextContains   []string
		hasError                    bool
	}{
		{
			name: "Structured material composition",
			html: `<div class="a-fixed-left-grid-inner" style="padding-left:140px">
						<div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
							<span style="font-weight: 600;">
								<span class="a-color-base">Materialzusammensetzung</span>
							</span>
						</div>
						<div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
							<span style="font-weight: 400;">
								<span class="a-color-base">80% Baumwolle, 20% Polyester</span>
							</span>
						</div>
					</div>`,
			expectedComposition: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 80.0},
					{Name: "Polyester", Percent: 20.0},
				},
				Confidence: 0.95,
				Source:     "structured",
			},
			expectedFullTextContains: []string{"Materialzusammensetzung: 80% Baumwolle, 20% Polyester"},
			hasError: false,
		},
		{
			name: "Single material 100%",
			html: `<div>Material: 100% Baumwolle</div>`,
			expectedComposition: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 100.0},
				},
				Confidence: 0.95,
				Source:     "regex",
			},
			expectedFullTextContains: []string{"100% Baumwolle"},
			hasError: false,
		},
		{
			name: "No material found",
			html: `<div>Color: Blue</div><div>Size: M</div>`,
			expectedComposition: nil,
			expectedFullTextContains: []string{},
			hasError: true,
		},
		{
			name: "Material with decimal percentages",
			html: `<div>Materialzusammensetzung: 80,5% Baumwolle, 19,5% Elasthan</div>`,
			expectedComposition: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 80.5},
					{Name: "Elasthan", Percent: 19.5},
				},
				Confidence: 0.95,
				Source:     "regex",
			},
			expectedFullTextContains: []string{"80,5% Baumwolle, 19,5% Elasthan"},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composition, fullText, err := parser.ExtractMaterialComposition(tt.html)

			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, composition)
			} else {
				require.NoError(t, err)
				require.NotNil(t, composition)

				assert.Equal(t, tt.expectedComposition.Source, composition.Source)
				assert.Equal(t, tt.expectedComposition.Confidence, composition.Confidence)
				require.Equal(t, len(tt.expectedComposition.Materials), len(composition.Materials))

				for i, expectedMat := range tt.expectedComposition.Materials {
					assert.Equal(t, expectedMat.Name, composition.Materials[i].Name)
					assert.Equal(t, expectedMat.Percent, composition.Materials[i].Percent)
				}

				for _, expectedContain := range tt.expectedFullTextContains {
					assert.Contains(t, fullText, expectedContain)
				}
			}
		})
	}
}

func TestParseMaterialComposition(t *testing.T) {
	parser := NewAmazonParser()

	tests := []struct {
		name        string
		text        string
		expected    *models.MaterialComposition
		shouldError bool
	}{
		{
			name: "Two materials with exact percentages",
			text: "80% Baumwolle, 20% Polyester",
			expected: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 80.0},
					{Name: "Polyester", Percent: 20.0},
				},
				Confidence: 0.95,
			},
			shouldError: false,
		},
		{
			name: "Three materials",
			text: "70% Baumwolle, 25% Polyester, 5% Elasthan",
			expected: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 70.0},
					{Name: "Polyester", Percent: 25.0},
					{Name: "Elasthan", Percent: 5.0},
				},
				Confidence: 0.95,
			},
			shouldError: false,
		},
		{
			name: "Single material 100%",
			text: "100% Baumwolle",
			expected: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 100.0},
				},
				Confidence: 0.95,
			},
			shouldError: false,
		},
		{
			name: "With decimal comma",
			text: "80,5% Baumwolle, 19,5% Elasthan",
			expected: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 80.5},
					{Name: "Elasthan", Percent: 19.5},
				},
				Confidence: 0.95,
			},
			shouldError: false,
		},
		{
			name: "Incomplete percentages (low confidence)",
			text: "60% Baumwolle, 15% Polyester",
			expected: &models.MaterialComposition{
				Materials: []models.MaterialItem{
					{Name: "Baumwolle", Percent: 60.0},
					{Name: "Polyester", Percent: 15.0},
				},
				Confidence: 0.7,
			},
			shouldError: false,
		},
		{
			name:        "No percentages found",
			text:        "Baumwolle und Polyester",
			expected:    nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseMaterialComposition(tt.text)

			if tt.shouldError {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Confidence, result.Confidence)
				require.Equal(t, len(tt.expected.Materials), len(result.Materials))

				for i, expectedMat := range tt.expected.Materials {
					assert.Equal(t, expectedMat.Name, result.Materials[i].Name)
					assert.Equal(t, expectedMat.Percent, result.Materials[i].Percent)
				}
			}
		})
	}
}