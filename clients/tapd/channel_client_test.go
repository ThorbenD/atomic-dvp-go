package tapd

import (
	"context"
	"io"
	"testing"

	"github.com/lightninglabs/taproot-assets/taprpc/tapchannelrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockTapChannelClient captures the SendPaymentRequest for assertion.
type mockTapChannelClient struct {
	lastReq *tapchannelrpc.SendPaymentRequest
	stream  tapchannelrpc.TaprootAssetChannels_SendPaymentClient
	sendErr error
}

func (m *mockTapChannelClient) SendPayment(
	_ context.Context,
	in *tapchannelrpc.SendPaymentRequest,
	_ ...grpc.CallOption,
) (tapchannelrpc.TaprootAssetChannels_SendPaymentClient, error) {
	m.lastReq = in
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return m.stream, nil
}

// mockSendPaymentStream implements tapchannelrpc.TaprootAssetChannels_SendPaymentClient.
// It returns its pre-configured responses and then io.EOF.
type mockSendPaymentStream struct {
	responses []*tapchannelrpc.SendPaymentResponse
	pos       int
}

func (s *mockSendPaymentStream) Recv() (*tapchannelrpc.SendPaymentResponse, error) {
	if s.pos >= len(s.responses) {
		return nil, io.EOF
	}
	r := s.responses[s.pos]
	s.pos++
	return r, nil
}

// grpc.ClientStream no-op implementations.
func (s *mockSendPaymentStream) Header() (metadata.MD, error) { return nil, nil }
func (s *mockSendPaymentStream) Trailer() metadata.MD          { return nil }
func (s *mockSendPaymentStream) CloseSend() error              { return nil }
func (s *mockSendPaymentStream) Context() context.Context      { return context.Background() }
func (s *mockSendPaymentStream) SendMsg(_ interface{}) error   { return nil }
func (s *mockSendPaymentStream) RecvMsg(_ interface{}) error   { return nil }

// newTestChannelClient bypasses the gRPC connection and injects a mock client.
func newTestChannelClient(client tapChannelClient) *ChannelClient {
	return &ChannelClient{client: client}
}

func testChannelSendRequest() ChannelSendRequest {
	return ChannelSendRequest{
		AssetID:     "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		Amount:      1000,
		PeerPubkey:  "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
		PaymentHash: "b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000",
	}
}

// TestChannelClient_SendAssetViaChannel_UsesDefaultDefaults verifies that
// SendAssetViaChannel always applies DefaultChannelDefaults to the gRPC request.
func TestChannelClient_SendAssetViaChannel_UsesDefaultDefaults(t *testing.T) {
	mock := &mockTapChannelClient{
		stream: &mockSendPaymentStream{}, // returns EOF → "stream closed" error
	}
	c := newTestChannelClient(mock)

	_, err := c.SendAssetViaChannel(context.Background(), testChannelSendRequest())

	// A stream that closes immediately returns an expected sentinel error.
	require.Error(t, err)
	require.NotNil(t, mock.lastReq, "SendPayment must have been called")
	require.NotNil(t, mock.lastReq.PaymentRequest)
	assert.Equal(t, DefaultChannelDefaults().TimeoutSeconds, mock.lastReq.PaymentRequest.TimeoutSeconds)
	assert.Equal(t, DefaultChannelDefaults().FeeLimitMsat, mock.lastReq.PaymentRequest.FeeLimitMsat)
}

// TestChannelClient_SendAssetViaChannelWithDefaults_UsesCustomDefaults verifies
// that custom routing parameters are forwarded to the underlying gRPC request.
func TestChannelClient_SendAssetViaChannelWithDefaults_UsesCustomDefaults(t *testing.T) {
	mock := &mockTapChannelClient{
		stream: &mockSendPaymentStream{},
	}
	c := newTestChannelClient(mock)

	custom := ChannelDefaults{
		TimeoutSeconds: 120,
		FeeLimitMsat:   50_000,
	}

	_, err := c.SendAssetViaChannelWithDefaults(context.Background(), testChannelSendRequest(), custom)

	require.Error(t, err)
	require.NotNil(t, mock.lastReq, "SendPayment must have been called")
	require.NotNil(t, mock.lastReq.PaymentRequest)
	assert.Equal(t, int32(120), mock.lastReq.PaymentRequest.TimeoutSeconds)
	assert.Equal(t, int64(50_000), mock.lastReq.PaymentRequest.FeeLimitMsat)
}

// TestChannelClient_SendAssetViaChannel_CustomDefaultsDifferFromBuiltin verifies
// that custom defaults produce different routing parameters than the built-in defaults.
func TestChannelClient_SendAssetViaChannel_CustomDefaultsDifferFromBuiltin(t *testing.T) {
	defaultMock := &mockTapChannelClient{stream: &mockSendPaymentStream{}}
	customMock := &mockTapChannelClient{stream: &mockSendPaymentStream{}}

	cDefault := newTestChannelClient(defaultMock)
	cCustom := newTestChannelClient(customMock)

	custom := ChannelDefaults{TimeoutSeconds: 5, FeeLimitMsat: 1}

	_, _ = cDefault.SendAssetViaChannel(context.Background(), testChannelSendRequest())
	_, _ = cCustom.SendAssetViaChannelWithDefaults(context.Background(), testChannelSendRequest(), custom)

	require.NotNil(t, defaultMock.lastReq.PaymentRequest)
	require.NotNil(t, customMock.lastReq.PaymentRequest)
	assert.NotEqual(t,
		defaultMock.lastReq.PaymentRequest.TimeoutSeconds,
		customMock.lastReq.PaymentRequest.TimeoutSeconds,
	)
	assert.NotEqual(t,
		defaultMock.lastReq.PaymentRequest.FeeLimitMsat,
		customMock.lastReq.PaymentRequest.FeeLimitMsat,
	)
}

// TestChannelClient_SendAssetViaChannel_StreamInitError verifies that a
// SendPayment RPC error is surfaced with a descriptive message.
func TestChannelClient_SendAssetViaChannel_StreamInitError(t *testing.T) {
	mock := &mockTapChannelClient{sendErr: io.ErrUnexpectedEOF}
	c := newTestChannelClient(mock)

	_, err := c.SendAssetViaChannel(context.Background(), testChannelSendRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initiate SendPayment stream")
}
