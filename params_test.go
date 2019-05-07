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
	t.Run("DefaultsAppToGitNameIfAppParamIsEmptyAndAppLabelIsEmpty", func(t *testing.T) {

		params := Params{
			App: "",
		}
		gitName := "mygitrepo"
		appLabel := ""

		// act
		params.SetDefaults(gitName, appLabel, "", "", "", map[string]string{})

		assert.Equal(t, "mygitrepo", params.App)
	})

	t.Run("DefaultsAppToAppLabelIfEmpty", func(t *testing.T) {

		params := Params{
			App: "",
		}
		appLabel := "myapp"

		// act
		params.SetDefaults("", appLabel, "", "", "", map[string]string{})

		assert.Equal(t, "myapp", params.App)
	})

	t.Run("KeepsAppIfNotEmpty", func(t *testing.T) {

		params := Params{
			App: "yourapp",
		}
		appLabel := "myapp"

		// act
		params.SetDefaults("", appLabel, "", "", "", map[string]string{})

		assert.Equal(t, "yourapp", params.App)
	})
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
