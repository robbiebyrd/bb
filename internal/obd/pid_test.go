package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePIDRequest(t *testing.T) {
	got, err := NormalizePIDRequest("01 0c")
	require.NoError(t, err)
	assert.Equal(t, "010C", got)
}

func TestNormalizePIDRequest_Errors(t *testing.T) {
	_, err := NormalizePIDRequest("")
	assert.Error(t, err)
	_, err = NormalizePIDRequest("010")
	assert.Error(t, err)
	_, err = NormalizePIDRequest("01ZZ")
	assert.Error(t, err)
}

func TestNormalizePIDRequests_SkipsBlanks(t *testing.T) {
	got, err := NormalizePIDRequests([]string{"010C", "", "  ", "01 0d"})
	require.NoError(t, err)
	assert.Equal(t, []string{"010C", "010D"}, got)
}
