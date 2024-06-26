package tools_build

import (
	"bytes"
	"flag"
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
	aggressive = flag.Bool("aggressive", false, "set to make dependency updates more aggressive")
	// function vars make it easy for tests to stub out functionality
	executor  = cmd.Exec
	exit      = os.Exit
	printLine = fmt.Println
	setenv    = os.Setenv
	unsetenv  = os.Unsetenv
)

// Clean deletes the named files, which must be located in, or in a subdirectory
// of, WorkingDir(). If any of the named files contains a back directory (".."),
// calls os.Exit(); this is to prevent callers from deceptively removing files
// they shouldn't.
func Clean(files []string) {
	workingFS := os.DirFS(WorkingDir())
	for _, file := range files {
		if isIllegalFileName(file) {
			_, _ = fmt.Fprintf(os.Stderr, "file %q will not be removed, exiting the build\n", file)
			exit(1)
		}
		openFile, err := workingFS.Open(file)
		if err == nil {
			_ = openFile.Close()
			_ = fileSystem.Remove(filepath.Join(WorkingDir(), file))
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
	printIt("cleaning up source code formatting")
	return RunCommand(a, "gofmt -e -l -s -w .")
}

// FormatSelective runs the gofmt tool to repair the formatting of selected source files; returns false if the command fails
func FormatSelective(a *goyek.A, exclusions []string) bool {
	if len(exclusions) == 0 {
		return Format(a)
	}
	printIt("cleaning up source code formatting, excluding folders", exclusions)
	srcDirs, err := relevantDirs(matchAnyGoFile)
	if err != nil {
		return false
	}
	command := []string{"gofmt -e -l -s -w"}
	for _, src := range srcDirs {
		switch src {
		case "":
			command = append(command, "*.go")
		default:
			formatSrc := true
			for _, dirToExclude := range exclusions {
				if isParentDir(src, dirToExclude) {
					formatSrc = false
					break
				}
			}
			if formatSrc {
				command = append(command, fmt.Sprintf("%s/*.go", src))
			}
		}
	}
	return RunCommand(a, strings.Join(command, " "))
}

// Generate runs the 'go generate' tool
func Generate(a *goyek.A) bool {
	printIt("running go generate")
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
			if isParentDir(dir, dirToExclude) {
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

func isParentDir(targetDir, possibleParent string) bool {
	if !strings.HasPrefix(targetDir, possibleParent) {
		return false
	}
	if targetDir == possibleParent {
		return true
	}
	if strings.HasSuffix(possibleParent, "/") {
		return true
	}
	remainder := strings.TrimPrefix(targetDir, possibleParent)
	return strings.HasPrefix(remainder, "/")
}

func matchModuleFile(name string) bool {
	return name == "go.mod"
}

func matchGoSource(name string) bool {
	return endsIn(name, ".go") && !endsIn(name, "_test.go") && !startsWith(name, "testing")
}

func matchAnyGoFile(name string) bool {
	return endsIn(name, ".go")
}

func printBuffer(b *bytes.Buffer) {
	s := eatTrailingEOL(b.String())
	if s != "" {
		printIt(s)
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
	printIt("installing the latest version of", packageName)
	return RunCommand(a, fmt.Sprintf("go install -v %s@latest", packageName))
}

// Lint runs lint on the source code after making sure that the lint tool is up-to-date;
// returns false on failure
func Lint(a *goyek.A) bool {
	if !Install(a, "github.com/go-critic/go-critic/cmd/gocritic") {
		return false
	}
	printIt("linting source code")
	return RunCommand(a, "gocritic check -enableAll ./...")
}

// NilAway runs the nilaway tool, which attempts, via static analysis, to detect
// potential nil access errors; returns false on errors
func NilAway(a *goyek.A) bool {
	if !Install(a, "go.uber.org/nilaway/cmd/nilaway") {
		return false
	}
	printIt("running nilaway analysis")
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
	printIt("running all unit tests")
	return RunCommand(a, "go test -cover ./...")
}

type envVar struct {
	name  string
	value string
	unset bool
}

type directedCommand struct {
	command string
	dir     string
	envVars []envVar
}

// UpdateDependencies updates module dependencies and prunes the modified go.mod
// and go.sum files
func UpdateDependencies(a *goyek.A) bool {
	dirs, err := relevantDirs(matchModuleFile)
	if err != nil {
		return false
	}
	getCommand := directedCommand{command: "go get -u ./..."}
	if *aggressive {
		getCommand.envVars = append(getCommand.envVars, envVar{
			name:  "GOPROXY",
			value: "direct",
			unset: false,
		})
	}
	tidyCommand := directedCommand{command: "go mod tidy"}
	for _, dir := range dirs {
		path := filepath.Join(WorkingDir(), dir)
		getCommand.dir = path
		tidyCommand.dir = path
		fmt.Printf("%q: updating dependencies\n", path)
		if !getCommand.execute(a) {
			return false
		}
		fmt.Printf("%q: pruning go.mod and go.sum\n", path)
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
	savedEnvVars, envVarsOK := setupEnvVars(dC.envVars)
	state := envVarsOK
	if state {
		defer restoreEnvVars(savedEnvVars)
		state = executor(a, dC.command, options...)
	}
	return state
}

func printRestoration(v envVar) {
	if v.unset {
		printIt("restoring (unsetting):", v.name)
	} else {
		printIt("restoring (resetting):", v.name, "<-", v.value)
	}
}

func restoreEnvVars(saved []envVar) {
	for _, v := range saved {
		printRestoration(v)
		if v.unset {
			_ = unsetenv(v.name)
		} else {
			_ = setenv(v.name, v.value)
		}
	}
}

func checkEnvVars(input []envVar) bool {
	if len(input) == 0 {
		return true
	}
	distinctVar := map[string]bool{}
	for _, v := range input {
		if distinctVar[v.name] {
			printIt("code error: detected attempt to set environment variable", v.name, "twice")
			return false
		}
		distinctVar[v.name] = true
	}
	return true
}

func printFormerEnvVarState(name, value string, defined bool) {
	if defined {
		printIt(name, "was set to", value)
	} else {
		printIt(name, "was not set")
	}
}

func setupEnvVars(input []envVar) ([]envVar, bool) {
	if !checkEnvVars(input) {
		return nil, false
	}
	savedEnvVars := make([]envVar, 0)
	for _, envVariable := range input {
		oldValue, defined := os.LookupEnv(envVariable.name)
		printFormerEnvVarState(envVariable.name, oldValue, defined)
		savedEnvVars = append(savedEnvVars, envVar{
			name:  envVariable.name,
			value: oldValue,
			unset: !defined,
		})
		if envVariable.unset {
			printIt("unsetting", envVariable.name)
			_ = unsetenv(envVariable.name)
		} else {
			printIt("setting", envVariable.name, "to", envVariable.value)
			_ = setenv(envVariable.name, envVariable.value)
		}
	}
	return savedEnvVars, true
}

// VulnerabilityCheck runs the govulncheck tool, which checks for unresolved
// known vulnerabilities in the libraries used; returns false on failure
func VulnerabilityCheck(a *goyek.A) bool {
	if !Install(a, "golang.org/x/vuln/cmd/govulncheck") {
		return false
	}
	printIt("running vulnerability checks")
	return RunCommand(a, "govulncheck -show verbose ./...")
}

// WorkingDir returns a 'best' guess of the working directory. If the directory
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
		_, _ = fmt.Fprintln(os.Stderr, "code error: empty candidate value passed to isAcceptableWorkingDir")
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
	sourceDirectories := make([]string, 0)
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

func printIt(a ...any) {
	_, _ = printLine(a...)
}
