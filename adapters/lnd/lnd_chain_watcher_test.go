package lnd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testHash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"

func makeUpdateChannels() (chan *settlement.InvoiceUpdate, chan error) {
	return make(chan *settlement.InvoiceUpdate, 4), make(chan error, 1)
}

// TestLndChainWatcher_DetectHTLC exercises the full state-machine of DetectHTLC
// via a table of scenarios, each with its own update sequence.
func TestLndChainWatcher_DetectHTLC(t *testing.T) {
	type result struct {
		status domain.HTLCStatus
		amount uint64
	}

	tests := []struct {
		name            string
		updates         []*settlement.InvoiceUpdate
		streamErr       error
		closeUpdates    bool
		wantResult      *result // nil means an error is expected
		wantErrContains string
	}{
		{
			name: "accepted immediately",
			updates: []*settlement.InvoiceUpdate{
				{Hash: testHash, State: settlement.InvoiceStateAccepted, Amt: 50_000},
			},
			wantResult: &result{domain.HTLCStatusConfirmed, 50_000},
		},
		{
			name: "open then accepted",
			updates: []*settlement.InvoiceUpdate{
				{Hash: testHash, State: settlement.InvoiceStateOpen, Amt: 0},
				{Hash: testHash, State: settlement.InvoiceStateAccepted, Amt: 60_000},
			},
			wantResult: &result{domain.HTLCStatusConfirmed, 60_000},
		},
		{
			name: "settled",
			updates: []*settlement.InvoiceUpdate{
				{Hash: testHash, State: settlement.InvoiceStateSettled, Amt: 55_000},
			},
			wantResult: &result{domain.HTLCStatusClaimed, 55_000},
		},
		{
			name: "canceled",
			updates: []*settlement.InvoiceUpdate{
				{Hash: testHash, State: settlement.InvoiceStateCanceled},
			},
			wantErrContains: "canceled",
		},
		{
			name:            "stream error",
			streamErr:       fmt.Errorf("rpc error: stream broken"),
			wantErrContains: "stream error",
		},
		{
			name:            "channel closed",
			closeUpdates:    true,
			wantErrContains: "closed",
		},
		{
			name:            "subscribe fails",
			wantErrContains: "subscribe invoice failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockLightningClient{}
			updateCh, errCh := makeUpdateChannels()

			for _, u := range tc.updates {
				updateCh <- u
			}
			if tc.streamErr != nil {
				errCh <- tc.streamErr
			}
			if tc.closeUpdates {
				close(updateCh)
			}

			if tc.name == "subscribe fails" {
				mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
					Return(nil, nil, fmt.Errorf("connection refused"))
			} else {
				mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
					Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)
			}

			watcher := adapterlnd.NewLndChainWatcher(mockClient)
			htlc, err := watcher.DetectHTLC(context.Background(), testHash)

			if tc.wantResult != nil {
				require.NoError(t, err)
				assert.Equal(t, tc.wantResult.status, htlc.Status)
				assert.Equal(t, tc.wantResult.amount, htlc.Amount)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestLndChainWatcher_DetectHTLC_ContextCancel is kept separate because it
// requires a goroutine to cancel the context mid-flight.
func TestLndChainWatcher_DetectHTLC_ContextCancel(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh := make(chan *settlement.InvoiceUpdate) // never receives
	errCh := make(chan error)

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.DetectHTLC(ctx, testHash)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestLndChainWatcher_ClaimHTLC_Success(t *testing.T) {
	mockClient := &MockLightningClient{}
	mockClient.On("SettleInvoice", mock.Anything, "mypreimage").Return(nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	txID, err := watcher.ClaimHTLC(context.Background(), "mypreimage")

	require.NoError(t, err)
	assert.Equal(t, "off-chain-settled", txID)
	mockClient.AssertExpectations(t)
}

func TestLndChainWatcher_ClaimHTLC_Error(t *testing.T) {
	mockClient := &MockLightningClient{}
	mockClient.On("SettleInvoice", mock.Anything, "badpreimage").
		Return(fmt.Errorf("unknown payment hash"))

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.ClaimHTLC(context.Background(), "badpreimage")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lnd settle failed")
}
