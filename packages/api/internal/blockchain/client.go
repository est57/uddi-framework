package blockchain

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

var ErrDIDNotFound = errors.New("did not found")

type Client struct {
	rpc   string
	store DIDStore
}

type RegisterDIDParams struct {
	DID       string
	PublicKey []byte
}

type DIDDocument struct {
	Context         []string `json:"@context"`
	ID              string   `json:"id"`
	PublicKeyBase64 string   `json:"publicKeyBase64"`
	Created         string   `json:"created"`
	Updated         string   `json:"updated"`
	Deactivated     bool     `json:"deactivated"`
}

func NewClient(rpc string) (*Client, error) {
	return NewClientWithStore(rpc, NewMemoryDIDStore()), nil
}

func NewClientWithStore(rpc string, store DIDStore) *Client {
	return &Client{
		rpc:   rpc,
		store: store,
	}
}

func (c *Client) DIDExists(ctx context.Context, did string) (bool, error) {
	return c.store.Exists(ctx, did)
}

func (c *Client) RegisterDID(ctx context.Context, params RegisterDIDParams) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	doc := DIDDocument{
		Context:         []string{"https://www.w3.org/ns/did/v1", "https://uddi.network/v1"},
		ID:              params.DID,
		PublicKeyBase64: base64.StdEncoding.EncodeToString(params.PublicKey),
		Created:         now,
		Updated:         now,
		Deactivated:     false,
	}

	if err := c.store.Create(ctx, doc); err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(c.rpc + ":" + params.DID + ":" + now))
	return "0x" + hex.EncodeToString(hash[:]), nil
}

func (c *Client) ResolveDID(ctx context.Context, did string) (*DIDDocument, error) {
	return c.store.Resolve(ctx, did)
}

func (c *Client) RevokeDID(ctx context.Context, did string) error {
	doc, err := c.store.Resolve(ctx, did)
	if err != nil {
		return err
	}
	doc.Deactivated = true
	return c.store.Update(ctx, *doc)
}

func (c *Client) RegistryStats(ctx context.Context) (RegistryStats, error) {
	stats, err := c.store.Stats(ctx)
	if err != nil {
		return RegistryStats{}, err
	}
	stats.Backend = registryBackend(c.rpc, stats.Backend)
	return stats, nil
}

func registryBackend(rpc string, fallback string) string {
	switch {
	case rpc == "":
		return fallback
	case rpc == "memory://local", rpc == "memory://test":
		return fallback
	case fallback != "":
		return fallback
	default:
		return rpc
	}
}
