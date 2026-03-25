package tapd

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

// newGRPCConn creates an authenticated gRPC connection using TLS and a macaroon file.
// This helper is shared by all tapd clients (Client, ChannelClient) to avoid duplication.
func newGRPCConn(host, tlsCertPath, macaroonPath string) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS cert: %w", err)
	}

	macBytes, err := os.ReadFile(macaroonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read macaroon: %w", err)
	}

	mac := &macaroon.Macaroon{}
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal macaroon: %w", err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(NewMacaroonCredential(mac)),
	}

	conn, err := grpc.NewClient(host, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", host, err)
	}

	return conn, nil
}

// MacaroonCredential implements grpc.PerRPCCredentials for macaroon-based authentication.
type MacaroonCredential struct {
	Macaroon *macaroon.Macaroon
}

// NewMacaroonCredential creates a new gRPC credential from a macaroon.
func NewMacaroonCredential(mac *macaroon.Macaroon) *MacaroonCredential {
	return &MacaroonCredential{Macaroon: mac}
}

func (m *MacaroonCredential) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	macBytes, err := m.Macaroon.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"macaroon": hex.EncodeToString(macBytes),
	}, nil
}

func (m *MacaroonCredential) RequireTransportSecurity() bool {
	return true
}
