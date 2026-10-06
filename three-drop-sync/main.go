// three-drop-sync archives 3Drop likes and collection references and downloads
// explicitly supplied model links into a local library.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type archiveOptions struct {
	output, input, cookieFile string
	interval                  time.Duration
	dryRun                    bool
	afterArchive              func(context.Context, Catalogue) error
}

func runArchive(ctx context.Context, opts archiveOptions) error {
	for {
		var snapshot Snapshot
		var err error
		if opts.input != "" {
			snapshot, err = ReadImport(opts.input)
		} else {
			// Re-read the mounted secret on each run to allow session renewal.
			var cookie string
			cookie, err = ReadCookie(opts.cookieFile)
			if err == nil {
				snapshot, err = NewClient(cookie).Fetch(ctx)
			}
		}
		if err == nil {
			var catalogue Catalogue
			catalogue, err = snapshot.Catalogue()
			if err == nil {
				if opts.dryRun {
					slog.Info("validated archive input", "models", len(catalogue.Models), "collections", len(catalogue.Collections))
				} else {
					var changed bool
					changed, err = SaveArchive(opts.output, snapshot)
					if err == nil {
						slog.Info("archive complete", "changed", changed, "models", len(catalogue.Models), "collections", len(catalogue.Collections))
					}
				}
				if err == nil && opts.afterArchive != nil {
					err = opts.afterArchive(ctx, catalogue)
				}
			}
		}
		if err != nil {
			if opts.interval == 0 {
				return err
			}
			slog.Error("sync run failed; retrying at configured interval", "error", err)
		}
		if opts.interval == 0 {
			return nil
		}
		if err := wait(ctx, opts.interval); err != nil {
			return err
		}
	}
}

func newCommand() *cobra.Command {
	cfg := viper.New()
	cfg.SetEnvPrefix("THREE_DROP")
	cfg.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	cfg.AutomaticEnv()
	var configFile string
	root := &cobra.Command{
		Use: "three-drop-sync", Short: "Back up 3Drop references and supported model files",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("choose a command: sync, import or download")
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if configFile != "" {
				cfg.SetConfigFile(configFile)
				if err := cfg.ReadInConfig(); err != nil {
					return fmt.Errorf("read configuration: %w", err)
				}
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&configFile, "config", "", "optional YAML, JSON or TOML configuration file")
	root.PersistentFlags().String("output", "/library", "archive directory (mount your NAS here)")
	root.PersistentFlags().String("interval", "0s", "repeat interval; zero runs once, otherwise at least 1m")
	root.PersistentFlags().Bool("dry-run", false, "validate inputs without writing files")
	root.PersistentFlags().Int("workers", 4, "maximum concurrent downloads (1..128)")
	root.PersistentFlags().Int("max-files", 0, "maximum new files per run; zero is unlimited")
	root.PersistentFlags().String("max-file-size", "1GiB", "maximum size per file in bytes or KiB/MiB/GiB")
	root.PersistentFlags().Int("retries", 2, "number of retries for temporary download failures (0..10)")
	for _, name := range []string{"output", "interval", "dry-run", "workers", "max-files", "max-file-size", "retries"} {
		_ = cfg.BindPFlag(name, root.PersistentFlags().Lookup(name))
	}

	for _, source := range []string{"sync", "import"} {
		source := source
		cmd := &cobra.Command{Use: source, Args: cobra.NoArgs}
		if source == "sync" {
			cmd.Short = "Archive liked models and collections using a local session cookie file"
			cmd.Flags().String("cookie-file", "", "local secret file containing the 3Drop Cookie header value")
			_ = cfg.BindPFlag("cookie-file", cmd.Flags().Lookup("cookie-file"))
			cmd.Flags().Bool("download", false, "download supported free model files after archiving references")
			_ = cfg.BindPFlag("download", cmd.Flags().Lookup("download"))
		} else {
			cmd.Short = "Archive saved likes.json and collections.json responses"
			cmd.Flags().String("input", "", "directory containing likes.json and collections.json")
			_ = cfg.BindPFlag("input", cmd.Flags().Lookup("input"))
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			opts, err := archiveConfig(cfg)
			if err != nil {
				return err
			}
			if source == "import" {
				opts.input = cfg.GetString("input")
				if opts.input == "" {
					return fmt.Errorf("import requires --input or THREE_DROP_INPUT")
				}
			} else {
				opts.cookieFile = cfg.GetString("cookie-file")
				if opts.cookieFile == "" {
					return fmt.Errorf("sync requires --cookie-file or THREE_DROP_COOKIE_FILE")
				}
			}
			if source == "sync" {
				enabled, err := configBool(cfg, "download")
				if err != nil {
					return err
				}
				if enabled {
					transfer, err := downloadConfig(cfg, opts.output)
					if err != nil {
						return err
					}
					opts.afterArchive = func(ctx context.Context, catalogue Catalogue) error {
						return downloadLibrary(ctx, catalogue, transfer, opts.dryRun)
					}
				}
			}
			return runArchive(cmd.Context(), opts)
		}
		root.AddCommand(cmd)
	}
	download := &cobra.Command{Use: "download", Short: "Download supported model files or authorised direct links", Args: cobra.NoArgs}
	download.Flags().String("manifest", "", "JSON array of explicit model download links")
	download.Flags().Bool("from-library", false, "discover supported free files from the current archived catalogue")
	for _, name := range []string{"manifest", "from-library"} {
		_ = cfg.BindPFlag(name, download.Flags().Lookup(name))
	}
	download.RunE = func(cmd *cobra.Command, args []string) error {
		common, err := archiveConfig(cfg)
		if err != nil {
			return err
		}
		if common.interval != 0 {
			return fmt.Errorf("download runs once; --interval must be zero")
		}
		manifest := cfg.GetString("manifest")
		fromLibrary, err := configBool(cfg, "from-library")
		if err != nil {
			return err
		}
		if (manifest == "") == !fromLibrary {
			return fmt.Errorf("download requires exactly one of --manifest or --from-library")
		}
		opts, err := downloadConfig(cfg, common.output)
		if err != nil {
			return err
		}
		if fromLibrary {
			catalogue, err := LoadCurrentCatalogue(common.output)
			if err != nil {
				return err
			}
			return downloadLibrary(cmd.Context(), catalogue, opts, common.dryRun)
		}
		files, err := ReadDownloadManifest(manifest)
		if err != nil {
			return err
		}
		opts.Files = files
		if common.dryRun {
			if err := ValidateDownloadManifest(files); err != nil {
				return err
			}
			slog.Info("validated download manifest", "files", len(files), "workers", opts.Concurrency, "max_files", opts.MaxFiles)
			return nil
		}
		summary, err := Download(cmd.Context(), opts)
		slog.Info("download complete", "downloaded", summary.Downloaded, "skipped", summary.Skipped, "limited", summary.Limited, "failed", summary.Failed, "bytes", summary.Bytes)
		return err
	}
	root.AddCommand(download)
	return root
}

func downloadConfig(cfg *viper.Viper, output string) (DownloadOptions, error) {
	opts := DownloadOptions{Output: output}
	var err error
	opts.Concurrency, err = configInt(cfg, "workers", 1)
	if err != nil {
		return opts, err
	}
	if opts.Concurrency > 128 {
		return opts, fmt.Errorf("workers must be <= 128")
	}
	opts.MaxFiles, err = configInt(cfg, "max-files", 0)
	if err != nil {
		return opts, err
	}
	opts.Retries, err = configInt(cfg, "retries", 0)
	if err != nil {
		return opts, err
	}
	if opts.Retries > 10 {
		return opts, fmt.Errorf("retries must be <= 10")
	}
	opts.MaxFileSize, err = parseFileSize(cfg.GetString("max-file-size"))
	return opts, err
}

func configBool(cfg *viper.Viper, key string) (bool, error) {
	value, err := strconv.ParseBool(cfg.GetString(key))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return value, nil
}

func downloadLibrary(ctx context.Context, catalogue Catalogue, opts DownloadOptions, dryRun bool) error {
	supported, unsupported := ProviderSupportCounts(catalogue.Models)
	sites := make(map[string]bool)
	sourceProviderCounts := make(map[string]int)
	for _, model := range catalogue.Models {
		count, _ := ProviderSupportCounts([]Model{model})
		if count > 0 {
			sourceProviderCounts[model.Website]++
		} else {
			sites[model.Website] = true
		}
	}
	unsupportedSites := make([]string, 0, len(sites))
	for site := range sites {
		unsupportedSites = append(unsupportedSites, site)
	}
	sort.Strings(unsupportedSites)
	if unsupported > 0 {
		slog.Warn("model sites lack automatic file download adapters", "models", unsupported, "sites", unsupportedSites)
	}
	if dryRun {
		slog.Info("validated library download plan; no provider requests made", "supported_references", supported, "source_provider_counts", sourceProviderCounts, "unsupported_references", unsupported, "max_files", opts.MaxFiles)
		return nil
	}
	discovered, discoveryErr := DiscoverProviders(ctx, catalogue.Models, ProviderOptions{MaxFiles: opts.MaxFiles, Output: opts.Output})
	slog.Info("model provider file discovery complete", "source_provider_counts", sourceProviderCounts, "files", len(discovered.Files), "existing", discovered.Existing, "restricted", discovered.Restricted, "unsupported", discovered.Unsupported, "limited", discovered.Limited, "failed", discovered.Failed)
	if len(discovered.Files) == 0 {
		return discoveryErr
	}
	opts.Files = discovered.Files
	summary, err := Download(ctx, opts)
	slog.Info("supported model file download complete", "downloaded", summary.Downloaded, "skipped", summary.Skipped, "limited", summary.Limited, "failed", summary.Failed, "bytes", summary.Bytes)
	return errors.Join(discoveryErr, err)
}

func archiveConfig(cfg *viper.Viper) (archiveOptions, error) {
	opts := archiveOptions{output: cfg.GetString("output")}
	if strings.TrimSpace(opts.output) == "" {
		return opts, fmt.Errorf("output must not be empty")
	}
	var err error
	opts.interval, err = time.ParseDuration(cfg.GetString("interval"))
	if err != nil || opts.interval < 0 || (opts.interval > 0 && opts.interval < time.Minute) {
		return opts, fmt.Errorf("interval must be zero or at least one minute")
	}
	opts.dryRun, err = strconv.ParseBool(cfg.GetString("dry-run"))
	if err != nil {
		return opts, fmt.Errorf("dry-run must be a boolean")
	}
	return opts, nil
}

func configInt(cfg *viper.Viper, key string, minimum int) (int, error) {
	value, err := strconv.Atoi(cfg.GetString(key))
	if err != nil || value < minimum {
		return 0, fmt.Errorf("%s must be an integer >= %d", key, minimum)
	}
	return value, nil
}

func parseFileSize(input string) (int64, error) {
	value := strings.TrimSpace(input)
	multiplier := int64(1)
	for _, unit := range []struct {
		name       string
		multiplier int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if strings.HasSuffix(value, unit.name) {
			value = strings.TrimSuffix(value, unit.name)
			multiplier = unit.multiplier
			break
		}
	}
	size, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || size < 1 || size > (1<<63-2)/multiplier {
		return 0, fmt.Errorf("max-file-size must be positive bytes or an integer with KiB/MiB/GiB suffix")
	}
	return size * multiplier, nil
}

func run(ctx context.Context, args []string) error {
	cmd := newCommand()
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil && ctx.Err() == nil {
		slog.Error("operation failed", "error", err)
		os.Exit(1)
	}
}
