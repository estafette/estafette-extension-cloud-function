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
	App                  string                 `json:"app,omitempty"`
	Runtime              string                 `json:"runtime,omitempty"`
	Trigger              string                 `json:"trigger,omitempty"`
	TriggerValue         string                 `json:"triggerValue,omitempty"`
	Memory               string                 `json:"memory,omitempty"`
	ServiceAccount       string                 `json:serviceAccount,omitempty"`
	Source               string                 `json:"source,omitempty"`
	IngressSettings      string                 `json:"ingressSettings,omitempty"`
	TimeoutSeconds       int                    `json:"timeout,omitempty"`
	EnvironmentVariables map[string]interface{} `json:"env,omitempty"`
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

	// default trigger to http-trigger
	if p.Trigger == "" {
		p.Trigger = "http"
	}

	// default memory to 256MB
	if p.Memory == "" {
		p.Memory = "256MB"
	}

	// default source to current directory
	if p.Source == "" {
		p.Source = "."
	}

	// default timeout to 60 seconds
	if p.TimeoutSeconds <= 0 {
		p.TimeoutSeconds = 60
	}

	// default ingress-settings to all
	if p.IngressSettings == "" {
		p.IngressSettings = "all"
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

	supportedTrigger := []string{
		"http",
		"bucket",
	}

	if !inStringArray(p.Trigger, supportedTrigger) {
		errors = append(errors, fmt.Errorf("Trigger %v is not supported; set it to %v", p.Trigger, strings.Join(supportedTrigger, ", ")))
	}

	if p.Trigger == "bucket" && p.TriggerValue == "" {
		errors = append(errors, fmt.Errorf("TriggerValue is required when Trigger is bucket; set TriggerValue as well"))
	}

	if p.TimeoutSeconds <= 0 || p.TimeoutSeconds > 540 {
		errors = append(errors, fmt.Errorf("Timeout %v is not supported; set it between 0 and 540 seconds", p.Memory))
	}

	supportedIngressSettings := []string{
		"all",
		"internal-only",
	}

	if !inStringArray(p.IngressSettings, supportedIngressSettings) {
		errors = append(errors, fmt.Errorf("IngressSettings %v is not supported; set it to %v", p.IngressSettings, strings.Join(supportedIngressSettings, ", ")))
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
