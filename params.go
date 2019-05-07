package main

import (
	"fmt"
	"strings"
)

// Params is used to parameterize the deployment, set from custom properties in the manifest
type Params struct {
	// control params
	DryRun bool `json:"dryrun,omitempty"`

	// app params
	App     string `json:"app,omitempty"`
	Runtime string `json:"runtime,omitempty"`
	Memory  string `json:"memory,omitempty"`
}

// SetDefaults fills in empty fields with convention-based defaults
func (p *Params) SetDefaults(gitName, appLabel, buildVersion, releaseName, releaseAction string, estafetteLabels map[string]string) {

	// default app to estafette app label if no override in stage params
	if p.App == "" && appLabel == "" && gitName != "" {
		p.App = gitName
	}
	if p.App == "" && appLabel != "" {
		p.App = appLabel
	}

	// default memory to 256MB
	if p.Memory == "" {
		p.Memory = "256MB"
	}
}

// ValidateRequiredProperties checks whether all needed properties are set
func (p *Params) ValidateRequiredProperties() (bool, []error, []string) {

	errors := []error{}
	warnings := []string{}

	supportedRuntimes := []string{
		"nodejs8",
		"nodejs10",
		"python37",
		"go111",
	}

	if !inStringArray(p.Runtime, supportedRuntimes) {
		errors = append(errors, fmt.Errorf("Runtime %v is not supported; set it to %v", p.Runtime, strings.Join(supportedRuntimes, ", ")))
	}

	supportedMemory := []string{
		"128MB",
		"256MB",
		"512MB",
		"1024MB",
		"2048MB",
	}

	if !inStringArray(p.Memory, supportedMemory) {
		errors = append(errors, fmt.Errorf("Memory %v is not supported; set it to %v", p.Memory, strings.Join(supportedMemory, ", ")))
	}

	return len(errors) == 0, errors, warnings
}

func inStringArray(value string, array []string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
}
