package cmd

import (
	"context"
	"errors"
	"io/fs"

	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

type applyCmdConfig struct {
	filter     *chezmoi.EntryTypeFilter
	init       bool
	parentDirs bool
	recursive  bool
}

func (c *Config) newApplyCmd() *cobra.Command {
	applyCmd := &cobra.Command{
		GroupID:           groupIDDaily,
		Use:               "apply [target]...",
		Short:             "Update the destination directory to match the target state",
		Long:              mustLongHelp("apply"),
		Example:           example("apply"),
		ValidArgsFunction: c.targetValidArgs,
		RunE:              c.runApplyCmd,
		Annotations: newAnnotations(
			modifiesDestinationDirectory,
			persistentStateModeReadWrite,
			requiresSourceDirectory,
		),
	}

	applyCmd.Flags().VarP(c.apply.filter.Exclude, "exclude", "x", "Exclude entry types")
	applyCmd.Flags().VarP(c.apply.filter.Include, "include", "i", "Include entry types")
	applyCmd.Flags().BoolVar(&c.apply.init, "init", c.apply.init, "Recreate config file from template")
	applyCmd.Flags().BoolVarP(&c.apply.parentDirs, "parent-dirs", "P", c.apply.parentDirs, "Apply all parent directories")
	applyCmd.Flags().BoolVarP(&c.apply.recursive, "recursive", "r", c.apply.recursive, "Recurse into subdirectories")

	return applyCmd
}

func (c *Config) runApplyCmd(cmd *cobra.Command, args []string) error {
	return c.applyArgsAndPrintSummary(cmd.Context(), args, applyArgsOptions{
		cmd:        cmd,
		filter:     c.apply.filter,
		init:       c.apply.init,
		parentDirs: c.apply.parentDirs,
		recursive:  c.apply.recursive,
		umask:      c.Umask,
	})
}

// applyArgsAndPrintSummary applies args to the destination directory and then
// reports the apply phase's effect on it. options.preApplyFunc must be nil so
// c.defaultPreApplyFunc can be wrapped to count the targets that change and
// the targets that are skipped.
func (c *Config) applyArgsAndPrintSummary(
	ctx context.Context,
	args []string,
	options applyArgsOptions,
) error {
	if options.preApplyFunc != nil {
		panic("applyArgsAndPrintSummary: options.preApplyFunc must be nil")
	}

	changedTargets := 0
	skippedTargets := 0
	options.preApplyFunc = func(
		targetRelPath chezmoi.RelPath,
		targetEntryState, lastWrittenEntryState, actualEntryState *chezmoi.EntryState,
	) error {
		err := c.defaultPreApplyFunc(
			targetRelPath, targetEntryState, lastWrittenEntryState, actualEntryState,
		)
		if errors.Is(err, fs.SkipDir) {
			skippedTargets++
			return err
		}
		if err != nil {
			return err
		}
		// Count here instead of inside defaultPreApplyFunc as --force makes that
		// function return before it compares the states.
		if !targetEntryState.Equivalent(actualEntryState) {
			changedTargets++
		}
		return nil
	}

	if err := c.applyArgs(ctx, c.destSystem, c.DestDirAbsPath, args, options); err != nil {
		// Quitting still leaves whatever was already applied so we print the
		// summary here too, as applyArgs would return early.
		var exitCodeErr chezmoi.ExitCodeError
		if !errors.As(err, &exitCodeErr) || exitCodeErr != 0 {
			return err
		}
		c.printApplySummary(changedTargets, skippedTargets)
		return err
	}

	c.printApplySummary(changedTargets, skippedTargets)
	return nil
}

// printApplySummary reports the apply phase's effect on the destination
// directory: targets changed, skipped, or that nothing happened.
func (c *Config) printApplySummary(changedTargets, skippedTargets int) {
	if changedTargets == 0 && skippedTargets == 0 {
		c.errorf("destination directory is up to date\n")
		return
	}

	if changedTargets == 0 {
		targets := english.Plural(skippedTargets, "target", "targets")
		c.errorf("skipped changes to %s\n", targets)
		return
	}

	verb := "applied"
	if c.dryRun {
		verb = "would apply"
	}
	targets := english.Plural(changedTargets, "target", "targets")
	c.errorf("%s changes to %s\n", verb, targets)
}
