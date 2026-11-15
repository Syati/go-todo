package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductValidationWithJapaneseNames(t *testing.T) {
	p := Product{Name: strings.Repeat("椅", 100), Status: "draft", TaxRate: 8}
	require.NoError(t, p.Validate())
	p.Name += "子"
	assert.Error(t, p.Validate())
	p.Name = "椅子"
	p.Status = "invalid"
	assert.Error(t, p.Validate())
	p.Status = "on_sale"
	p.TaxRate = 9
	assert.Error(t, p.Validate())
}
func TestSKUNormalizationAndLimits(t *testing.T) {
	s := SKU{Code: " chair-01 ", Size: " M ", Color: " 青 ", PriceYen: 10000000}
	s.Normalize()
	assert.Equal(t, "CHAIR-01", s.Code)
	assert.Equal(t, "M", s.Size)
	assert.Equal(t, "青", s.Color)
	require.NoError(t, s.Validate())
	s.PriceYen++
	assert.Error(t, s.Validate())
	s.PriceYen = 0
	s.StockQuantity = -1
	assert.Error(t, s.Validate())
	s.StockQuantity = 0
	s.Code = "-INVALID"
	assert.Error(t, s.Validate())
}
