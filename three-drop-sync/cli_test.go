package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cliImportFixture(t *testing.T) string {
	t.Helper()
	input := t.TempDir()
	for name, content := range map[string]json.RawMessage{"likes.json": fixture().Likes, "collections.json": fixture().Collections} {
		if err := os.WriteFile(filepath.Join(input, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return input
}

func TestCLIConfigPrecedence(t *testing.T) {
	input := cliImportFixture(t)
	for _, test := range []struct {
		name              string
		environment, flag bool
	}{
		{name: "file"}, {name: "environment", environment: true}, {name: "flag", environment: true, flag: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			fileOutput, envOutput, flagOutput := filepath.Join(dir, "file"), filepath.Join(dir, "env"), filepath.Join(dir, "flag")
			configPath := filepath.Join(dir, "config.json")
			config, _ := json.Marshal(map[string]any{"input": input, "output": fileOutput, "dry-run": false, "interval": "0s"})
			if err := os.WriteFile(configPath, config, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"import", "--config", configPath}
			expected := fileOutput
			if test.environment {
				t.Setenv("THREE_DROP_OUTPUT", envOutput)
				expected = envOutput
			}
			if test.flag {
				args = append(args, "--output", flagOutput)
				expected = flagOutput
			}
			if err := run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			for _, output := range []string{fileOutput, envOutput, flagOutput} {
				_, err := os.Stat(filepath.Join(output, "latest.json"))
				if output == expected && err != nil {
					t.Fatalf("expected %s: %v", output, err)
				}
				if output != expected && !os.IsNotExist(err) {
					t.Fatalf("unexpected output %s: %v", output, err)
				}
			}
		})
	}
}

func TestCLIRejectsInvalidConfiguration(t *testing.T) {
	input := cliImportFixture(t)
	for _, test := range []struct {
		key     string
		value   any
		message string
	}{
		{"interval", "garbage", "interval"}, {"interval", "59s", "interval"}, {"interval", "-1m", "interval"},
		{"interval", false, "interval"}, {"dry-run", "sometimes", "dry-run"}, {"output", "", "output"},
	} {
		t.Run(test.key+":"+test.message, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.json")
			config, _ := json.Marshal(map[string]any{"input": input, "output": t.TempDir(), test.key: test.value})
			if err := os.WriteFile(configPath, config, 0600); err != nil {
				t.Fatal(err)
			}
			err := run(context.Background(), []string{"import", "--config", configPath})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %s error, got %v", test.message, err)
			}
		})
	}
}

func TestCLIDownloadRejectsInvalidLimits(t *testing.T) {
	for _, args := range [][]string{
		{"--workers", "0"}, {"--max-files", "-1"}, {"--max-file-size", "0"}, {"--max-file-size", "9223372036854775807GiB"}, {"--retries", "-1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			full := append([]string{"download", "--manifest", "unused.json"}, args...)
			err := run(context.Background(), full)
			if err == nil || !strings.Contains(err.Error(), strings.TrimPrefix(args[0], "--")) {
				t.Fatalf("incorrect validation: %v", err)
			}
		})
	}
}

func TestCLIDryRunUsesEnvironmentWithoutWriting(t *testing.T) {
	output := filepath.Join(t.TempDir(), "missing")
	t.Setenv("THREE_DROP_INPUT", cliImportFixture(t))
	t.Setenv("THREE_DROP_OUTPUT", output)
	t.Setenv("THREE_DROP_DRY_RUN", "true")
	if err := run(context.Background(), []string{"import"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote files: %v", err)
	}
}

func TestCLISourceRequirements(t *testing.T) {
	for _, args := range [][]string{nil, {"sync"}, {"import"}, {"download"}, {"import", "unexpected"}} {
		if err := run(context.Background(), args); err == nil {
			t.Fatalf("accepted incomplete command: %v", args)
		}
	}
}

func TestCLIFromLibraryDryRun(t *testing.T) {
	output := t.TempDir()
	if _, err := SaveArchive(output, fixture()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(output, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"download", "--from-library", "--dry-run", "--output", output}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(output, "latest.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("dry-run changed archive")
	}
	if _, err := os.Stat(filepath.Join(output, "models")); !os.IsNotExist(err) {
		t.Fatalf("dry run created model files: %v", err)
	}
}

func TestCLIDownloadSourceExclusive(t *testing.T) {
	err := run(context.Background(), []string{"download", "--from-library", "--manifest", "unused.json", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("accepted both download sources: %v", err)
	}
}

func TestCLIEnforcesDownloaderBoundsInDryRun(t *testing.T) {
	for _, args := range [][]string{{"--workers", "129"}, {"--retries", "11"}, {"--max-file-size", "9223372036854775807"}} {
		full := append([]string{"download", "--from-library", "--dry-run"}, args...)
		err := run(context.Background(), full)
		if err == nil || !strings.Contains(err.Error(), strings.TrimPrefix(args[0], "--")) {
			t.Fatalf("incorrect validation: %v", err)
		}
	}
}

func TestCLISyncDownloadConfigValidation(t *testing.T) {
	err := run(context.Background(), []string{"sync", "--cookie-file", "unused", "--download", "--dry-run", "--workers", "129"})
	if err == nil || !strings.Contains(err.Error(), "workers") {
		t.Fatalf("transfer settings not checked before auth: %v", err)
	}
}

func TestArchiveDryRunTransferCallback(t *testing.T) {
	called := 0
	output := filepath.Join(t.TempDir(), "archive")
	err := runArchive(context.Background(), archiveOptions{
		input: cliImportFixture(t), output: output, dryRun: true,
		afterArchive: func(ctx context.Context, catalogue Catalogue) error {
			called++
			if len(catalogue.Models) != 3 {
				t.Fatalf("wrong catalogue: %+v", catalogue)
			}
			return downloadLibrary(ctx, catalogue, DownloadOptions{Output: output}, true)
		},
	})
	if err != nil || called != 1 {
		t.Fatalf("callback not run: %d %v", called, err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote archive: %v", err)
	}
}

func TestCLILibraryDryRunReportsProviderCoverage(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	catalogue, err := fixture().Catalogue()
	if err != nil {
		t.Fatal(err)
	}
	if err := downloadLibrary(context.Background(), catalogue, DownloadOptions{Output: t.TempDir()}, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"source_provider_counts":{"printables":1}`, `"supported_references":1`, `"unsupported_references":2`, `"sites":["makerworld","thingiverse"]`, `no provider requests made`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing coverage %q in %s", want, output.String())
		}
	}
}
