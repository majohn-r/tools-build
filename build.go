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
// calls os.Exit(); this is to prevent callers from deceptively removing files
// they shouldn't.
func Clean(files []string) {
	workingFS := os.DirFS(WorkingDir())
	for _, file := range files {
		if isIllegalFileName(file) {
			fmt.Fprintf(os.Stderr, "file %q will not be removed, exiting the build\n", file)
			exit(1)
		}
		openFile, err := workingFS.Open(file)
		if err == nil {
			openFile.Close()
			fileSystem.Remove(filepath.Join(WorkingDir(), file))
		}
	}
}

func isIllegalFileName(path string) bool {
	return path == "" || isMalformedFileName(path)
}

func isMalformedFileName(path string) bool {
	if path == "" {
		return false
	}
	if startsWith(path, "/") || startsWith(path, "\\") {
		return true
	}
	dir, file := filepath.Split(canonicalizePath(path))
	if file == ".." {
		return true
	}
	if dir == path && file == "" {
		// happens when the original path begins with a drive letter and colon
		return true
	}
	return isMalformedFileName(strings.TrimSuffix(dir, "/"))
}

func canonicalizePath(path string) string {
	if strings.Contains(path, "\\") {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

// Format runs the gofmt tool to repair the formatting of each source file;
// returns false if the command fails
func Format(a *goyek.A) bool {
	printLine("cleaning up source code formatting")
	return RunCommand(a, "gofmt -e -l -s -w .")
}

// Generate runs the 'go generate' tool
func Generate(a *goyek.A) bool {
	printLine("running go generate")
	return RunCommand(a, "go generate -x ./...")
}

// GenerateCoverageReport runs the unit tests, generating a coverage profile; if
// the unit tests all succeed, generates the report as HTML to be displayed in
// the current browser window. Returns false if either the unit tests or the
// coverage report display fails
func GenerateCoverageReport(a *goyek.A, coverageDataFile string) bool {
	if isIllegalFileName(coverageDataFile) {
		fmt.Fprintf(os.Stderr, "cannot accept %q as a valid file name to which coverage data can be written", coverageDataFile)
		return false
	}
	fmt.Printf("executing unit tests, writing coverage data to %q\n", coverageDataFile)
	if !RunCommand(a, fmt.Sprintf("go test -coverprofile=%s ./...", coverageDataFile)) {
		return false
	}
	fmt.Printf("displaying coverage report from %q\n", coverageDataFile)
	return RunCommand(a, fmt.Sprintf("go tool cover -html=%s", coverageDataFile))
}

// GenerateDocumentation generates documentation of the code, outputting it to
// stdout; returns false on error
func GenerateDocumentation(a *goyek.A, excludedDirs []string) bool {
	dirs, err := relevantDirs(matchGoSource)
	if err != nil {
		return false
	}
	o := &bytes.Buffer{}
	for _, dir := range dirs {
		documentSources := true
		for _, dirToExclude := range excludedDirs {
			if dir == dirToExclude {
				documentSources = false
				break
			}
		}
		if documentSources {
			docCommand := directedCommand{
				command: fmt.Sprintf("go doc -all ./%s", dir),
				dir:     WorkingDir(),
			}
			if !docCommand.execute(a) {
				return false
			}
		}
	}
	printBuffer(o)
	return true
}

func matchModuleFile(name string) bool {
	return name == "go.mod"
}

func matchGoSource(name string) bool {
	return endsIn(name, ".go") && !endsIn(name, "_test.go") && !startsWith(name, "testing")
}

func printBuffer(b *bytes.Buffer) {
	s := eatTrailingEOL(b.String())
	if s != "" {
		printLine(s)
	}
}

func eatTrailingEOL(s string) string {
	switch {
	case strings.HasSuffix(s, "\n"):
		return eatTrailingEOL(strings.TrimSuffix(s, "\n"))
	case strings.HasSuffix(s, "\r"):
		return eatTrailingEOL(strings.TrimSuffix(s, "\r"))
	default:
		return s
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
	dc := directedCommand{command: command, dir: WorkingDir()}
	return dc.execute(a)
}

// UnitTests runs all unit tests, with code coverage enabled; returns false on
// failure
func UnitTests(a *goyek.A) bool {
	printLine("running all unit tests")
	return RunCommand(a, "go test -cover ./...")
}

type directedCommand struct {
	command string
	dir     string
}

// UpdateDependencies updates module dependencies and prunes the modified go.mod
// and go.sum files
func UpdateDependencies(a *goyek.A) bool {
	dirs, err := relevantDirs(matchModuleFile)
	if err != nil {
		return false
	}
	getCommand := directedCommand{command: "go get -u ./..."}
	tidyCommand := directedCommand{command: "go mod tidy"}
	for _, dir := range dirs {
		getCommand.dir = dir
		tidyCommand.dir = dir
		fmt.Printf("updating dependencies in folder %q", dir)
		if !getCommand.execute(a) {
			return false
		}
		printLine("pruning go.mod and go.sum")
		if !tidyCommand.execute(a) {
			return false
		}
	}
	return true
}

func (dC directedCommand) execute(a *goyek.A) bool {
	outputBuffer := &bytes.Buffer{}
	defer printBuffer(outputBuffer)
	options := make([]cmd.Option, 3)
	options[0] = cmd.Dir(dC.dir)
	options[1] = cmd.Stderr(outputBuffer)
	options[2] = cmd.Stdout(outputBuffer)
	return executor(a, dC.command, options...)
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
		if dirValue, dirExists := os.LookupEnv("DIR"); dirExists {
			candidate = dirValue
		}
		if isUnacceptableWorkingDir(candidate) {
			exit(1)
		}
		// ok, it's acceptable
		workingDir = candidate
	}
	return workingDir
}

func isUnacceptableWorkingDir(candidate string) bool {
	if candidate == "" {
		fmt.Fprintln(os.Stderr, "code error: empty candidate value passed to isAcceptableWorkingDir")
		return true
	}
	if isInvalidDir(candidate) {
		return true
	}
	if isInvalidDir(filepath.Join(candidate, ".git")) {
		return true
	}
	return false // directory is appropriate to use
}

func isInvalidDir(path string) bool {
	pathIsConfirmedDir, err := afero.IsDir(fileSystem, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation error %v for %q", err, path)
		return true
	}
	if !pathIsConfirmedDir {
		fmt.Fprintf(os.Stderr, "not a directory: %q", path)
		return true
	}
	return false
}

func allDirs(top string) ([]string, error) {
	var topIsDir bool
	var err error
	if topIsDir, err = afero.IsDir(fileSystem, top); err != nil {
		return nil, err
	}
	if !topIsDir {
		return nil, fmt.Errorf("%q is not a directory", top)
	}
	top = canonicalizePath(top)
	entries, _ := afero.ReadDir(fileSystem, top)
	dirs := []string{top}
	for _, entry := range entries {
		if entry.IsDir() {
			subDirs, _ := allDirs(filepath.Join(top, entry.Name()))
			dirs = append(dirs, subDirs...)
		}
	}
	return dirs, nil
}

func relevantDirs(fileMatcher func(string) bool) ([]string, error) {
	topDir := WorkingDir()
	dirs, err := allDirs(topDir)
	if err != nil {
		return nil, err
	}
	sourceDirectories := []string{}
	for _, dir := range dirs {
		if includesRelevantFiles(dir, fileMatcher) {
			sourceDir := strings.TrimPrefix(strings.TrimPrefix(dir, topDir), "/")
			sourceDirectories = append(sourceDirectories, sourceDir)
		}
	}
	return sourceDirectories, nil
}

func includesRelevantFiles(dir string, fileMatcher func(string) bool) bool {
	entries, err := afero.ReadDir(fileSystem, dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if isRelevantFile(e, fileMatcher) {
			return true
		}
	}
	return false
}

func isRelevantFile(entry fs.FileInfo, fileMatcher func(string) bool) bool {
	if !entry.Mode().IsRegular() {
		return false
	}
	name := entry.Name()
	return fileMatcher(name)
}

// Silly functions? They make the usage clearer

func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func endsIn(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}
