package mqtt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
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
