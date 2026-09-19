# `preview`

An alias for `update --dry-run --verbose`. Fetches from the remote and prints
the diff that `update` would apply, without pulling and without modifying the
destination directory.

`preview` accepts the same flags as [`update`][update], because it uses
`update`'s logic directly with dry run mode forced on.

## Flags

### `-a`, `--apply`

Apply changes after pulling, `true` by default. Can be disabled with
`--apply=false`. With `--apply=false`, nothing is applied, so `preview` has
nothing to show.

### `--recurse-submodules`

Update submodules recursively. This defaults to `true`. Can be disabled with
`--recurse-submodules=false`.

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

## Examples

```sh
chezmoi preview
```

[update]: /reference/commands/update.md
