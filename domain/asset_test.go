package domain_test

import (
	"testing"

	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/stretchr/testify/assert"
)

func TestAssetTypeConstants(t *testing.T) {
	assert.Equal(t, domain.AssetType("FIAT"), domain.AssetTypeFiat)
	assert.Equal(t, domain.AssetType("TAPROOT"), domain.AssetTypeTaproot)
	assert.Equal(t, domain.AssetType("LIGHTNING"), domain.AssetTypeLightning)
}

func TestAssetFields(t *testing.T) {
	a := domain.Asset{
		ID:       "asset-1",
		Ticker:   "BTC",
		Name:     "Bitcoin",
		Decimals: 8,
		Type:     domain.AssetTypeLightning,
	}
	assert.Equal(t, "asset-1", a.ID)
	assert.Equal(t, "BTC", a.Ticker)
	assert.Equal(t, "Bitcoin", a.Name)
	assert.Equal(t, int32(8), a.Decimals)
	assert.Equal(t, domain.AssetTypeLightning, a.Type)
}

func TestAssetBalanceEmbedding(t *testing.T) {
	b := domain.AssetBalance{
		Asset: domain.Asset{
			ID:     "usd-1",
			Ticker: "USD",
			Type:   domain.AssetTypeFiat,
		},
		Amount: 10000,
	}
	assert.Equal(t, "usd-1", b.ID)
	assert.Equal(t, "USD", b.Ticker)
	assert.Equal(t, domain.AssetTypeFiat, b.Type)
	assert.Equal(t, uint64(10000), b.Amount)
}
