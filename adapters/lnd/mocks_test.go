package lnd_test

import (
	"context"
	"sync"

	"github.com/ThorbenD/atomic-dvp-go/clients/tapd"
	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/stretchr/testify/mock"
)

// MockLightningClient implements settlement.LightningClient for tests.
type MockLightningClient struct {
	mock.Mock
}

func (m *MockLightningClient) GetInfo(ctx context.Context) (*settlement.NodeInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*settlement.NodeInfo), args.Error(1)
}

func (m *MockLightningClient) AddHoldInvoice(ctx context.Context, memo, hash string, val uint64) (string, uint64, error) {
	args := m.Called(ctx, memo, hash, val)
	return args.String(0), uint64(args.Int(1)), args.Error(2)
}

func (m *MockLightningClient) SettleInvoice(ctx context.Context, preimage string) error {
	args := m.Called(ctx, preimage)
	return args.Error(0)
}

func (m *MockLightningClient) CancelInvoice(ctx context.Context, hash string) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockLightningClient) StartInterceptor(ctx context.Context, handler func(settlement.HtlcPacket) settlement.HtlcResolution) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}

func (m *MockLightningClient) SubscribeSingleInvoice(ctx context.Context, hash string) (<-chan *settlement.InvoiceUpdate, <-chan error, error) {
	args := m.Called(ctx, hash)
	if args.Error(2) != nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(<-chan *settlement.InvoiceUpdate), args.Get(1).(<-chan error), nil
}

func (m *MockLightningClient) SubscribeInvoices(ctx context.Context) (<-chan *settlement.InvoiceUpdate, <-chan error, error) {
	args := m.Called(ctx)
	if args.Error(2) != nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(<-chan *settlement.InvoiceUpdate), args.Get(1).(<-chan error), nil
}

func (m *MockLightningClient) FundPsbt(ctx context.Context, outputs map[string]uint64) (string, int32, error) {
	args := m.Called(ctx, outputs)
	return args.String(0), int32(args.Int(1)), args.Error(2)
}

func (m *MockLightningClient) SignPsbt(ctx context.Context, packet string) (string, bool, error) {
	args := m.Called(ctx, packet)
	return args.String(0), args.Bool(1), args.Error(2)
}

func (m *MockLightningClient) PublishTransaction(ctx context.Context, txHex string) error {
	args := m.Called(ctx, txHex)
	return args.Error(0)
}

func (m *MockLightningClient) WaitForConfirmations(ctx context.Context, txid string, numConfs uint32) error {
	args := m.Called(ctx, txid, numConfs)
	return args.Error(0)
}

// MockChainWatcher implements settlement.ChainWatcher for tests.
type MockChainWatcher struct {
	mock.Mock
}

func (m *MockChainWatcher) DetectHTLC(ctx context.Context, hash string) (*domain.HTLC, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.HTLC), args.Error(1)
}

func (m *MockChainWatcher) ClaimHTLC(ctx context.Context, preimage string) (string, error) {
	args := m.Called(ctx, preimage)
	return args.String(0), args.Error(1)
}

// MockChannelSender implements tapd.ChannelSender for tests.
type MockChannelSender struct {
	mock.Mock
}

func (m *MockChannelSender) SendAssetViaChannel(ctx context.Context, req tapd.ChannelSendRequest) (*tapd.ChannelSendResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tapd.ChannelSendResult), args.Error(1)
}

// mockDepositHandler implements lnd.DepositHandler for invoice subscriber tests.
type mockDepositHandler struct {
	mu    sync.Mutex
	calls []string
	done  chan struct{}
}

func newMockDepositHandler() *mockDepositHandler {
	return &mockDepositHandler{done: make(chan struct{}, 10)}
}

func (h *mockDepositHandler) OnDepositDetected(_ context.Context, hash string) error {
	h.mu.Lock()
	h.calls = append(h.calls, hash)
	h.mu.Unlock()
	h.done <- struct{}{}
	return nil
}

func (h *mockDepositHandler) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.calls)
}
