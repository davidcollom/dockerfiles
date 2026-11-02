package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Provider owns source-specific public file discovery and request pacing.
// Downloads run separately in the bounded concurrent transfer pool. New providers
// can use this seam for a future resolver queue without sharing authentication.
type Provider interface {
	Name() string
	Discover(ctx context.Context, models []Model, options ProviderOptions) (ProviderResult, error)
}

type ProviderOptions struct {
	Output   string
	MaxFiles int // Zero means unlimited across all registered providers.
}

type ProviderResult struct {
	Files       []DownloadFile `json:"-"` // Expiring or credential-bearing URLs must not be persisted.
	Existing    int            `json:"existing"`
	Restricted  int            `json:"restricted"`
	Unsupported int            `json:"unsupported"`
	Limited     int            `json:"limited"`
	Failed      int            `json:"failed"`
}

// PrintablesProvider supports the observed public interfaces for free files.
type PrintablesProvider struct{}

func (PrintablesProvider) Name() string { return "printables" }
func (PrintablesProvider) Discover(ctx context.Context, models []Model, options ProviderOptions) (ProviderResult, error) {
	result, err := DiscoverPrintables(ctx, models, PrintablesOptions{Output: options.Output, MaxFiles: options.MaxFiles})
	return ProviderResult{Files: result.Files, Existing: result.Existing, Restricted: result.Restricted, Unsupported: result.Unsupported, Limited: result.Limited, Failed: result.Failed}, err
}

func defaultProviders() []Provider { return []Provider{PrintablesProvider{}} }

func uniqueProviderModels(models []Model) []Model {
	result := make([]Model, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		key := model.Website + "\x00" + model.ExternalID
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, model)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Website != result[j].Website {
			return result[i].Website < result[j].Website
		}
		return result[i].ExternalID < result[j].ExternalID
	})
	return result
}

// ProviderSupportCounts counts unique references without network or file writes.
func ProviderSupportCounts(models []Model) (supported, unsupported int) {
	names := make(map[string]bool)
	for _, provider := range defaultProviders() {
		names[provider.Name()] = true
	}
	for _, model := range uniqueProviderModels(models) {
		if names[model.Website] {
			supported++
		} else {
			unsupported++
		}
	}
	return supported, unsupported
}

// DiscoverProviders groups references by exact source name and reserves a global
// file budget deterministically. Provider discovery stays sequential so a future
// provider cannot spend another source's budget or bypass its request pacing.
func DiscoverProviders(ctx context.Context, models []Model, options ProviderOptions) (ProviderResult, error) {
	return discoverWithProviders(ctx, models, options, defaultProviders())
}

func discoverWithProviders(ctx context.Context, models []Model, options ProviderOptions, providers []Provider) (ProviderResult, error) {
	var result ProviderResult
	if options.MaxFiles < 0 {
		return result, errors.New("max-files must not be negative")
	}
	registry := make(map[string]Provider, len(providers))
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			return result, errors.New("nil discovery provider")
		}
		name := provider.Name()
		if !validComponent(name) {
			return result, errors.New("invalid discovery provider name")
		}
		if _, exists := registry[name]; exists {
			return result, errors.New("duplicate discovery provider name")
		}
		registry[name] = provider
		names = append(names, name)
	}
	sort.Strings(names)
	groups := make(map[string][]Model, len(registry))
	for _, model := range uniqueProviderModels(models) {
		if _, ok := registry[model.Website]; !ok {
			result.Unsupported++
			continue
		}
		groups[model.Website] = append(groups[model.Website], model)
	}
	var failures []error
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return result, errors.Join(append(failures, err)...)
		}
		group := groups[name]
		if len(group) == 0 {
			continue
		}
		remaining := options.MaxFiles
		if remaining > 0 {
			remaining -= len(result.Files)
			if remaining <= 0 {
				result.Limited += len(group)
				continue
			}
		}
		partial, err := registry[name].Discover(ctx, group, ProviderOptions{Output: options.Output, MaxFiles: remaining})
		if options.MaxFiles > 0 && len(partial.Files) > remaining {
			partial.Limited += len(partial.Files) - remaining
			partial.Files = partial.Files[:remaining]
			err = errors.Join(err, errors.New("provider exceeded requested file budget"))
		}
		result.Files = append(result.Files, partial.Files...)
		result.Existing += partial.Existing
		result.Restricted += partial.Restricted
		result.Unsupported += partial.Unsupported
		result.Limited += partial.Limited
		result.Failed += partial.Failed
		if err != nil {
			failures = append(failures, fmt.Errorf("%s discovery: %w", name, err))
		}
	}
	if err := ctx.Err(); err != nil {
		failures = append(failures, err)
	}
	return result, errors.Join(failures...)
}
