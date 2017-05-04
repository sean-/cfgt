# `cfgt` - Configuration File Translator

## Installation

```
$ go get -u github.com/sean-/cfgt
```

## Usage

NOTE: The `convert` sub-command is the default if no sub-command is specified.

```
usage: cfgt [<flags>] <command> [<args> ...]

A configuration file translation utility

Flags:
      --help               Show context-sensitive help (also try --help-long and
                           --help-man).
      --version            Show application version.
  -d, --debug              Enable debug mode.
  -i, --in="-"             Filename to read from
  -o, --out="-"            Filename to write to
  -I, --in-format="*"      Input format ("json", "json5", or "hcl")
  -O, --out-format="json"  Output format ("json")
  -p, --pretty             Pretty-print the output (true if output is a
                           terminal)

Commands:
  help [<command>...]
    Show help.


  convert [<flags>]
    Convert a configuration file to JSON

    -I, --in-format="*"      Input format ("json", "json5", or "hcl")
    -O, --out-format="json"  Output format ("json")
    -p, --pretty             Pretty-print the output (true if output is a
                             terminal)
```
