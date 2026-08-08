package validate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/validate"
)

func TestIsGreaterThan(t *testing.T) {
	t.Parallel()

	ok, _ := validate.IsGreaterThan(5)(6)
	assert.True(t, ok)

	ok, msg := validate.IsGreaterThan(5)(5)
	assert.False(t, ok)
	assert.Equal(t, "must be greater than 5", msg)
}

func TestIsGreaterThanOrEqual(t *testing.T) {
	t.Parallel()

	ok, _ := validate.IsGreaterThanOrEqual(5)(5)
	assert.True(t, ok)

	ok, msg := validate.IsGreaterThanOrEqual(5)(4)
	assert.False(t, ok)
	assert.Equal(t, "must be greater than or equal to 5", msg)
}

func TestIsLesserThan(t *testing.T) {
	t.Parallel()

	ok, _ := validate.IsLesserThan(5)(4)
	assert.True(t, ok)

	ok, msg := validate.IsLesserThan(5)(5)
	assert.False(t, ok)
	assert.Equal(t, "must be lesser than 5", msg)
}

func TestIsLesserThanOrEqual(t *testing.T) {
	t.Parallel()

	ok, _ := validate.IsLesserThanOrEqual(5)(5)
	assert.True(t, ok)

	ok, msg := validate.IsLesserThanOrEqual(5)(6)
	assert.False(t, ok)
	assert.Equal(t, "must be lesser than or equal to 5", msg)
}

func TestIsValidTimeZone(t *testing.T) {
	t.Parallel()

	ok, _ := validate.IsValidTimeZone("Europe/Brussels")
	assert.True(t, ok)

	ok, msg := validate.IsValidTimeZone("not-a-timezone")
	assert.False(t, ok)
	assert.Equal(t, "must be a valid IANA value", msg)
}

func TestCheckOptional(t *testing.T) {
	t.Parallel()

	v := validate.New()
	validate.CheckOptional(v, "field", nil, validate.IsNotEmpty)
	assert.True(t, v.Valid())

	empty := ""
	validate.CheckOptional(v, "field", &empty, validate.IsNotEmpty)
	assert.False(t, v.Valid())
	assert.Equal(t, "must be provided", v.Errors()["field"])
}
