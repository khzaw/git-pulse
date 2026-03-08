package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"git-pulse/internal/config"
	"git-pulse/internal/tui"
)

type options struct {
	RepoPath   string
	ConfigPath string
	Theme      string
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
