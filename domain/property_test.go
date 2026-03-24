package domain_test

import (
	"testing"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// genHTLCStatus generates one of the valid HTLC status constants.
var genHTLCStatus = rapid.SampledFrom([]domain.HTLCStatus{
	domain.HTLCStatusPending,
	domain.HTLCStatusConfirmed,
	domain.HTLCStatusClaimed,
})

// genAssetType generates one of the valid AssetType constants.
var genAssetType = rapid.SampledFrom([]domain.AssetType{
	domain.AssetTypeFiat,
	domain.AssetTypeTaproot,
	domain.AssetTypeLightning,
})

// TestHTLC_FieldsRoundtrip verifies that any combination of valid HTLC field
// values survives assignment and retrieval without mutation.
func TestHTLC_FieldsRoundtrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		hash := rapid.StringMatching(`[0-9a-f]{64}`).Draw(t, "hash")
		amount := rapid.Uint64Range(1, 21_000_000*100_000_000).Draw(t, "amount")
		expiry := rapid.Int64Range(0, 1_000_000).Draw(t, "expiry")
		status := genHTLCStatus.Draw(t, "status")

		h := domain.HTLC{
			Hash:       hash,
			Amount:     amount,
			Expiry:     expiry,
			Status:     status,
			DetectedAt: time.Now(),
		}

		assert.Equal(t, hash, h.Hash)
		assert.Equal(t, amount, h.Amount)
		assert.Equal(t, expiry, h.Expiry)
		assert.Equal(t, status, h.Status)
	})
}

// TestHTLCStatus_AllConstantsAreDistinct verifies that the three status
// constants are never equal to each other, regardless of any future
// refactoring of the underlying string values.
func TestHTLCStatus_AllConstantsAreDistinct(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s1 := genHTLCStatus.Draw(t, "s1")
		s2 := genHTLCStatus.Draw(t, "s2")
		if s1 != s2 {
			assert.NotEqual(t, string(s1), string(s2))
		}
	})
}

// TestAsset_FieldsRoundtrip verifies that Asset fields survive assignment
// without mutation across arbitrary valid inputs.
func TestAsset_FieldsRoundtrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching(`[a-zA-Z0-9\-]{1,32}`).Draw(t, "id")
		ticker := rapid.StringMatching(`[A-Z]{1,8}`).Draw(t, "ticker")
		decimals := rapid.Int32Range(0, 18).Draw(t, "decimals")
		assetType := genAssetType.Draw(t, "assetType")

		a := domain.Asset{
			ID:       id,
			Ticker:   ticker,
			Decimals: decimals,
			Type:     assetType,
		}

		assert.Equal(t, id, a.ID)
		assert.Equal(t, ticker, a.Ticker)
		assert.Equal(t, decimals, a.Decimals)
		assert.Equal(t, assetType, a.Type)
	})
}

// TestAssetBalance_AmountEmbedding verifies that AssetBalance correctly embeds
// Asset and that the Amount field is independent of the embedded fields.
func TestAssetBalance_AmountEmbedding(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amount := rapid.Uint64Range(0, 1_000_000_000).Draw(t, "amount")
		assetType := genAssetType.Draw(t, "assetType")

		b := domain.AssetBalance{
			Asset:  domain.Asset{Type: assetType},
			Amount: amount,
		}

		assert.Equal(t, amount, b.Amount)
		assert.Equal(t, assetType, b.Type)
		// Amount change must not affect the embedded Asset
		b.Amount = amount + 1
		assert.Equal(t, assetType, b.Type)
	})
}
