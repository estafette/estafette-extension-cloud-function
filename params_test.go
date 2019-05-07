package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	trueValue   = true
	falseValue  = false
	validParams = Params{
		Runtime: "go111",
	}
	validCredential = GKECredentials{
		Name: "gke-production",
	}
)

func TestSetDefaults(t *testing.T) {

}

func TestValidateRequiredProperties(t *testing.T) {

	t.Run("ReturnsFalseIfRuntimeIsNotSupported", func(t *testing.T) {

		params := validParams
		params.Runtime = "nodejs6"

		// act
		valid, errors, _ := params.ValidateRequiredProperties()

		assert.False(t, valid)
		assert.True(t, len(errors) > 0)
	})

	t.Run("ReturnsTrueIfRuntimeIsSupported", func(t *testing.T) {

		params := validParams
		params.Runtime = "go111"

		// act
		valid, errors, _ := params.ValidateRequiredProperties()

		assert.True(t, valid)
		assert.True(t, len(errors) == 0)
	})
}
