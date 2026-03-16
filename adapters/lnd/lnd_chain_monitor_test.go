package lnd_test

import (
	"context"
	"fmt"
	"testing"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLndChainMonitor_WaitForConfirmations_Success(t *testing.T) {
	mockClient := &MockLightningClient{}
	mockClient.On("WaitForConfirmations", mock.Anything, "deadbeeftx", uint32(6)).Return(nil)

	monitor := adapterlnd.NewLndChainMonitor(mockClient)
	err := monitor.WaitForConfirmations(context.Background(), "deadbeeftx", 6)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestLndChainMonitor_WaitForConfirmations_Error(t *testing.T) {
	mockClient := &MockLightningClient{}
	mockClient.On("WaitForConfirmations", mock.Anything, "deadbeeftx", uint32(3)).
		Return(fmt.Errorf("block not found"))

	monitor := adapterlnd.NewLndChainMonitor(mockClient)
	err := monitor.WaitForConfirmations(context.Background(), "deadbeeftx", 3)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wait for confirmations failed")
}
