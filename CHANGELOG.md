# Changelog

This project uses [semantic versioning](https://semver.org/); be aware that, until the major version becomes non-zero,
[this proviso](https://semver.org/#spec-item-4) applies.

Key to symbols
- ❗ breaking change
- 🐛 bug fix
- ⚠️ change in behavior, may surprise the user
- 😒 change is invisible to the user
- 🆕 new feature

## v0.14.2

_release `2026-02-17`_

- 😒 update to reference go 1.26

## v0.14.1

_release `2026-02-17`_

- 😒 update one build dependency and accept one change made by `go fix`.

## v0.14.0

_release `2026-01-24`_

- ⚠️ update dependencies. Since the previous update, goyek released a new major version (v3). This release uses that
new version. **Consuming projects will need to upgrade as well.** Releasing this project with a bump in the minor
version, as the [changes in goyek](https://github.com/goyek/goyek/pull/532) do not appear to break existing usage;
also, this does not seem to be the kind of change that forces a version 1.

## v0.13.3

_release `2025-10-09`_

- 😒 update dependencies

## v0.13.2

_release `2025-08-29`_

- 😒 update dependencies, particularly the go version

## v0.13.1

_release `2025-07-19`_

- 😒 update dependencies

## v0.13.0

_release `2024-08-20`_

- 🆕add **TaskDisabled()** function so that tasks can be disabled on the fly

## v0.12.1

_release `2024-07-21`_

- 🐛correct **Deadcode()** function options name from ~~notype~~ to **notest** 

## v0.12.0

_release `2024-07-21`_

- 🆕add **Deadcode()** function to run deadcode analysis

## v0.11.0

_release `2024-06-27`_

- 😒no significant changes visible to consumers

## v0.10.1

_release `2024-06-26`_

- 🐛explicitly expand filenames when **""** is specified in **FormatSelective()**

## v0.10.0

_release `2024-06-26`_

- 🆕add **FormatSelective()** function

## v0.9.0

_release `2024-06-25`_

- 🐛allow directories to be excluded at a high level

## v0.8.3

_release `2024-06-16`_

- 🐛improve output for setting/unsetting/restoring environment variables, including outputting the initial state of the
variable

## v0.8.2

_release `2024-06-16`_

- 🐛improve output for setting/unsetting/restoring environment variables

## v0.8.1

_release `2024-06-16`_

- 🐛if a task involves setting or unsetting an environment variable, those actions (set/unset) are output

## v0.8.0

_release `2024-06-15`_

- 🆕add **-aggressive** option for **UpdateDependencies()**

## v0.7.2

_release `2024-05-28`_

- 🐛**UpdateDependencies()** output includes the folder containing the `go.mod` file it's working on

## v0.7.1

_release `2024-05-28`_

- 🆕**UpdateDependencies()** searches for `go.mod` files under the working directory and updates each one it finds

## v0.6.0

_release `2024-05-27`_

- 🐛**Generate()** now uses `go generate -x` instead of `go generate -v`

## v0.5.0

_release `2024-05-19`_

- ⚠️**Clean()** and **GenerateCoverageReport()** are stricter on invalid file names, specifically blocking
files whose paths contain '..' or whose paths begin with a path separator or a drive letter

## v0.4.0

_release `2024-05-18`_

- ⚠️**Clean()** will only delete files that are contained in the current working directory

## v0.3.0

_release `2024-05-04`_

- 🆕add function to update dependencies

## v0.2.2

_release `2024-04-28`_

- 🐛add newline to **Clean()** output when an invalid file is detected
- 🐛further reduce empty lines of output by eliminating redundant end-of-line characters

## v0.2.1

_release `2024-04-28`_

- 🆕add support for running **go generate**
- 🐛reduce empty lines of output

## v0.2.0

_release `2024-04-28`_

- ⚠️**Clean()** will exit if any of the files include **'..'** in its path

## v0.1.0

_release `2024-04-27`_

- 🆕initial version