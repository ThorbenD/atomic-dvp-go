package tapd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/lightninglabs/taproot-assets/taprpc/tapchannelrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"google.golang.org/grpc"
)

// ChannelSender is the interface for sending assets via Lightning channels.
// Implemented by ChannelClient; can be mocked for testing.
type ChannelSender interface {
	SendAssetViaChannel(ctx context.Context, req ChannelSendRequest) (*ChannelSendResult, error)
}

// tapChannelClient is the minimal gRPC interface needed by ChannelClient.
// Using a narrower interface (ISP) makes the client testable without depending
// on the full tapchannelrpc.TaprootAssetChannelsClient.
type tapChannelClient interface {
	SendPayment(ctx context.Context, in *tapchannelrpc.SendPaymentRequest, opts ...grpc.CallOption) (tapchannelrpc.TaprootAssetChannels_SendPaymentClient, error)
}

// ChannelClient wraps the tapd gRPC channel service
// for off-chain Taproot Asset transfers via Lightning channels.
type ChannelClient struct {
	conn   *grpc.ClientConn
	client tapChannelClient
}

// NewChannelClient creates a new channel client for tapd.
func NewChannelClient(cfg Config) (*ChannelClient, error) {
	conn, err := newGRPCConn(cfg.Host, cfg.TLSCertPath, cfg.MacaroonPath)
	if err != nil {
		return nil, err
	}

	return &ChannelClient{
		conn:   conn,
		client: tapchannelrpc.NewTaprootAssetChannelsClient(conn),
	}, nil
}

// Close closes the connection.
func (c *ChannelClient) Close() error {
	return c.conn.Close()
}

// ChannelSendRequest contains parameters for sending an asset via channel.
type ChannelSendRequest struct {
	AssetID     string
	Amount      uint64
	PeerPubkey  string
	PaymentHash string
	PaymentAddr []byte // Optional: for MPP
}

// ChannelSendResult contains the result of an off-chain asset transfer.
type ChannelSendResult struct {
	Preimage string
}

// ChannelDefaults holds configurable routing parameters for channel payments.
type ChannelDefaults struct {
	TimeoutSeconds int32
	FeeLimitMsat   int64
}

// DefaultChannelDefaults returns sensible defaults for channel payments.
func DefaultChannelDefaults() ChannelDefaults {
	return ChannelDefaults{
		TimeoutSeconds: 60,
		FeeLimitMsat:   10_000,
	}
}

// SendAssetViaChannel initiates an off-chain asset transfer
// through an existing Lightning channel using HTLC semantics.
// Settlement time: milliseconds (vs ~10 minutes on-chain).
func (c *ChannelClient) SendAssetViaChannel(
	ctx context.Context,
	req ChannelSendRequest,
) (*ChannelSendResult, error) {
	return c.sendAssetViaChannel(ctx, req, DefaultChannelDefaults())
}

// SendAssetViaChannelWithDefaults allows callers to override routing defaults.
func (c *ChannelClient) SendAssetViaChannelWithDefaults(
	ctx context.Context,
	req ChannelSendRequest,
	defaults ChannelDefaults,
) (*ChannelSendResult, error) {
	return c.sendAssetViaChannel(ctx, req, defaults)
}

func (c *ChannelClient) sendAssetViaChannel(
	ctx context.Context,
	req ChannelSendRequest,
	defaults ChannelDefaults,
) (*ChannelSendResult, error) {
	assetIDBytes, err := hex.DecodeString(req.AssetID)
	if err != nil {
		return nil, fmt.Errorf("invalid asset ID: %w", err)
	}

	peerPubkeyBytes, err := hex.DecodeString(req.PeerPubkey)
	if err != nil {
		return nil, fmt.Errorf("invalid peer pubkey: %w", err)
	}

	paymentHashBytes, err := hex.DecodeString(req.PaymentHash)
	if err != nil {
		return nil, fmt.Errorf("invalid payment hash: %w", err)
	}

	sendReq := &tapchannelrpc.SendPaymentRequest{
		AssetId:     assetIDBytes,
		AssetAmount: req.Amount,
		PeerPubkey:  peerPubkeyBytes,
		PaymentRequest: &routerrpc.SendPaymentRequest{
			Dest:           peerPubkeyBytes,
			PaymentHash:    paymentHashBytes,
			PaymentAddr:    req.PaymentAddr,
			TimeoutSeconds: defaults.TimeoutSeconds,
			FeeLimitMsat:   defaults.FeeLimitMsat,
		},
	}

	stream, err := c.client.SendPayment(ctx, sendReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate SendPayment stream: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}

		if resp.GetPaymentResult() != nil {
			paymentResult := resp.GetPaymentResult()

			switch paymentResult.Status {
			case lnrpc.Payment_SUCCEEDED:
				return &ChannelSendResult{
					Preimage: paymentResult.PaymentPreimage,
				}, nil
			case lnrpc.Payment_FAILED:
				return nil, fmt.Errorf("payment failed: %s", paymentResult.FailureReason)
			}
			// Status IN_FLIGHT is ignored, keep waiting
		}

		if resp.GetAcceptedSellOrder() != nil {
			continue
		}
	}

	return nil, errors.New("stream closed without a terminal payment status")
}
