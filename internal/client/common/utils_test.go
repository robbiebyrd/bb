package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPadOrTrim(t *testing.T) {
	// pad: input shorter than half of size
	assert.Equal(t, []byte{1, 2, 0, 0}, PadOrTrim([]byte{1, 2}, 4), "Should pad with zeros on the right.")

	// pad: input longer than half of size (bug case)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 0, 0, 0}, PadOrTrim([]byte{1, 2, 3, 4, 5}, 8), "Should pad with zeros on the right.")

	// trim: input longer than size
	assert.Equal(t, []byte{1, 2, 3}, PadOrTrim([]byte{1, 2, 3, 4, 5}, 3), "Should trim to size.")

	// exact: input equals size
	assert.Equal(t, []byte{1, 2, 3}, PadOrTrim([]byte{1, 2, 3}, 3), "Should return unchanged when exact fit.")
}

func TestArrayContainsTrue(t *testing.T) {
	allTrue := ArrayContainsTrue([]bool{true, true, true})
	assert.Equal(t, true, allTrue, "Should be true.")

	allFalse := ArrayContainsTrue([]bool{false, false, false})
	assert.Equal(t, false, allFalse, "Should be false.")

	oneTrue := ArrayContainsTrue([]bool{true, false, false})
	assert.Equal(t, true, oneTrue, "Should be true.")
}

func TestArrayContainsFalse(t *testing.T) {
	oneFalse := ArrayContainsFalse([]bool{true, true, false})
	assert.Equal(t, true, oneFalse, "Should be true.")

	allFalseAndTrue := ArrayContainsFalse([]bool{false, false, false})
	assert.Equal(t, true, allFalseAndTrue, "Should be true.")

	allTrueAndFalse := ArrayContainsFalse([]bool{true, true, true})
	assert.Equal(t, false, allTrueAndFalse, "Should be true.")
}

func TestArrayAllTrue(t *testing.T) {
	allTrue := ArrayAllTrue([]bool{true, true, true})
	assert.Equal(t, true, allTrue, "Should be true.")

	oneFalse := ArrayAllTrue([]bool{true, true, false})
	assert.Equal(t, false, oneFalse, "Should be true.")
}

func TestArrayAllFalse(t *testing.T) {
	allFalse := ArrayAllFalse([]bool{false, false, false})
	assert.Equal(t, true, allFalse, "Should be true.")

	oneTrue := ArrayAllFalse([]bool{false, true, false})
	assert.Equal(t, false, oneTrue, "Should be true.")
}
