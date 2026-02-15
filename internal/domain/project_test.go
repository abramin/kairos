package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateShortID_Valid(t *testing.T) {
	cases := []string{"PHI01", "MATH02", "ABC1234", "ABCDEF01", "XYZ99"}
	for _, id := range cases {
		p := &Project{ShortID: id}
		assert.NoError(t, p.ValidateShortID(), "should accept %q", id)
	}
}

func TestValidateShortID_Empty(t *testing.T) {
	p := &Project{ShortID: ""}
	err := p.ValidateShortID()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestValidateShortID_Lowercase(t *testing.T) {
	p := &Project{ShortID: "phi01"}
	err := p.ValidateShortID()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uppercase")
}

func TestValidateShortID_TooShort(t *testing.T) {
	p := &Project{ShortID: "AB1"}
	err := p.ValidateShortID()
	require.Error(t, err)
}

func TestValidateShortID_NoDigits(t *testing.T) {
	p := &Project{ShortID: "PHYSICS"}
	err := p.ValidateShortID()
	require.Error(t, err)
}

func TestDisplayID_WithShortID(t *testing.T) {
	p := &Project{ID: "550e8400-e29b-41d4-a716-446655440000", ShortID: "PHI01"}
	assert.Equal(t, "PHI01", p.DisplayID())
}

func TestDisplayID_WithoutShortID(t *testing.T) {
	p := &Project{ID: "550e8400-e29b-41d4-a716-446655440000", ShortID: ""}
	assert.Equal(t, "550e8400", p.DisplayID())
}

func TestDisplayID_ShortUUID(t *testing.T) {
	p := &Project{ID: "abc", ShortID: ""}
	assert.Equal(t, "abc", p.DisplayID())
}
