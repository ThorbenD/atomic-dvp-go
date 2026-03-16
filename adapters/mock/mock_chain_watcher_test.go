package mock_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/adapters/mock"
	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validPreimage/validHash form a proper SHA256 preimage→hash pair.
const (
	validPreimage = "0000000000000000000000000000000000000000000000000000000000000001"
)

func validHash() string {
	b, _ := hex.DecodeString(validPreimage)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestMockChainWatcher_DetectHTLC_Found(t *testing.T) {
	w := mock.NewMockChainWatcher()
	hash := validHash()

	go func() {
		time.Sleep(50 * time.Millisecond)
		w.SimulateIncomingHTLC(hash, 100_000)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	htlc, err := w.DetectHTLC(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, hash, htlc.Hash)
	assert.Equal(t, uint64(100_000), htlc.Amount)
	assert.Equal(t, domain.HTLCStatusConfirmed, htlc.Status)
}

func TestMockChainWatcher_DetectHTLC_ContextCancel(t *testing.T) {
	w := mock.NewMockChainWatcher()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := w.DetectHTLC(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMockChainWatcher_ClaimHTLC_ValidPreimage(t *testing.T) {
	w := mock.NewMockChainWatcher()
	hash := validHash()
	w.SimulateIncomingHTLC(hash, 50_000)

	txID, err := w.ClaimHTLC(context.Background(), validPreimage)
	require.NoError(t, err)
	assert.Contains(t, txID, "tx_mock_sweep_")
	assert.Contains(t, txID, validPreimage[:8])
}

func TestMockChainWatcher_ClaimHTLC_InvalidPreimage(t *testing.T) {
	w := mock.NewMockChainWatcher()
	// No HTLC in map — preimage won't hash to anything stored

	_, err := w.ClaimHTLC(context.Background(), validPreimage)
	assert.Error(t, err)
}

func TestMockChainWatcher_ClaimHTLC_AlreadyClaimed(t *testing.T) {
	w := mock.NewMockChainWatcher()
	hash := validHash()
	w.SimulateIncomingHTLC(hash, 50_000)

	_, err := w.ClaimHTLC(context.Background(), validPreimage)
	require.NoError(t, err)

	// Second claim must fail
	_, err = w.ClaimHTLC(context.Background(), validPreimage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already claimed")
}

func TestMockChainWatcher_SimulateIncomingHTLC_WritesMap(t *testing.T) {
	w := mock.NewMockChainWatcher()
	hash := validHash()
	w.SimulateIncomingHTLC(hash, 42_000)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	htlc, err := w.DetectHTLC(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, uint64(42_000), htlc.Amount)
}
