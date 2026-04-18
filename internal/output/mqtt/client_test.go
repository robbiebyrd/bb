package mqtt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// writeSelfSignedCACert writes a self-signed CA certificate in PEM format to a
// temporary file and returns the path. The caller is responsible for cleanup.
func writeSelfSignedCACert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	f, err := os.CreateTemp(t.TempDir(), "ca-*.pem")
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return f.Name()
}

func TestBuildTLSConfig_NoCACert_NilPool(t *testing.T) {
	cfg := canModels.MQTTConfig{
		TLS:           true,
		TLSCACertFile: "",
	}

	tlsCfg, err := buildTLSConfig(cfg)

	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.Nil(t, tlsCfg.RootCAs, "nil RootCAs means Go uses system roots")
	assert.False(t, tlsCfg.InsecureSkipVerify)
}

func TestBuildTLSConfig_ValidCACert_PopulatesPool(t *testing.T) {
	caPath := writeSelfSignedCACert(t)

	cfg := canModels.MQTTConfig{
		TLS:           true,
		TLSCACertFile: caPath,
	}

	tlsCfg, err := buildTLSConfig(cfg)

	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.RootCAs, "RootCAs must be populated when CA cert file is provided")
	assert.False(t, tlsCfg.InsecureSkipVerify)
}

func TestBuildTLSConfig_BadCACertPath_ReturnsError(t *testing.T) {
	cfg := canModels.MQTTConfig{
		TLS:           true,
		TLSCACertFile: filepath.Join(t.TempDir(), "nonexistent-ca.pem"),
	}

	tlsCfg, err := buildTLSConfig(cfg)

	assert.Error(t, err)
	assert.Nil(t, tlsCfg)
}

func TestBuildTLSConfig_InvalidPEM_ReturnsError(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bad-ca-*.pem")
	require.NoError(t, err)
	_, err = f.WriteString("this is not valid PEM data")
	require.NoError(t, err)
	f.Close()

	cfg := canModels.MQTTConfig{
		TLS:           true,
		TLSCACertFile: f.Name(),
	}

	tlsCfg, err := buildTLSConfig(cfg)

	assert.Error(t, err)
	assert.Nil(t, tlsCfg)
}

// stubFilter is a minimal FilterInterface implementation that never filters any message.
type stubFilter struct{}

func (s *stubFilter) Filter(_ canModels.CanMessageTimestamped) bool { return false }

// TestHandleCanMessageChannel_DrainAndReturn verifies the channel handler
// returns nil once the incoming channel is closed with no pending messages.
func TestHandleCanMessageChannel_DrainAndReturn(t *testing.T) {
	c := &MQTTClient{
		l:               slog.Default(),
		ctx:             context.Background(),
		incomingChannel: make(chan canModels.CanMessageTimestamped),
		filters:         make(map[string]canModels.FilterInterface),
		resolver:        &mockResolver{conns: map[int]*mockCanConn{}},
	}

	close(c.incomingChannel)

	err := c.HandleCanMessageChannel()
	assert.NoError(t, err)
}

// TestHandleSignalChannel_DrainAndReturn verifies the signal channel handler
// returns nil once the signal channel is closed with no pending signals.
func TestHandleSignalChannel_DrainAndReturn(t *testing.T) {
	c := &MQTTClient{
		l:             slog.Default(),
		ctx:           context.Background(),
		signalChannel: make(chan canModels.CanSignalTimestamped),
		filters:       make(map[string]canModels.FilterInterface),
		resolver:      &mockResolver{conns: map[int]*mockCanConn{}},
	}

	close(c.signalChannel)

	err := c.HandleSignalChannel()
	assert.NoError(t, err)
}

// TestAddFilter_HappyPath verifies a new filter is accepted and stored.
func TestAddFilter_HappyPath(t *testing.T) {
	c := &MQTTClient{
		l:       slog.Default(),
		filters: make(map[string]canModels.FilterInterface),
	}

	err := c.AddFilter("my-filter", &stubFilter{})
	require.NoError(t, err)
	assert.Contains(t, c.filters, "my-filter")
}

// TestAddFilter_DuplicateRejected verifies registering the same filter name twice returns an error.
func TestAddFilter_DuplicateRejected(t *testing.T) {
	c := &MQTTClient{
		l:       slog.Default(),
		filters: make(map[string]canModels.FilterInterface),
	}

	require.NoError(t, c.AddFilter("dup", &stubFilter{}))

	err := c.AddFilter("dup", &stubFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup")
}
