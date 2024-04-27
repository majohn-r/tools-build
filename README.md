# tools-build

[![GoDoc Reference](https://godoc.org/github.com/majohn-r/tools-build?status.svg)](https://pkg.go.dev/github.com/majohn-r/tools-build)
[![go.mod](https://img.shields.io/github/go-mod/go-version/majohn-r/tools-build)](go.mod)
[![LICENSE](https://img.shields.io/github/license/majohn-r/tools-build)](LICENSE)

[![Release](https://img.shields.io/github/v/release/majohn-r/tools-build?include_prereleases)](https://github.com/majohn-r/tools-build/releases)
[![Code Coverage Report](https://codecov.io/github/majohn-r/tools-build/branch/main/graph/badge.svg)](https://codecov.io/github/majohn-r/tools-build)
[![Go Report Card](https://goreportcard.com/badge/github.com/majohn-r/tools-build)](https://goreportcard.com/report/github.com/majohn-r/tools-build)
[![Build Status](https://img.shields.io/github/actions/workflow/status/majohn-r/tools-build/build.yml?branch=main)](https://github.com/majohn-r/tools-build/actions?query=workflow%3Abuild+branch%3Amain)

This package provides build script tooling for go-based projects. The tooling is
in the form of code utilizing [goyek build
automation](https://pkg.go.dev/github.com/goyek/goyek/v2). goyek builds are
typically set up as tasks, and this project provides some common code to perform
the work of the tasks. Here is a sample set of tasks:

```go
var (
    build = goyek.Define(goyek.Task{
        Name:  "build",
        Usage: "build the executable",
        Action: func(a *goyek.A) {
            buildExecutable(a)
        },
    })

    clean = goyek.Define(goyek.Task{
        Name:  "clean",
        Usage: "delete build products",
        Action: func(a *goyek.A) {
            fmt.Println("deleting build products")
            exec, path, _, _ := readConfig()
            workingDir := WorkingDir()
            files := []string{
                filepath.Join(workingDir, path, versionInfoFile),
                filepath.Join(workingDir, path, resourceFile),
                filepath.Join(workingDir, coverageFile),
                filepath.Join(workingDir, exec),
            }
            Clean(files)
        },
    })

    _ = goyek.Define(goyek.Task{
        Name:  "coverage",
        Usage: "run unit tests and produce a coverage report",
        Action: func(a *goyek.A) {
            GenerateCoverageReport(a, coverageFile)
        },
    })

    _ = goyek.Define(goyek.Task{
        Name:  "doc",
        Usage: "generate documentation",
        Action: func(a *goyek.A) {
            GenerateDocumentation(a)
        },
    })

    format = goyek.Define(goyek.Task{
        Name:  "format",
        Usage: "clean up source code formatting",
        Action: func(a *goyek.A) {
            Format(a)
        },
    })

    lint = goyek.Define(goyek.Task{
        Name:  "lint",
        Usage: "run the linter on source code",
        Action: func(a *goyek.A) {
            Lint(a)
        },
    })

    nilaway = goyek.Define(goyek.Task{
        Name:  "nilaway",
        Usage: "run nilaway on source code",
        Action: func(a *goyek.A) {
            NilAway(a)
        },
    })

    vulnCheck = goyek.Define(goyek.Task{
        Name:  "vulnCheck",
        Usage: "run vulnerability check on source code",
        Action: func(a *goyek.A) {
            VulnerabilityCheck(a)
        },
    })

    _ = goyek.Define(goyek.Task{
        Name:  "preCommit",
        Usage: "run all pre-commit tasks",
        Deps:  goyek.Deps{clean, lint, nilaway, format, vulnCheck, tests, build},
    })

    tests = goyek.Define(goyek.Task{
        Name:  "tests",
        Usage: "run unit tests",
        Action: func(a *goyek.A) {
            UnitTests(a)
        },
    })
)

```

And here is a typical build script to execute the tasks:

```bash
#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
    tracing=true
else
    tracing=false
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
cd "${DIR}/build"
if [[ "${tracing}" == "true" ]]; then
    DIR=${DIR} go run . -v "$@"
else
    DIR=${DIR} go run . "$@"
fi
```

The script employs a **DIR** environment variable for the benefit of the
**WorkingDir** function, which is used by this package to find the project's top
level directory. If **DIR** is not set as an environment variable,
**WorkingDir** will assume that "**..**" is the correct location, which is based
on the assumption that the go code running the build is placed in a directory,
one level deep, such as **build** (as seen in the line above ```cd
"${DIR}/build```). Regardless of whether or not the **DIR** environment variable
is set, the **WorkingDir** function looks for the **.git** directory in its
candidate value, and it's not found, then the **WorkingDir** function calls
**os.Exit** and the build ends.

## Opinionated?

Well, yes. I wrote this for _my_ go projects, and, as such, it reflects _my_
thinking about the proper tooling to use, and how to use that tooling. The
biggest example of this is probably my use of
[gocritic](https://github.com/go-critic/go-critic) as the tool called by the
**Lint** function.

That said, if you find the package useful but don't like some of my choices, you
can easily create your own functions to replace the ones you don't care for.
Won't hurt my feelings a bit.
