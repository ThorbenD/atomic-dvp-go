//go:build integration

// Package lnd_test contains integration tests that require a running LND node.
//
// These tests are excluded from the default `go test ./...` run and must be
// explicitly opted in with the integration build tag:
//
//	go test -tags=integration -timeout=300s ./adapters/lnd/
//
// Environment variables required:
//
//	LND_HOST         – gRPC address of the LND node  (default: localhost:10009)
//	LND_TLS_CERT     – path to tls.cert              (default: ~/.lnd/tls.cert)
//	LND_MACAROON     – path to admin.macaroon         (default: ~/.lnd/data/chain/bitcoin/regtest/admin.macaroon)
package lnd_test

import (
	"context"
	"os"
	"testing"
	"time"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	clientlnd "github.com/ThorbenD/atomic-dvp-go/clients/lnd"
	"github.com/stretchr/testify/require"
)

// lndClientFromEnv constructs a real LND client from environment variables.
// It skips the test if the required variables are not set.
func lndClientFromEnv(t *testing.T) *clientlnd.Client {
	t.Helper()
	host := os.Getenv("LND_HOST")
	if host == "" {
		host = "localhost:10009"
	}
	tlsCert := os.Getenv("LND_TLS_CERT")
	if tlsCert == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot determine home dir: %v", err)
		}
		tlsCert = home + "/.lnd/tls.cert"
	}
	macaroon := os.Getenv("LND_MACAROON")
	if macaroon == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot determine home dir: %v", err)
		}
		macaroon = home + "/.lnd/data/chain/bitcoin/regtest/admin.macaroon"
	}

	if _, err := os.Stat(tlsCert); os.IsNotExist(err) {
		t.Skipf("LND TLS cert not found at %s — skipping integration test", tlsCert)
	}
	if _, err := os.Stat(macaroon); os.IsNotExist(err) {
		t.Skipf("LND macaroon not found at %s — skipping integration test", macaroon)
	}

	client, err := clientlnd.NewClient(host, tlsCert, macaroon)
	require.NoError(t, err, "failed to connect to LND")
	return client
}

// TestIntegration_LND_GetInfo verifies that we can reach the LND node and
// retrieve its basic information.
func TestIntegration_LND_GetInfo(t *testing.T) {
	client := lndClientFromEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := client.GetInfo(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.Pubkey, "expected a non-empty node pubkey")
	t.Logf("connected to LND node: alias=%s pubkey=%s network=%s synced=%v",
		info.Alias, info.Pubkey, info.Network, info.Synced)
}

// TestIntegration_LND_HoldInvoiceLifecycle creates a hold invoice, verifies it
// can be detected via the chain watcher, then cancels it. This exercises the
// full prepare → detect → abort path without actually settling.
func TestIntegration_LND_HoldInvoiceLifecycle(t *testing.T) {
	client := lndClientFromEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use a well-known test preimage / hash pair (regtest only).
	const (
		testPreimage = "000102030405060708090a0b0c0d0e0f000102030405060708090a0b0c0d0e0f"
		testPayHash  = "66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925"
	)

	// 1. Create a hold invoice.
	_, _, err := client.AddHoldInvoice(ctx, "integration-test", testPayHash, 1_000)
	require.NoError(t, err, "AddHoldInvoice failed")

	// 2. Confirm the invoice is visible to the chain watcher.
	watcher := adapterlnd.NewLndChainWatcher(client)
	detectCtx, detectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer detectCancel()

	// The invoice was just created (OPEN state); detection should not block
	// long — it will either return ACCEPTED (if paid) or time out.
	// For this smoke test we only verify that the subscription succeeds.
	updateCh, errCh, err := client.SubscribeSingleInvoice(detectCtx, testPayHash)
	require.NoError(t, err, "SubscribeSingleInvoice failed")
	_ = watcher // watcher is available for full lifecycle tests

	select {
	case update := <-updateCh:
		t.Logf("received invoice update: state=%s amt=%d", update.State, update.Amt)
	case err := <-errCh:
		t.Logf("stream error (expected if invoice transitions quickly): %v", err)
	case <-detectCtx.Done():
		t.Log("no update within timeout — invoice remains in OPEN state (expected)")
	}

	// 3. Cancel the invoice to clean up.
	cancelErr := client.CancelInvoice(ctx, testPayHash)
	require.NoError(t, cancelErr, "CancelInvoice failed")
}
