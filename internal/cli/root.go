package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"git-pulse/internal/aggregator"
	"git-pulse/internal/config"
	"git-pulse/internal/dashboard"
	"git-pulse/internal/tui"
	exportpkg "git-pulse/pkg/export"
)

type options struct {
	RepoPath   string
	ConfigPath string
	Theme      string
	JSON       bool
	Markdown   bool
	CSV        bool
	CI         bool
}

func Execute() error {
	opts := options{}

	rootCmd := &cobra.Command{
		Use:           "git-pulse",
		Short:         "A terminal dashboard for git repository trends",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(context.Background(), opts)
		},
	}

	rootCmd.Flags().StringVar(&opts.RepoPath, "repo", ".", "Path to the repository to analyze")
	rootCmd.Flags().StringVar(&opts.ConfigPath, "config", "", "Path to a config file")
	rootCmd.Flags().StringVar(&opts.Theme, "theme", "", "Theme override")
	rootCmd.Flags().BoolVar(&opts.JSON, "json", false, "Print a JSON snapshot instead of launching the TUI")
	rootCmd.Flags().BoolVar(&opts.Markdown, "markdown", false, "Print a Markdown snapshot instead of launching the TUI")
	rootCmd.Flags().BoolVar(&opts.CSV, "csv", false, "Print a CSV summary instead of launching the TUI")
	rootCmd.Flags().BoolVar(&opts.CI, "ci", false, "Print a JSON snapshot for CI systems")

	return rootCmd.Execute()
}

func run(ctx context.Context, opts options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}

	if opts.Theme != "" {
		cfg.Theme = opts.Theme
	}

	if opts.RepoPath != "" {
		cfg.RepoPath = opts.RepoPath
	}

	window := aggregator.TimeWindow(cfg.DefaultWindow)
	if opts.JSON || opts.Markdown || opts.CSV || opts.CI {
		loader := dashboard.NewLoader()
		result, err := loader.Load(ctx, cfg.RepoPath, window)
		if err != nil {
			return err
		}

		switch {
		case opts.Markdown:
			_, err = fmt.Fprintln(os.Stdout, exportpkg.Markdown(result))
		case opts.CSV:
			payload, exportErr := exportpkg.CSV(result)
			if exportErr != nil {
				return exportErr
			}
			_, err = fmt.Fprint(os.Stdout, payload)
		default:
			payload, exportErr := exportpkg.JSON(result)
			if exportErr != nil {
				return exportErr
			}
			_, err = fmt.Fprintln(os.Stdout, string(payload))
		}
		return err
	}

	model, err := tui.NewModel(cfg)
	if err != nil {
		return err
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	_ = ctx
	_ = os.Stdout
	return nil
}
