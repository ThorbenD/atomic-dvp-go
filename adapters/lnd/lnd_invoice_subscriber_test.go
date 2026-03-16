package lnd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLndInvoiceSubscriber_AcceptedInvoice_CallsHandler(t *testing.T) {
	mockClient := &MockLightningClient{}
	handler := newMockDepositHandler()

	updateCh := make(chan *settlement.InvoiceUpdate, 2)
	errCh := make(chan error, 1)

	mockClient.On("SubscribeInvoices", mock.Anything).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := adapterlnd.NewLndInvoiceSubscriber(mockClient, handler)
	sub.Start(ctx)

	time.Sleep(20 * time.Millisecond)
	updateCh <- &settlement.InvoiceUpdate{Hash: "abc123", State: "ACCEPTED", Amt: 50_000}

	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDepositDetected")
	}

	assert.Equal(t, 1, handler.callCount())
}

func TestLndInvoiceSubscriber_NonAcceptedState_IgnoredByHandler(t *testing.T) {
	mockClient := &MockLightningClient{}
	handler := newMockDepositHandler()

	updateCh := make(chan *settlement.InvoiceUpdate, 4)
	errCh := make(chan error, 1)

	mockClient.On("SubscribeInvoices", mock.Anything).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := adapterlnd.NewLndInvoiceSubscriber(mockClient, handler)
	sub.Start(ctx)

	time.Sleep(20 * time.Millisecond)

	// Send non-ACCEPTED states
	updateCh <- &settlement.InvoiceUpdate{Hash: "x1", State: "OPEN"}
	updateCh <- &settlement.InvoiceUpdate{Hash: "x2", State: "SETTLED"}
	updateCh <- &settlement.InvoiceUpdate{Hash: "x3", State: "CANCELED"}

	// Give subscriber time to process
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, handler.callCount())
}

func TestLndInvoiceSubscriber_ContextCancel_Stops(t *testing.T) {
	mockClient := &MockLightningClient{}
	handler := newMockDepositHandler()

	updateCh := make(chan *settlement.InvoiceUpdate)
	errCh := make(chan error)

	mockClient.On("SubscribeInvoices", mock.Anything).
		Return((<-chan *settlement.InvoiceUpdate)(updateCh), (<-chan error)(errCh), nil)

	ctx, cancel := context.WithCancel(context.Background())

	sub := adapterlnd.NewLndInvoiceSubscriber(mockClient, handler)
	sub.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	// After cancel, no additional SubscribeInvoices calls
	mockClient.AssertNumberOfCalls(t, "SubscribeInvoices", 1)
}

func TestLndInvoiceSubscriber_StreamError_Reconnects(t *testing.T) {
	mockClient := &MockLightningClient{}
	handler := newMockDepositHandler()

	firstUpdateCh := make(chan *settlement.InvoiceUpdate)
	firstErrCh := make(chan error, 1)

	secondUpdateCh := make(chan *settlement.InvoiceUpdate)
	secondErrCh := make(chan error)
	reconnected := make(chan struct{}, 1)

	mockClient.On("SubscribeInvoices", mock.Anything).
		Return((<-chan *settlement.InvoiceUpdate)(firstUpdateCh), (<-chan error)(firstErrCh), nil).
		Once()

	mockClient.On("SubscribeInvoices", mock.Anything).
		Run(func(args mock.Arguments) {
			reconnected <- struct{}{}
		}).
		Return((<-chan *settlement.InvoiceUpdate)(secondUpdateCh), (<-chan error)(secondErrCh), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := adapterlnd.NewLndInvoiceSubscriber(mockClient, handler)
	sub.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	// Trigger error on first stream
	firstErrCh <- fmt.Errorf("stream disconnected")

	// Wait for reconnect (1s backoff + margin)
	select {
	case <-reconnected:
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("subscriber did not reconnect within 2.5s")
	}

	mockClient.AssertNumberOfCalls(t, "SubscribeInvoices", 2)
}
