package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeDiscoveryProvider struct {
	name     string
	discover func(context.Context, []Model, ProviderOptions) (ProviderResult, error)
}

func (p fakeDiscoveryProvider) Name() string { return p.name }
func (p fakeDiscoveryProvider) Discover(ctx context.Context, models []Model, opts ProviderOptions) (ProviderResult, error) {
	return p.discover(ctx, models, opts)
}
func providerModel(website, id string) Model {
	return Model{Reference: Reference{Website: website, ExternalID: id}}
}

func TestProviderSupportCounts(t *testing.T) {
	supported, unsupported := ProviderSupportCounts([]Model{providerModel("printables", "1"), providerModel("printables", "1"), providerModel("unknown", "2"), providerModel("unknown", "2"), providerModel("Printables", "3")})
	if supported != 1 || unsupported != 2 {
		t.Fatalf("support counts %d %d", supported, unsupported)
	}
}
func TestProvidersGroupAndDeduplicate(t *testing.T) {
	calls := 0
	provider := fakeDiscoveryProvider{name: "printables", discover: func(ctx context.Context, models []Model, opts ProviderOptions) (ProviderResult, error) {
		calls++
		if len(models) != 2 || models[0].ExternalID != "1" || models[1].ExternalID != "2" {
			t.Fatalf("unexpected provider refs %+v", models)
		}
		if opts.Output != "/library" || opts.MaxFiles != 0 {
			t.Fatalf("unexpected options %+v", opts)
		}
		return ProviderResult{Existing: 1, Restricted: 1}, nil
	}}
	result, err := discoverWithProviders(context.Background(), []Model{providerModel("unknown", "1"), providerModel("printables", "2"), providerModel("printables", "1"), providerModel("printables", "1"), providerModel("unknown", "1")}, ProviderOptions{Output: "/library"}, []Provider{provider})
	if err != nil || calls != 1 || result.Unsupported != 1 || result.Existing != 1 || result.Restricted != 1 {
		t.Fatalf("result %+v error %v calls %d", result, err, calls)
	}
}
func TestProvidersGlobalBudget(t *testing.T) {
	var called []string
	a := fakeDiscoveryProvider{name: "alpha", discover: func(ctx context.Context, models []Model, opts ProviderOptions) (ProviderResult, error) {
		called = append(called, "alpha")
		if opts.MaxFiles != 3 {
			t.Fatalf("alpha budget %d", opts.MaxFiles)
		}
		return ProviderResult{Files: []DownloadFile{{Name: "a"}, {Name: "b"}}}, nil
	}}
	b := fakeDiscoveryProvider{name: "beta", discover: func(ctx context.Context, models []Model, opts ProviderOptions) (ProviderResult, error) {
		called = append(called, "beta")
		if opts.MaxFiles != 1 {
			t.Fatalf("beta budget %d", opts.MaxFiles)
		}
		return ProviderResult{Files: []DownloadFile{{Name: "c"}}}, nil
	}}
	c := fakeDiscoveryProvider{name: "gamma", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		t.Fatal("exhausted provider called")
		return ProviderResult{}, nil
	}}
	result, err := discoverWithProviders(context.Background(), []Model{providerModel("alpha", "1"), providerModel("beta", "1"), providerModel("gamma", "1")}, ProviderOptions{MaxFiles: 3}, []Provider{c, b, a})
	if err != nil || len(result.Files) != 3 || result.Limited != 1 || strings.Join(called, ",") != "alpha,beta" {
		t.Fatalf("budget result %+v %v calls %v", result, err, called)
	}
}
func TestProvidersPartialErrorsAndCancellation(t *testing.T) {
	sourceErr := errors.New("source failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := fakeDiscoveryProvider{name: "alpha", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		cancel()
		return ProviderResult{Files: []DownloadFile{{Name: "partial"}}, Failed: 1}, sourceErr
	}}
	next := fakeDiscoveryProvider{name: "beta", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		t.Fatal("called provider after cancellation")
		return ProviderResult{}, nil
	}}
	result, err := discoverWithProviders(ctx, []Model{providerModel("alpha", "1"), providerModel("beta", "2")}, ProviderOptions{}, []Provider{next, provider})
	if len(result.Files) != 1 || result.Failed != 1 || !errors.Is(err, sourceErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("partial cancellation result %+v %v", result, err)
	}
}
func TestProvidersEnforceBudgetAndRedactURLs(t *testing.T) {
	provider := fakeDiscoveryProvider{name: "alpha", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		return ProviderResult{Files: []DownloadFile{{URL: "https://example.com/?secret=token"}, {URL: "https://example.com/2"}}}, nil
	}}
	result, err := discoverWithProviders(context.Background(), []Model{providerModel("alpha", "1")}, ProviderOptions{MaxFiles: 1}, []Provider{provider})
	if err == nil || len(result.Files) != 1 || result.Limited != 1 {
		t.Fatalf("provider overspend accepted %+v %v", result, err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "example.com") || strings.Contains(string(data), "token") {
		t.Fatal("discovery report persisted file URL")
	}
}
func TestProvidersContinueAfterFailure(t *testing.T) {
	sourceErr := errors.New("source unavailable")
	first := fakeDiscoveryProvider{name: "alpha", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		return ProviderResult{Failed: 1}, sourceErr
	}}
	next := fakeDiscoveryProvider{name: "beta", discover: func(context.Context, []Model, ProviderOptions) (ProviderResult, error) {
		return ProviderResult{Files: []DownloadFile{{Name: "success"}}}, nil
	}}
	result, err := discoverWithProviders(context.Background(), []Model{providerModel("alpha", "1"), providerModel("beta", "2")}, ProviderOptions{}, []Provider{next, first})
	if !errors.Is(err, sourceErr) || len(result.Files) != 1 || result.Failed != 1 {
		t.Fatalf("didn't preserve partial success: %+v %v", result, err)
	}
}
