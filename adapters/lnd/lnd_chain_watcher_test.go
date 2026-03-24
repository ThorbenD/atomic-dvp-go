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

func TestLndChainWatcher_DetectHTLC_Accepted(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	updateCh <- &settlement.InvoiceUpdate{Hash: testHash, State: settlement.InvoiceStateAccepted, Amt: 50_000}

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	htlc, err := watcher.DetectHTLC(context.Background(), testHash)

	require.NoError(t, err)
	assert.Equal(t, testHash, htlc.Hash)
	assert.Equal(t, uint64(50_000), htlc.Amount)
	assert.Equal(t, domain.HTLCStatusConfirmed, htlc.Status)
	mockClient.AssertExpectations(t)
}

func TestLndChainWatcher_DetectHTLC_OpenThenAccepted(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	updateCh <- &settlement.InvoiceUpdate{Hash: testHash, State: settlement.InvoiceStateOpen, Amt: 0}
	updateCh <- &settlement.InvoiceUpdate{Hash: testHash, State: settlement.InvoiceStateAccepted, Amt: 60_000}

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	htlc, err := watcher.DetectHTLC(context.Background(), testHash)

	require.NoError(t, err)
	assert.Equal(t, uint64(60_000), htlc.Amount)
	assert.Equal(t, domain.HTLCStatusConfirmed, htlc.Status)
}

func TestLndChainWatcher_DetectHTLC_Settled(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	updateCh <- &settlement.InvoiceUpdate{Hash: testHash, State: settlement.InvoiceStateSettled, Amt: 55_000}

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	htlc, err := watcher.DetectHTLC(context.Background(), testHash)

	require.NoError(t, err)
	assert.Equal(t, domain.HTLCStatusClaimed, htlc.Status)
	assert.Equal(t, uint64(55_000), htlc.Amount)
}

func TestLndChainWatcher_DetectHTLC_Canceled(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	updateCh <- &settlement.InvoiceUpdate{Hash: testHash, State: settlement.InvoiceStateCanceled}

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.DetectHTLC(context.Background(), testHash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestLndChainWatcher_DetectHTLC_StreamError(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	errCh <- fmt.Errorf("rpc error: stream broken")

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.DetectHTLC(context.Background(), testHash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream error")
}

func TestLndChainWatcher_DetectHTLC_ChannelClosed(t *testing.T) {
	mockClient := &MockLightningClient{}
	updateCh, errCh := makeUpdateChannels()

	// Close both channels immediately
	close(updateCh)

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.DetectHTLC(context.Background(), testHash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

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

func TestLndChainWatcher_DetectHTLC_SubscribeError(t *testing.T) {
	mockClient := &MockLightningClient{}

	mockClient.On("SubscribeSingleInvoice", mock.Anything, testHash).
		Return(nil, nil, fmt.Errorf("connection refused"))

	watcher := adapterlnd.NewLndChainWatcher(mockClient)
	_, err := watcher.DetectHTLC(context.Background(), testHash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subscribe invoice failed")
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
