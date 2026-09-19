# `apply` [*target*...]

Ensure that *target*... are in the target state, updating them if necessary. If
no targets are specified, the state of all targets are ensured. If a target has
been modified since chezmoi last wrote it then the user will be prompted if
they want to overwrite the file.

!!! note

    chezmoi reports the effect on the destination directory: how many targets
    changed, how many were skipped, or that the destination was already up to
    date. This count reflects what was applied, not what `--verbose` rendered,
    so it can differ from the diff when `diff.exclude` filters out some of
    the rendered hunks.

## Common flags

### `-x`, `--exclude` *types*

--8<-- "common-flags/exclude.md"

### `-i`, `--include` *types*

--8<-- "common-flags/include.md"

### `--init`

--8<-- "common-flags/init.md"

### `-P`, `--parent-dirs`

--8<-- "common-flags/parent-dirs.md"

### `-r`, `--recursive`

--8<-- "common-flags/recursive.md:default-true"

### `--source-path`

Specify targets by source path, rather than target path. This is useful for
applying changes after editing.

## Examples

```sh
chezmoi apply
chezmoi apply --dry-run --verbose
chezmoi apply ~/.bashrc
```
