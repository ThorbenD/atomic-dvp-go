package tapd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lightninglabs/taproot-assets/taprpc/tapchannelrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

// ChannelClient wraps the tapd gRPC channel service
// for off-chain Taproot Asset transfers via Lightning channels.
type ChannelClient struct {
	conn   *grpc.ClientConn
	client tapchannelrpc.TaprootAssetChannelsClient
}

// NewChannelClient creates a new channel client for tapd.
func NewChannelClient(cfg Config) (*ChannelClient, error) {
	// Load TLS cert
	creds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS cert: %w", err)
	}

	// Load Macaroon
	macBytes, err := os.ReadFile(cfg.MacaroonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read macaroon: %w", err)
	}
	mac := &macaroon.Macaroon{}
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal macaroon: %w", err)
	}

	macCreds := NewMacaroonCredential(mac)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(macCreds),
	}

	conn, err := grpc.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tapd: %w", err)
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

// SendAssetViaChannel initiates an off-chain asset transfer
// through an existing Lightning channel using HTLC semantics.
// Settlement time: milliseconds (vs ~10 minutes on-chain).
func (c *ChannelClient) SendAssetViaChannel(
	ctx context.Context,
	req ChannelSendRequest,
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

	// Create the request
	sendReq := &tapchannelrpc.SendPaymentRequest{
		AssetId:     assetIDBytes,
		AssetAmount: req.Amount,
		PeerPubkey:  peerPubkeyBytes,
		PaymentRequest: &routerrpc.SendPaymentRequest{
			Dest:           peerPubkeyBytes,
			PaymentHash:    paymentHashBytes,
			PaymentAddr:    req.PaymentAddr,
			TimeoutSeconds: 60, // Reasonable timeout
			FeeLimitMsat:   10000,
		},
	}

	stream, err := c.client.SendPayment(ctx, sendReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate SendPayment stream: %w", err)
	}

	// Consume the stream to wait for settlement
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
			// lnrpc.Payment_SUCCEEDED = 2
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
			// We received an RFQ sell order acceptance.
			// The payment will proceed, so we continue listening to the stream.
			continue
		}
	}

	return nil, errors.New("stream closed without a terminal payment status")
}
