package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type handoffOptions struct {
	archiveOptions
	maxAge time.Duration
	load   func(context.Context) (Handoff, Snapshot, error)
}

// runHandoff retries at the configured interval, including interrupted transfers.
// Every pass reloads the manifest and credentials so mounted secrets can rotate.
func runHandoff(ctx context.Context, opts handoffOptions) error {
	for {
		h, snapshot, err := opts.load(ctx)
		if err == nil {
			err = checkHandoffAge(h, opts.maxAge, time.Now())
		}
		if err == nil {
			var catalogue Catalogue
			catalogue, err = snapshot.Catalogue()
			if err == nil && !opts.dryRun {
				err = saveHandoff(opts.output, h, snapshot)
			}
			if err == nil {
				slog.Info("complete handoff validated", "models", len(catalogue.Models), "collections", len(catalogue.Collections), "dry_run", opts.dryRun)
				if opts.afterArchive != nil {
					err = opts.afterArchive(ctx, catalogue)
				}
			}
		}
		if opts.interval == 0 {
			return err
		}
		if err != nil {
			slog.Error("handoff run failed; retrying at configured interval", "error", err)
		}
		if err := wait(ctx, opts.interval); err != nil {
			return err
		}
	}
}

func checkHandoffAge(h Handoff, maxAge time.Duration, now time.Time) error {
	captured, err := time.Parse(time.RFC3339, h.CapturedAt)
	if err != nil {
		return fmt.Errorf("invalid handoff timestamp")
	}
	if captured.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("handoff timestamp is in the future")
	}
	if maxAge > 0 && now.Sub(captured) > maxAge {
		return fmt.Errorf("handoff is stale; publish a fresh complete inventory")
	}
	return nil
}

// Preserve titles and source URLs separately from the existing reference archive.
// An older handoff cannot move the current catalogue backwards.
func saveHandoff(root string, h Handoff, snapshot Snapshot) error {
	if err := os.MkdirAll(root, 0750); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".handoff.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("handoff archive already in use")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	previous, err := os.ReadFile(filepath.Join(root, "latest-handoff.json"))
	if err == nil {
		var old struct {
			CapturedAt string `json:"capturedAt"`
		}
		if json.Unmarshal(previous, &old) != nil {
			return fmt.Errorf("invalid previous handoff pointer")
		}
		prior, err := time.Parse(time.RFC3339, old.CapturedAt)
		if err != nil {
			return fmt.Errorf("invalid previous handoff timestamp")
		}
		current, _ := time.Parse(time.RFC3339, h.CapturedAt)
		if current.Before(prior) {
			return fmt.Errorf("handoff predates the current archive")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	name := hex.EncodeToString(digest[:]) + ".json"
	directory := filepath.Join(root, "handoffs")
	if err := os.MkdirAll(directory, 0750); err != nil {
		return err
	}
	if err := writeAtomic(directory, name, data); err != nil {
		return err
	}
	if _, err := SaveArchive(root, snapshot); err != nil {
		return err
	}
	pointer, _ := json.Marshal(struct {
		CapturedAt string `json:"capturedAt"`
		SHA256     string `json:"sha256"`
	}{h.CapturedAt, hex.EncodeToString(digest[:])})
	return writeAtomic(root, "latest-handoff.json", pointer)
}

func addHandoffCommands(root *cobra.Command, cfg *viper.Viper) {
	drive := &cobra.Command{Use: "drive", Short: "Publish or consume a private Google Drive reference inventory"}
	drive.PersistentFlags().String("drive-file-id", "", "Drive file ID to read or update; publication creates a file if omitted")
	drive.PersistentFlags().String("drive-token-file", "", "private file containing a short-lived Google OAuth access token")
	drive.PersistentFlags().String("drive-credentials-file", "", "private authorised_user OAuth JSON with a renewable refresh token")
	for _, name := range []string{"drive-file-id", "drive-token-file", "drive-credentials-file"} {
		_ = cfg.BindPFlag(name, drive.PersistentFlags().Lookup(name))
	}
	makeSync := func(use string, remote bool) *cobra.Command {
		cmd := &cobra.Command{Use: use, Short: "Validate, archive and optionally download a complete JSON inventory", Args: cobra.NoArgs}
		if !remote {
			cmd.Flags().String("handoff-file", "", "complete browser-exported JSON inventory file")
		}
		cmd.Flags().String("max-manifest-age", "168h", "maximum age of inventory; zero disables age limit")
		cmd.Flags().Bool("download", false, "resolve and download supported provider files")
		// Viper bindings belong to the executing command; sibling flags must not overwrite them.
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			for _, name := range []string{"handoff-file", "max-manifest-age", "download"} {
				if flag := cmd.Flags().Lookup(name); flag != nil {
					_ = cfg.BindPFlag(name, flag)
				}
			}
			return nil
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			common, err := archiveConfig(cfg)
			if err != nil {
				return err
			}
			age, err := time.ParseDuration(cfg.GetString("max-manifest-age"))
			if err != nil || age < 0 {
				return fmt.Errorf("max-manifest-age must be a non-negative duration")
			}
			opts := handoffOptions{archiveOptions: common, maxAge: age}
			if remote {
				settings := driveSettings(cfg)
				if settings.FileID == "" {
					return fmt.Errorf("drive sync requires --drive-file-id")
				}
				if err := validateDriveConfig(settings); err != nil {
					return err
				}
				opts.load = func(ctx context.Context) (Handoff, Snapshot, error) {
					data, err := FetchDriveManifest(ctx, settings)
					if err != nil {
						return Handoff{}, Snapshot{}, err
					}
					return ParseHandoff(data)
				}
			} else {
				path := cfg.GetString("handoff-file")
				if path == "" {
					return fmt.Errorf("handoff requires --handoff-file")
				}
				opts.load = func(context.Context) (Handoff, Snapshot, error) { return ReadHandoff(path) }
			}
			enabled, err := configBool(cfg, "download")
			if err != nil {
				return err
			}
			if enabled {
				transfer, err := downloadConfig(cfg, common.output)
				if err != nil {
					return err
				}
				opts.afterArchive = func(ctx context.Context, c Catalogue) error { return downloadLibrary(ctx, c, transfer, common.dryRun) }
			}
			return runHandoff(cmd.Context(), opts)
		}
		return cmd
	}
	drive.AddCommand(makeSync("sync", true))
	root.AddCommand(makeSync("handoff", false))
	publish := &cobra.Command{Use: "publish", Short: "Create or update a Drive file with a complete validated inventory", Args: cobra.NoArgs}
	publish.Flags().String("handoff-file", "", "complete browser-exported JSON inventory file")
	publish.Flags().String("drive-folder-id", "", "parent Drive folder ID for a new file")
	publish.Flags().String("drive-name", "three-drop-catalogue.json", "name for a newly created Drive file")
	publish.RunE = func(cmd *cobra.Command, args []string) error {
		for _, name := range []string{"handoff-file", "drive-folder-id", "drive-name"} {
			_ = cfg.BindPFlag(name, cmd.Flags().Lookup(name))
		}
		common, err := archiveConfig(cfg)
		if err != nil {
			return err
		}
		if common.interval != 0 {
			return fmt.Errorf("drive publish runs once; interval must be zero")
		}
		path := cfg.GetString("handoff-file")
		if path == "" {
			return fmt.Errorf("drive publish requires --handoff-file")
		}
		h, _, err := ReadHandoff(path)
		if err != nil {
			return err
		}
		if err := checkHandoffAge(h, 168*time.Hour, time.Now()); err != nil {
			return err
		}
		data, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return err
		}
		if common.dryRun {
			slog.Info("complete Drive publication validated; no upload performed", "models", len(h.Models))
			return nil
		}
		settings := driveSettings(cfg)
		settings.FolderID = cfg.GetString("drive-folder-id")
		settings.Name = cfg.GetString("drive-name")
		id, err := PublishDriveManifest(cmd.Context(), settings, data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), id)
		return err
	}
	drive.AddCommand(publish)
	root.AddCommand(drive)
}

func driveSettings(cfg *viper.Viper) DriveConfig {
	return DriveConfig{FileID: cfg.GetString("drive-file-id"), TokenFile: cfg.GetString("drive-token-file"), CredentialsFile: cfg.GetString("drive-credentials-file")}
}
