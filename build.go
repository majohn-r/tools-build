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
	// BuildFS is the file system used; accessible so that tests can override it
	BuildFS = afero.NewOsFs()
	// CachedWorkingDir is the cached working directory
	CachedWorkingDir = ""
	// AggressiveFlag is a flag for the UpdateDependencies function to more aggressively get updates
	AggressiveFlag = flag.Bool("aggressive", false, "set to make dependency updates more aggressive")
	// ExecFn is the goyek Exec function. set as a variable so that unit tests can override
	ExecFn = cmd.Exec
	// ExitFn is the os.Exit function, set as a variable so that unit tests can override
	ExitFn = os.Exit
	// PrintlnFn is the fmt.Println function, set as a variable so that unit tests can override
	PrintlnFn = fmt.Println
	// SetenvFn is the os.Setenv function, set as a variable so that unit tests can override
	SetenvFn = os.Setenv
	// UnsetenvFn is the os.Unsetenv function, set as a variable so that unit tests can override
	UnsetenvFn = os.Unsetenv
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
			ExitFn(1)
		}
		openFile, err := workingFS.Open(file)
		if err == nil {
			_ = openFile.Close()
			_ = BuildFS.Remove(filepath.Join(WorkingDir(), file))
		}
	}
}

func isIllegalFileName(path string) bool {
	return path == "" || IsMalformedFileName(path)
}

// IsMalformedFileName determines whether a file name is malformed, such that it could be used to access a file outside
// the working directory (starts with '/' or '\\', or contains a path component of '..'
func IsMalformedFileName(path string) bool {
	if path == "" {
		return false
	}
	if startsWith(path, "/") || startsWith(path, "\\") {
		return true
	}
	dir, file := filepath.Split(canonicalPath(path))
	if file == ".." {
		return true
	}
	if dir == path && file == "" {
		// happens when the original path begins with a drive letter and colon
		return true
	}
	return IsMalformedFileName(strings.TrimSuffix(dir, "/"))
}

func canonicalPath(path string) string {
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
	srcDirs, err := RelevantDirs(matchAnyGoFile)
	if err != nil {
		return false
	}
	command := []string{"gofmt -e -l -s -w"}
	for _, src := range srcDirs {
		switch src {
		case "":
			entries, _ := afero.ReadDir(BuildFS, WorkingDir())
			for _, entry := range entries {
				if !entry.IsDir() && matchAnyGoFile(entry.Name()) {
					command = append(command, entry.Name())
				}
			}
		default:
			formatSrc := true
			for _, dirToExclude := range exclusions {
				if isParentDir(src, dirToExclude) {
					formatSrc = false
					break
				}
			}
			if formatSrc {
				command = append(command, src)
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
	dirs, err := RelevantDirs(MatchGoSource)
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
	PrintBuffer(o)
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

// MatchGoSource matches a file name ending in '.go', but does not match test files.
func MatchGoSource(name string) bool {
	return endsIn(name, ".go") && !endsIn(name, "_test.go") && !startsWith(name, "testing")
}

func matchAnyGoFile(name string) bool {
	return endsIn(name, ".go")
}

// PrintBuffer sends the buffer contents to stdout, but first strips trailing EOL characters, and then only prints the
// remaining content if that content is not empty
func PrintBuffer(b *bytes.Buffer) {
	s := EatTrailingEOL(b.String())
	if s != "" {
		printIt(s)
	}
}

// EatTrailingEOL removes trailing \n and \r characters from the end of a string; recurses.
func EatTrailingEOL(s string) string {
	switch {
	case strings.HasSuffix(s, "\n"):
		return EatTrailingEOL(strings.TrimSuffix(s, "\n"))
	case strings.HasSuffix(s, "\r"):
		return EatTrailingEOL(strings.TrimSuffix(s, "\r"))
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

// EnvVarMemento captures an environment variable's desired state
type EnvVarMemento struct {
	// Name is the variable's name
	Name string
	// Value is what the variable should be set to
	Value string
	// Unset, if true, means the variable should be unset
	Unset bool
}

type directedCommand struct {
	command string
	dir     string
	envVars []EnvVarMemento
}

// UpdateDependencies updates module dependencies and prunes the modified go.mod
// and go.sum files
func UpdateDependencies(a *goyek.A) bool {
	dirs, err := RelevantDirs(matchModuleFile)
	if err != nil {
		return false
	}
	getCommand := directedCommand{command: "go get -u ./..."}
	if *AggressiveFlag {
		getCommand.envVars = append(getCommand.envVars, EnvVarMemento{
			Name:  "GOPROXY",
			Value: "direct",
			Unset: false,
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
	defer PrintBuffer(outputBuffer)
	options := make([]cmd.Option, 3)
	options[0] = cmd.Dir(dC.dir)
	options[1] = cmd.Stderr(outputBuffer)
	options[2] = cmd.Stdout(outputBuffer)
	savedEnvVars, envVarsOK := SetupEnvVars(dC.envVars)
	state := envVarsOK
	if state {
		defer RestoreEnvVars(savedEnvVars)
		state = ExecFn(a, dC.command, options...)
	}
	return state
}

func printRestoration(v EnvVarMemento) {
	if v.Unset {
		printIt("restoring (unsetting):", v.Name)
	} else {
		printIt("restoring (resetting):", v.Name, "<-", v.Value)
	}
}

// RestoreEnvVars reverts the environment changes made by SetupEnvVars
func RestoreEnvVars(saved []EnvVarMemento) {
	for _, v := range saved {
		printRestoration(v)
		if v.Unset {
			_ = UnsetenvFn(v.Name)
		} else {
			_ = SetenvFn(v.Name, v.Value)
		}
	}
}

func checkEnvVars(input []EnvVarMemento) bool {
	if len(input) == 0 {
		return true
	}
	distinctVar := map[string]bool{}
	for _, v := range input {
		if distinctVar[v.Name] {
			printIt("code error: detected attempt to set environment variable", v.Name, "twice")
			return false
		}
		distinctVar[v.Name] = true
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

// SetupEnvVars executes the intent of the provided slice of EnvVarMementos, and returns a slice to be executed to
// revert the directed changes
func SetupEnvVars(input []EnvVarMemento) ([]EnvVarMemento, bool) {
	if !checkEnvVars(input) {
		return nil, false
	}
	savedEnvVars := make([]EnvVarMemento, 0)
	for _, envVariable := range input {
		oldValue, defined := os.LookupEnv(envVariable.Name)
		printFormerEnvVarState(envVariable.Name, oldValue, defined)
		savedEnvVars = append(savedEnvVars, EnvVarMemento{
			Name:  envVariable.Name,
			Value: oldValue,
			Unset: !defined,
		})
		if envVariable.Unset {
			printIt("unsetting", envVariable.Name)
			_ = UnsetenvFn(envVariable.Name)
		} else {
			printIt("setting", envVariable.Name, "to", envVariable.Value)
			_ = SetenvFn(envVariable.Name, envVariable.Value)
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
	if CachedWorkingDir == "" {
		candidate := ".."
		if dirValue, dirExists := os.LookupEnv("DIR"); dirExists {
			candidate = dirValue
		}
		if UnacceptableWorkingDir(candidate) {
			ExitFn(1)
		}
		// ok, it's acceptable
		CachedWorkingDir = candidate
	}
	return CachedWorkingDir
}

// UnacceptableWorkingDir determines whether a specified candidate directory could be the working directory for the
// build. The candidate cannot be empty, must be a valid directory, and must contain a valid subdirectory named '.git'
func UnacceptableWorkingDir(candidate string) bool {
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
	pathIsConfirmedDir, err := afero.IsDir(BuildFS, path)
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

// AllDirs returns all directories in the directory specified by the top parameter, including that directory. Recurses.
func AllDirs(top string) ([]string, error) {
	var topIsDir bool
	var err error
	if topIsDir, err = afero.IsDir(BuildFS, top); err != nil {
		return nil, err
	}
	if !topIsDir {
		return nil, fmt.Errorf("%q is not a directory", top)
	}
	top = canonicalPath(top)
	entries, _ := afero.ReadDir(BuildFS, top)
	dirs := []string{top}
	for _, entry := range entries {
		if entry.IsDir() {
			subDirs, _ := AllDirs(filepath.Join(top, entry.Name()))
			dirs = append(dirs, subDirs...)
		}
	}
	return dirs, nil
}

// RelevantDirs returns the directories that contain files matching the provided fileMatcher
func RelevantDirs(fileMatcher func(string) bool) ([]string, error) {
	topDir := WorkingDir()
	dirs, err := AllDirs(topDir)
	if err != nil {
		return nil, err
	}
	sourceDirectories := make([]string, 0)
	for _, dir := range dirs {
		if IncludesRelevantFiles(dir, fileMatcher) {
			sourceDir := strings.TrimPrefix(strings.TrimPrefix(dir, topDir), "/")
			sourceDirectories = append(sourceDirectories, sourceDir)
		}
	}
	return sourceDirectories, nil
}

// IncludesRelevantFiles returns true if the provided directory contains any regular files whose names conform to the
// fileMatcher
func IncludesRelevantFiles(dir string, fileMatcher func(string) bool) bool {
	entries, err := afero.ReadDir(BuildFS, dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if IsRelevantFile(e, fileMatcher) {
			return true
		}
	}
	return false
}

// IsRelevantFile returns true if the entry is a file and its name is validated by the provided fileMatcher
func IsRelevantFile(entry fs.FileInfo, fileMatcher func(string) bool) bool {
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
	_, _ = PrintlnFn(a...)
}
