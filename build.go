package tools_build

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
	"github.com/spf13/afero"
)

var (
	fileSystem = afero.NewOsFs()
	workingDir = ""
	// function vars make it easy for tests to stub out functionality
	exit      = os.Exit
	executor  = cmd.Exec
	printLine = fmt.Println
)

// Clean deletes the named files, which must be located in, or in a subdirectory
// of, WorkingDir(). If any of the named files contains a back directory (".."),
// calls os.Exit()
func Clean(files []string) {
	for _, file := range files {
		if containsBackDir(file) {
			fmt.Fprintf(os.Stderr, "file %q will not be removed, exiting the build", file)
			exit(1)
		}
		fileSystem.Remove(filepath.Join(WorkingDir(), file))
	}
}

func containsBackDir(f string) bool {
	if !strings.Contains(f, "..") {
		return false
	}
	if f == ".." {
		return true
	}
	dir, file := filepath.Split(f)
	if file == ".." {
		return true
	}
	dir = strings.ReplaceAll(dir, "\\", "/")
	dir = strings.TrimSuffix(dir, "/")
	return containsBackDir(dir)
}

// Format runs the gofmt tool to repair the formatting of each source file;
// returns false if the command fails
func Format(a *goyek.A) bool {
	printLine("cleaning up source code formatting")
	return RunCommand(a, "gofmt -e -l -s -w .")
}

// Generate runs the 'go generate' tool
func Generate(a *goyek.A) bool {
	fmt.Println("running go generate")
	return RunCommand(a, "go generate -v ./...")
}

// GenerateCoverageReport runs the unit tests, generating a coverage profile; if
// the unit tests all succeed, generates the report as HTML to be displayed in
// the current browser window. Returns false if either the unit tests or the
// coverage report display fails
func GenerateCoverageReport(a *goyek.A, file string) bool {
	fmt.Printf("executing unit tests, writing coverage data to %q\n", file)
	if !RunCommand(a, fmt.Sprintf("go test -coverprofile=%s ./...", file)) {
		return false
	}
	fmt.Printf("displaying coverage report from %q\n", file)
	return RunCommand(a, fmt.Sprintf("go tool cover -html=%s", file))
}

// GenerateDocumentation generates documentation of the code, outputting it to
// stdout; returns false on error
func GenerateDocumentation(a *goyek.A, excludedDirs []string) bool {
	dirs, err := sourceDirs()
	if err != nil {
		return false
	}
	o := &bytes.Buffer{}
	for _, dir := range dirs {
		shouldDocument := true
		for _, dirToExclude := range excludedDirs {
			if dir == dirToExclude {
				shouldDocument = false
				break
			}
		}
		if shouldDocument {
			if !executor(a, fmt.Sprintf("go doc -all ./%s", dir), MakeCmdOptions(o)...) {
				return false
			}
		}
	}
	printBuffer(o)
	return true
}

func printBuffer(b *bytes.Buffer) {
	s := b.String()
	if s != "" {
		printLine(s)
	}
}

// Install runs the command to install the '@latest' version of a specified
// package; returns false on failure
func Install(a *goyek.A, packageName string) bool {
	fmt.Printf("installing the latest version of %s\n", packageName)
	return RunCommand(a, fmt.Sprintf("go install -v %s@latest", packageName))
}

// Lint runs lint on the source code after making sure that the lint tool is up
// to date; returns false on failure
func Lint(a *goyek.A) bool {
	if !Install(a, "github.com/go-critic/go-critic/cmd/gocritic") {
		return false
	}
	printLine("linting source code")
	return RunCommand(a, "gocritic check -enableAll ./...")
}

// MakeCmdOptions creates a slice of cmd.Option instances consisting of the
// working directory, stderr (using the provided buffer), and stdout (using the
// same provided buffer)
func MakeCmdOptions(b *bytes.Buffer) []cmd.Option {
	o := make([]cmd.Option, 3)
	o[0] = cmd.Dir(WorkingDir())
	o[1] = cmd.Stderr(b)
	o[2] = cmd.Stdout(b)
	return o
}

// NilAway runs the nilaway tool, which attempts, via static analysis, to detect
// potential nil access errors; returns false on errors
func NilAway(a *goyek.A) bool {
	if !Install(a, "go.uber.org/nilaway/cmd/nilaway") {
		return false
	}
	printLine("running nilaway analysis")
	return RunCommand(a, "nilaway ./...")
}

// RunCommand runs a command and displays all of its output; returns true on
// success
func RunCommand(a *goyek.A, command string) bool {
	outputBuffer := &bytes.Buffer{}
	defer printBuffer(outputBuffer)
	return executor(a, command, MakeCmdOptions(outputBuffer)...)
}

// UnitTests runs all unit tests, with code coverage enabled; returns false on
// failure
func UnitTests(a *goyek.A) bool {
	printLine("running all unit tests")
	return RunCommand(a, "go test -cover ./...")
}

// VulnerabilityCheck runs the govulncheck tool, which checks for unresolved
// known vulnerabilities in the libraries used; returns false on failure
func VulnerabilityCheck(a *goyek.A) bool {
	if !Install(a, "golang.org/x/vuln/cmd/govulncheck") {
		return false
	}
	printLine("running vulnerability checks")
	return RunCommand(a, "govulncheck -show verbose ./...")
}

// WorkingDir returns a best guess of the working directory. If the directory
// found is not, in fact, a directory, or is, but does not contain a .git
// subdirectory, calls exit. A successful call's value is cached.
func WorkingDir() string {
	if workingDir == "" {
		candidate := ".."
		if dir, ok := os.LookupEnv("DIR"); ok {
			candidate = dir
		}
		if err := isAcceptableWorkingDir(candidate); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			exit(1)
		}
		// ok, it's acceptable
		workingDir = candidate
	}
	return workingDir
}

func isAcceptableWorkingDir(candidate string) error {
	if candidate == "" {
		return fmt.Errorf("empty value")
	}
	ok, err := afero.IsDir(fileSystem, candidate)
	if err != nil {
		return fmt.Errorf("validation error %v for %q", err, candidate)
	}
	if !ok {
		return fmt.Errorf("not a directory: %q", candidate)
	}
	gitDir := filepath.Join(candidate, ".git")
	ok, err = afero.IsDir(fileSystem, gitDir)
	if err != nil {
		return fmt.Errorf("validation error %v for %q", err, gitDir)
	}
	if !ok {
		return fmt.Errorf("not a directory: %q", gitDir)
	}
	// ok, it's acceptable
	return nil
}

func allDirs(top string) ([]string, error) {
	var entries []fs.FileInfo
	var err error
	if entries, err = afero.ReadDir(fileSystem, top); err != nil {
		return nil, err
	}
	dirs := []string{strings.ReplaceAll(top, "\\", "/")}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		f := filepath.Join(top, entry.Name())
		var subDirs []string
		// possibility of an error in accessing subdirectories is virtually non-existent
		if subDirs, err = allDirs(f); err == nil {
			dirs = append(dirs, subDirs...)
		}
	}
	return dirs, nil
}

func sourceDirs() ([]string, error) {
	topDir := WorkingDir()
	dirs, err := allDirs(topDir)
	if err != nil {
		return nil, err
	}
	sourceDirectories := []string{}
	for _, dir := range dirs {
		if includesGoSource(dir) {
			sourceDir := strings.TrimPrefix(strings.TrimPrefix(dir, topDir), "/")
			sourceDirectories = append(sourceDirectories, sourceDir)
		}
	}
	return sourceDirectories, nil
}

func includesGoSource(dir string) bool {
	entries, err := afero.ReadDir(fileSystem, dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if isGoSourceFile(e) {
			return true
		}
	}
	return false
}

func isGoSourceFile(entry fs.FileInfo) bool {
	if !entry.Mode().IsRegular() {
		return false
	}
	name := entry.Name()
	return endsIn(name, ".go") && !endsIn(name, "_test.go") && !startsWith(name, "testing")
}

// Silly functions? They make the usage clearer

func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func endsIn(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}
