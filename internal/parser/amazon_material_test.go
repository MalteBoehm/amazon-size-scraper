package parser

import (
    "testing"
)

func TestExtractMaterialFromBullets(t *testing.T) {
    html := `
    <html><body>
      <div id="feature-bullets">
        <ul>
          <li>Material: the women's shirt is made from 75% cotton, 20% polyester, 5% spandex, stretchy and breathable.</li>
        </ul>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN1")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected material composition, got empty: %#v", got)
    }
    if got["cotton"] < 74.9 || got["cotton"] > 75.1 {
        t.Errorf("expected cotton 75, got %v", got["cotton"])
    }
    if got["polyester"] < 19.9 || got["polyester"] > 20.1 {
        t.Errorf("expected polyester 20, got %v", got["polyester"])
    }
    if got["elastane"] < 4.9 || got["elastane"] > 5.1 {
        t.Errorf("expected elastane 5, got %v", got["elastane"])
    }
}

func TestExtractMaterialFromFactsNoPercent(t *testing.T) {
    html := `
    <html><body>
      <div class="product-facts-detail">
        <div class="a-col-left"><span></span><span>Materialzusammensetzung</span></div>
        <div class="a-col-right"><span></span><span>Baumwolle, Polyester</span></div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN2")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected keys for materials, got empty")
    }
    if _, ok := got["cotton"]; !ok {
        t.Errorf("expected cotton key present")
    }
    if _, ok := got["polyester"]; !ok {
        t.Errorf("expected polyester key present")
    }
}

func TestExtractMaterialFromFixedLeftGridLayout(t *testing.T) {
    html := `
    <html><body>
      <div class="a-fixed-left-grid-inner" style="padding-left:140px">
        <div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
          <span style="font-weight: 600;">
            <span class="a-color-base">Materialzusammensetzung</span>
          </span>
        </div>
        <div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
          <span style="font-weight: 400;">
            <span class="a-color-base">Baumwolle，Polyster</span>
          </span>
        </div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN3")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected keys for materials, got empty")
    }
    if _, ok := got["cotton"]; !ok {
        t.Errorf("expected cotton key present from Baumwolle")
    }
    if _, ok := got["polyester"]; !ok {
        t.Errorf("expected polyester key present from Polyster (misspelled)")
    }
}

func TestExtractMaterialWithChineseComma(t *testing.T) {
    html := `
    <html><body>
      <div class="a-fixed-left-grid-inner" style="padding-left:140px">
        <div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
          <span style="font-weight: 600;">
            <span class="a-color-base">Materialzusammensetzung</span>
          </span>
        </div>
        <div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
          <span style="font-weight: 400;">
            <span class="a-color-base">60% Baumwolle，40% Polyester</span>
          </span>
        </div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN4")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected material composition with percentages, got empty")
    }
    if got["cotton"] < 59.9 || got["cotton"] > 60.1 {
        t.Errorf("expected cotton 60, got %v", got["cotton"])
    }
    if got["polyester"] < 39.9 || got["polyester"] > 40.1 {
        t.Errorf("expected polyester 40, got %v", got["polyester"])
    }
}

func TestExtractMaterialWithDifferentSeparators(t *testing.T) {
    html := `
    <html><body>
      <div class="product-facts-detail">
        <div class="a-col-left"><span></span><span>Material</span></div>
        <div class="a-col-right"><span></span><span>Baumwolle / Polyester; Elasthan</span></div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN5")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected keys for materials, got empty")
    }
    if _, ok := got["cotton"]; !ok {
        t.Errorf("expected cotton key present")
    }
    if _, ok := got["polyester"]; !ok {
        t.Errorf("expected polyester key present")
    }
    if _, ok := got["elastane"]; !ok {
        t.Errorf("expected elastane key present")
    }
}

func TestExtractMaterialFromAlternativeSection(t *testing.T) {
    html := `
    <html><body>
      <div class="a-section a-spacing-medium">
        <div>
          <span>Obermaterial</span>
          <div>100% Baumwolle</div>
        </div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN6")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    got := prod.MaterialComposition
    if got == nil || len(got) == 0 {
        t.Fatalf("expected material composition, got empty")
    }
    if got["cotton"] < 99.9 || got["cotton"] > 100.1 {
        t.Errorf("expected cotton 100, got %v", got["cotton"])
    }

    // Test the new MaterialInfo structure
    if prod.MaterialInfo == nil {
        t.Fatalf("expected MaterialInfo to be populated, got nil")
    }

    if prod.MaterialInfo.Source != "alternative_sections" {
        t.Errorf("expected source 'alternative_sections', got %s", prod.MaterialInfo.Source)
    }

    if prod.MaterialInfo.Confidence < 0.79 || prod.MaterialInfo.Confidence > 0.81 {
        t.Errorf("expected confidence 0.8, got %v", prod.MaterialInfo.Confidence)
    }

    if prod.MaterialInfo.RawText != "100% Baumwolle" {
        t.Errorf("expected raw text '100%% Baumwolle', got %s", prod.MaterialInfo.RawText)
    }

    if prod.MaterialInfo.ExtractionMethod != "structured" {
        t.Errorf("expected extraction method 'structured', got %s", prod.MaterialInfo.ExtractionMethod)
    }
}

func TestMaterialInfoStructureWithFixedGrid(t *testing.T) {
    html := `
    <html><body>
      <div class="a-fixed-left-grid-inner" style="padding-left:140px">
        <div class="a-fixed-left-grid-col a-col-left" style="width:140px;margin-left:-140px;float:left;">
          <span style="font-weight: 600;">
            <span class="a-color-base">Materialzusammensetzung</span>
          </span>
        </div>
        <div class="a-fixed-left-grid-col a-col-right" style="padding-left:6%;float:left;">
          <span style="font-weight: 400;">
            <span class="a-color-base">60% Baumwolle，40% Polyester</span>
          </span>
        </div>
      </div>
    </body></html>`

    p := NewAmazonParser()
    prod, err := p.ParseProductPage(html, "TESTASIN7")
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    if prod.MaterialInfo == nil {
        t.Fatalf("expected MaterialInfo to be populated, got nil")
    }

    if prod.MaterialInfo.Source != "fixed_grid" {
        t.Errorf("expected source 'fixed_grid', got %s", prod.MaterialInfo.Source)
    }

    if prod.MaterialInfo.Confidence < 0.94 || prod.MaterialInfo.Confidence > 0.96 {
        t.Errorf("expected confidence 0.95, got %v", prod.MaterialInfo.Confidence)
    }

    if prod.MaterialInfo.RawText != "60% Baumwolle，40% Polyester" {
        t.Errorf("expected raw text '60%% Baumwolle，40%% Polyester', got %s", prod.MaterialInfo.RawText)
    }

    if prod.MaterialInfo.ExtractionMethod != "structured" {
        t.Errorf("expected extraction method 'structured', got %s", prod.MaterialInfo.ExtractionMethod)
    }

    if len(prod.MaterialInfo.AlternativeTexts) == 0 {
        t.Errorf("expected alternative texts to contain at least one entry")
    }
}
