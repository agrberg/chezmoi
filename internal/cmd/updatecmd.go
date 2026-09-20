package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

type updateCmdConfig struct {
	Command           string   `json:"command"           mapstructure:"command"           yaml:"command"`
	Args              []string `json:"args"              mapstructure:"args"              yaml:"args"`
	Apply             bool     `json:"apply"             mapstructure:"apply"             yaml:"apply"`
	RecurseSubmodules bool     `json:"recurseSubmodules" mapstructure:"recurseSubmodules" yaml:"recurseSubmodules"`
	filter            *chezmoi.EntryTypeFilter
	init              bool
	parentDirs        bool
	recursive         bool
}

func (c *Config) newUpdateCmd() *cobra.Command {
	updateCmd := &cobra.Command{
		GroupID:           groupIDDaily,
		Use:               "update",
		Short:             "Pull and apply any changes",
		Long:              mustLongHelp("update"),
		Example:           example("update"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.runUpdateCmd,
		Annotations: newAnnotations(
			modifiesDestinationDirectory,
			persistentStateModeReadWrite,
			requiresSourceDirectory,
			requiresWorkingTree,
			runsCommands,
		),
	}

	updateCmd.Flags().BoolVarP(&c.Update.Apply, "apply", "a", c.Update.Apply, "Apply after pulling")
	updateCmd.Flags().VarP(c.Update.filter.Exclude, "exclude", "x", "Exclude entry types")
	updateCmd.Flags().VarP(c.Update.filter.Include, "include", "i", "Include entry types")
	updateCmd.Flags().BoolVar(&c.Update.init, "init", c.Update.init, "Recreate config file from template")
	updateCmd.Flags().BoolVarP(&c.Update.parentDirs, "parent-dirs", "P", c.Update.parentDirs, "Update all parent directories")
	updateCmd.Flags().
		BoolVar(&c.Update.RecurseSubmodules, "recurse-submodules", c.Update.RecurseSubmodules, "Recursively update submodules")
	updateCmd.Flags().BoolVarP(&c.Update.recursive, "recursive", "r", c.Update.recursive, "Recurse into subdirectories")

	return updateCmd
}

func (c *Config) runUpdateCmd(cmd *cobra.Command, args []string) error {
	if err := c.updatePull(); err != nil {
		return err
	}

	if c.Update.Apply {
		if err := c.applyArgsAndPrintSummary(cmd.Context(), args, applyArgsOptions{
			cmd:        cmd,
			filter:     c.Update.filter,
			init:       c.Update.init,
			parentDirs: c.Update.parentDirs,
			recursive:  c.Update.recursive,
			umask:      c.Umask,
		}); err != nil {
			return err
		}
	}

	return nil
}

// updatePull brings the source state up to date with the remote. Dry run mode
// should not change state.
func (c *Config) updatePull() error {
	if c.dryRun {
		return c.updatePullDryRun()
	}
	switch {
	case c.Update.Command != "":
		if err := c.run(c.WorkingTreeAbsPath, c.Update.Command, c.Update.Args); err != nil {
			return err
		}
	case c.UseBuiltinGit.Value(c.useBuiltinGitAutoFunc):
		rawWorkingTreeAbsPath, err := c.baseSystem.RawPath(c.WorkingTreeAbsPath)
		if err != nil {
			return err
		}
		repo, err := git.PlainOpen(rawWorkingTreeAbsPath.String())
		if err != nil {
			return err
		}
		wt, err := repo.Worktree()
		if err != nil {
			return err
		}
		if err := wt.Pull(&git.PullOptions{
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return err
		}
	default:
		gitArgs := []string{
			"pull",
			"--autostash",
			"--rebase",
		}
		if c.Update.RecurseSubmodules {
			gitArgs = append(gitArgs,
				"--recurse-submodules",
			)
		}
		if err := c.run(c.WorkingTreeAbsPath, c.Git.Command, gitArgs); err != nil {
			return err
		}
	}
	return nil
}

// updatePullDryRun fetches from the remote and, unless the apply step is
// disabled, repoints the source state at an extracted copy of upstream tree so
// subsequent dry-run apply reflects the incoming changes without moving the
// local branch.
//
// update.command and builtin git cannot fetch without also updating the
// working tree so this warns and skips the pull entirely leaving dry-run apply
// to report only changes already present in the source directory.
func (c *Config) updatePullDryRun() error {
	if c.Update.Command != "" {
		c.errorf("warning: update.command does not support --dry-run, skipping pull\n")
		return nil
	}
	if c.UseBuiltinGit.Value(c.useBuiltinGitAutoFunc) {
		c.errorf("warning: useBuiltinGit does not support --dry-run, skipping pull\n")
		return nil
	}

	gitArgs := []string{"fetch"}
	if c.Update.RecurseSubmodules {
		gitArgs = append(gitArgs, "--recurse-submodules")
	}
	if err := c.run(c.WorkingTreeAbsPath, c.Git.Command, gitArgs); err != nil {
		return err
	}

	if !c.Update.Apply {
		// Nothing will be applied, so there is nothing to render.
		return nil
	}

	sourceDirRelPath, err := c.SourceDirAbsPath.TrimDirPrefix(c.WorkingTreeAbsPath)
	if err != nil {
		return err
	}

	upstreamCommitOutput, err := c.cmdOutput(c.WorkingTreeAbsPath, c.Git.Command, []string{"rev-parse", "--verify", "@{u}"})
	if err != nil {
		return fmt.Errorf("no upstream branch configured for the current branch: %w", err)
	}
	upstreamCommit := strings.TrimSpace(string(upstreamCommitOutput))

	archiveData, err := c.cmdOutput(
		c.WorkingTreeAbsPath, c.Git.Command, []string{"archive", "--format=tar", upstreamCommit},
	)
	if err != nil {
		return err
	}

	tempDirAbsPath, err := c.tempDir("chezmoi-update")
	if err != nil {
		return err
	}
	if err := c.extractUpdateArchive(archiveData, tempDirAbsPath); err != nil {
		return err
	}

	c.SourceDirAbsPath = tempDirAbsPath.Join(sourceDirRelPath)
	sourceDirAbsPath, err := c.getSourceDirAbsPath(&getSourceDirAbsPathOptions{
		refresh: true,
	})
	if err != nil {
		return err
	}
	c.templateData.sourceDir = sourceDirAbsPath.String()
	if err := os.Setenv("CHEZMOI_SOURCE_DIR", sourceDirAbsPath.String()); err != nil {
		return err
	}
	c.resetSourceState()

	return nil
}

func (c *Config) extractUpdateArchive(data []byte, dirAbsPath chezmoi.AbsPath) error {
	return chezmoi.WalkArchive(data, chezmoi.ArchiveFormatTar, func(
		name chezmoi.RelPath, info fs.FileInfo, r io.Reader, linkname string,
	) error {
		absPath := dirAbsPath.Join(name)
		switch {
		case info.IsDir():
			return chezmoi.MkdirAll(c.baseSystem, absPath, info.Mode().Perm())
		case info.Mode()&fs.ModeType == 0:
			contents, err := io.ReadAll(r)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return c.baseSystem.WriteFile(absPath, contents, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			return c.baseSystem.WriteSymlink(linkname, absPath)
		default:
			return fmt.Errorf("%s: not a file, directory, or symlink", name)
		}
	})
}
