package tapd_test

import (
	"context"
	"testing"

	"github.com/ThorbenD/atomic-dvp-go/adapters/tapd"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStubSender_SendAsset_ReturnsTxID(t *testing.T) {
	s := tapd.NewStubSender()
	txID, err := s.SendAsset(context.Background(), "asset-1", decimal.NewFromFloat(1.5), "addr123")
	require.NoError(t, err)
	assert.NotEmpty(t, txID)
	assert.Contains(t, txID, "tx_sim_")
}

func TestStubSender_SendAsset_ContextCancel(t *testing.T) {
	s := tapd.NewStubSender()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := s.SendAsset(ctx, "asset-1", decimal.NewFromFloat(1.0), "addr123")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
