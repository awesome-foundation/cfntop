package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	cfnaws "github.com/awesome-foundation/cfntop/internal/aws"
	"github.com/awesome-foundation/cfntop/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		region   string
		profile  string
		interval int
		absolute bool
	)

	cmd := &cobra.Command{
		Use:   "cfntop",
		Short: "Live monitor for AWS CloudFormation stacks",
		Long: `cfntop is a TUI that continuously monitors CloudFormation stacks in a region.

Stacks are sorted by last update, with active deployments on top.
Expand a stack to see its resources and their current status.
ECS services show active deployments with task counts.`,
		Version: fmt.Sprintf("%s (%s, %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			var opts []func(*config.LoadOptions) error
			if region != "" {
				opts = append(opts, config.WithRegion(region))
			}
			if profile != "" {
				opts = append(opts, config.WithSharedConfigProfile(profile))
			}
			cfg, err := config.LoadDefaultConfig(ctx, opts...)
			if err != nil {
				return fmt.Errorf("loading AWS config: %w", err)
			}

			cfnClient, err := cfnaws.NewClient(ctx, region, profile)
			if err != nil {
				return fmt.Errorf("%s", cfnaws.FormatError(err))
			}
			ecsClient := ecs.NewFromConfig(cfg)

			m := tui.NewModel(cfnClient, ecsClient, time.Duration(interval)*time.Second, !absolute)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&region, "region", "r", "", "AWS region")
	cmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS profile")
	cmd.Flags().IntVarP(&interval, "interval", "n", 5, "Poll interval in seconds")
	cmd.Flags().BoolVar(&absolute, "absolute-time", false, "Show absolute timestamps instead of relative (e.g. 2m ago)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
