package tapd

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/ThorbenD/atomic-dvp-go/domain"

	"github.com/lightninglabs/taproot-assets/taprpc"
	"github.com/lightninglabs/taproot-assets/taprpc/assetwalletrpc"
	"github.com/lightninglabs/taproot-assets/taprpc/mintrpc"
	"google.golang.org/grpc"
)

// Config holds connection configuration.
type Config struct {
	Host         string
	TLSCertPath  string
	MacaroonPath string
}

// Client implements settlement.AssetService.
type Client struct {
	conn              *grpc.ClientConn
	assetClient       taprpc.TaprootAssetsClient
	mintClient        mintrpc.MintClient
	assetWalletClient assetwalletrpc.AssetWalletClient
}

// New creates a new Tapd Client.
func New(cfg Config) (*Client, error) {
	conn, err := newGRPCConn(cfg.Host, cfg.TLSCertPath, cfg.MacaroonPath)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:              conn,
		assetClient:       taprpc.NewTaprootAssetsClient(conn),
		mintClient:        mintrpc.NewMintClient(conn),
		assetWalletClient: assetwalletrpc.NewAssetWalletClient(conn),
	}, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// MintAsset mints a new asset batch.
func (c *Client) MintAsset(ctx context.Context, name string, amount uint64) (*domain.AssetBalance, error) {
	req := &mintrpc.MintAssetRequest{
		Asset: &mintrpc.MintAsset{
			AssetType: taprpc.AssetType_NORMAL,
			Name:      name,
			AssetMeta: &taprpc.AssetMeta{
				Data: []byte("minted-by-atomic-dvp-go"),
			},
			Amount: amount,
		},
	}

	_, err := c.mintClient.MintAsset(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to mint asset: %w", err)
	}

	batchResp, err := c.mintClient.FinalizeBatch(ctx, &mintrpc.FinalizeBatchRequest{ShortResponse: true})
	if err != nil {
		return nil, fmt.Errorf("failed to finalize batch: %w", err)
	}

	return &domain.AssetBalance{
		Asset: domain.Asset{
			Name: name,
			Type: domain.AssetTypeTaproot,
			ID:   hex.EncodeToString(batchResp.Batch.BatchKey),
		},
		Amount: amount,
	}, nil
}

// ListAssets lists all assets.
func (c *Client) ListAssets(ctx context.Context) ([]*domain.AssetBalance, error) {
	resp, err := c.assetClient.ListAssets(ctx, &taprpc.ListAssetRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	var assets []*domain.AssetBalance
	for _, a := range resp.Assets {
		assets = append(assets, &domain.AssetBalance{
			Asset: domain.Asset{
				ID:   hex.EncodeToString(a.AssetGenesis.AssetId),
				Name: a.AssetGenesis.Name,
				Type: domain.AssetType(a.AssetGenesis.AssetType.String()),
			},
			Amount: a.Amount,
		})
	}
	return assets, nil
}

// SendAsset sends an asset to an address.
func (c *Client) SendAsset(ctx context.Context, addr string) (string, error) {
	resp, err := c.assetClient.SendAsset(ctx, &taprpc.SendAssetRequest{
		TapAddrs: []string{addr},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send asset: %v", err)
	}

	if resp.Transfer != nil {
		if len(resp.Transfer.AnchorTxHash) > 0 {
			return hex.EncodeToString(resp.Transfer.AnchorTxHash), nil
		}
		return fmt.Sprintf("pending-%d", resp.Transfer.TransferTimestamp), nil
	}

	return "unknown-transfer-id", nil
}

// NewAddress generates a new receive address for a specific asset.
func (c *Client) NewAddress(ctx context.Context, assetID string, amount uint64) (string, error) {
	assetIDBytes, err := hex.DecodeString(assetID)
	if err != nil {
		return "", fmt.Errorf("invalid asset ID hex: %w", err)
	}

	resp, err := c.assetClient.NewAddr(ctx, &taprpc.NewAddrRequest{
		AssetId: assetIDBytes,
		Amt:     amount,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate new address: %w", err)
	}

	return resp.Encoded, nil
}

// FundVirtualPsbt funds a virtual PSBT for an asset transfer.
func (c *Client) FundVirtualPsbt(ctx context.Context, assetID string, amount uint64, recipientAddr string) (string, uint32, error) {
	req := &assetwalletrpc.FundVirtualPsbtRequest{
		Template: &assetwalletrpc.FundVirtualPsbtRequest_Raw{
			Raw: &assetwalletrpc.TxTemplate{
				Recipients: map[string]uint64{
					recipientAddr: amount,
				},
			},
		},
	}

	resp, err := c.assetWalletClient.FundVirtualPsbt(ctx, req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fund virtual PSBT: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resp.FundedPsbt), uint32(resp.ChangeOutputIndex), nil
}

// SignVirtualPsbt signs a virtual PSBT.
func (c *Client) SignVirtualPsbt(ctx context.Context, fundedPsbt string) (string, error) {
	rawPsbt, err := base64.StdEncoding.DecodeString(fundedPsbt)
	if err != nil {
		return "", fmt.Errorf("failed to decode psbt: %w", err)
	}

	resp, err := c.assetWalletClient.SignVirtualPsbt(ctx, &assetwalletrpc.SignVirtualPsbtRequest{
		FundedPsbt: rawPsbt,
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign virtual PSBT: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resp.SignedPsbt), nil
}

// AnchorVirtualPsbt anchors a virtual PSBT to a BTC PSBT.
func (c *Client) AnchorVirtualPsbt(ctx context.Context, virtualPsbt string) (string, error) {
	rawPsbt, err := base64.StdEncoding.DecodeString(virtualPsbt)
	if err != nil {
		return "", fmt.Errorf("failed to decode psbt: %w", err)
	}

	resp, err := c.assetWalletClient.CommitVirtualPsbts(ctx, &assetwalletrpc.CommitVirtualPsbtsRequest{
		VirtualPsbts: [][]byte{rawPsbt},
		Fees: &assetwalletrpc.CommitVirtualPsbtsRequest_SatPerVbyte{
			SatPerVbyte: 2,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to anchor virtual PSBT: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resp.AnchorPsbt), nil
}
