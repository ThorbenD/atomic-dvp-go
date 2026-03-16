package domain_test

import (
	"testing"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/stretchr/testify/assert"
)

func TestHTLCStatusConstants(t *testing.T) {
	assert.Equal(t, domain.HTLCStatus("PENDING"), domain.HTLCStatusPending)
	assert.Equal(t, domain.HTLCStatus("CONFIRMED"), domain.HTLCStatusConfirmed)
	assert.Equal(t, domain.HTLCStatus("CLAIMED"), domain.HTLCStatusClaimed)
}

func TestHTLCFields(t *testing.T) {
	now := time.Now()
	h := domain.HTLC{
		Hash:       "abc123",
		Amount:     50000,
		Expiry:     800000,
		Status:     domain.HTLCStatusPending,
		DetectedAt: now,
	}
	assert.Equal(t, "abc123", h.Hash)
	assert.Equal(t, uint64(50000), h.Amount)
	assert.Equal(t, int64(800000), h.Expiry)
	assert.Equal(t, domain.HTLCStatusPending, h.Status)
	assert.Equal(t, now, h.DetectedAt)
}

func TestHTLCStatusDistinct(t *testing.T) {
	statuses := []domain.HTLCStatus{
		domain.HTLCStatusPending,
		domain.HTLCStatusConfirmed,
		domain.HTLCStatusClaimed,
	}
	seen := map[domain.HTLCStatus]bool{}
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate status: %s", s)
		seen[s] = true
	}
}
