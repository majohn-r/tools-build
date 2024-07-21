package tools_build

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
	"github.com/spf13/afero"
	"os"
	"path/filepath"
	"strings"
)

var (
	// AggressiveFlag is a flag for the UpdateDependencies function to more aggressively get updates
	AggressiveFlag = flag.Bool(
		"aggressive",
		false,
		"set to make dependency updates more aggressive",
	)
	// NoFormatFlag is a flag to disable formatting from the deadcode command
	NoFormatFlag = flag.Bool(
		"noformat",
		false,
		"set to remove custom formatting from dead code analysis")
	// NoTestFlag is a flag for the AnalyzeDeadCode function to remove the -test parameter from the deadcode command
	NoTestFlag = flag.Bool(
		"notype",
		false,
		"set to remove the -test parameter from dead code analysis")
	// TemplateFlag is a flag to allow the caller to change the format template used by the deadcode command
	TemplateFlag = flag.String(
		"template",
		`{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
		"set to change the template used to format dead code analysis (ignored if -noformat is true)")
	// ExecFn is the goyek Exec function. set as a variable so that unit tests can override
	ExecFn = cmd.Exec
	// ExitFn is the os.Exit function, set as a variable so that unit tests can override
	ExitFn = os.Exit
)

// Deadcode runs dead code analysis on the source code after making sure that the deadcode tool is
// up-to-date; returns false on failure
func Deadcode(a *goyek.A) bool {
	if !Install(a, "golang.org/x/tools/cmd/deadcode") {
		return false
	}
	// assemble command line
	// deadcode -f='{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}'  -test .
	cmdParts := make([]string, 0)
	cmdParts = append(cmdParts, "deadcode")
	if !*NoFormatFlag {
		cmdParts = append(cmdParts, fmt.Sprintf("-f='%s'", *TemplateFlag))
	}
	if !*NoTestFlag {
		cmdParts = append(cmdParts, "-test")
	}
	cmdParts = append(cmdParts, ".")
	printIt("running dead code analysis")
	return RunCommand(a, strings.Join(cmdParts, " "))
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

// VulnerabilityCheck runs the govulncheck tool, which checks for unresolved
// known vulnerabilities in the libraries used; returns false on failure
func VulnerabilityCheck(a *goyek.A) bool {
	if !Install(a, "golang.org/x/vuln/cmd/govulncheck") {
		return false
	}
	printIt("running vulnerability checks")
	return RunCommand(a, "govulncheck -show verbose ./...")
}

type directedCommand struct {
	command string
	dir     string
	envVars []EnvVarMemento
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
