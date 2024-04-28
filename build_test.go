package tools_build

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
	"github.com/spf13/afero"
)

func TestClean(t *testing.T) {
	originalFileSystem := fileSystem
	originalWorkingDir := workingDir
	originalExit := exit
	defer func() {
		fileSystem = originalFileSystem
		workingDir = originalWorkingDir
		exit = originalExit
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("a/b/c", 0o755)
	workingDir = "a/b/c"
	afero.WriteFile(fileSystem, "a/b/c/myFile", []byte("foo"), 0o644)
	afero.WriteFile(fileSystem, "a/b/c/myOtherFile", []byte(""), 0o644)
	tests := map[string]struct {
		files          []string
		wantExitCalled bool
	}{
		"no files":     {files: nil},
		"no problems":  {files: []string{"myFile", "myOtherFile", "myNonExistentFile"}},
		"illegal path": {files: []string{"foo/../../bar"}, wantExitCalled: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotExitCalled := false
			exit = func(_ int) {
				gotExitCalled = true
			}
			Clean(tt.files)
			for _, file := range tt.files {
				f := filepath.Join(workingDir, file)
				if exists, err := afero.Exists(fileSystem, f); err != nil {
					t.Errorf("Clean: something went wrong verifying the non-existence of %q: %v", f, err)
				} else if exists {
					t.Errorf("Clean failed to delete %q", f)
				}
			}
			if gotExitCalled != tt.wantExitCalled {
				t.Errorf("Clean exit called %t, want %t", gotExitCalled, tt.wantExitCalled)
			}
		})
	}
}

func Test_isAcceptableWorkingDir(t *testing.T) {
	originalFileSystem := fileSystem
	defer func() {
		fileSystem = originalFileSystem
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("successful/.git", 0o755)
	fileSystem.Mkdir("empty", 0o755)
	fileSystem.Mkdir("defective", 0o755)
	afero.WriteFile(fileSystem, filepath.Join("defective", ".git"), []byte("data"), 0o644)
	afero.WriteFile(fileSystem, "not a directory", []byte("gibberish"), 0o644)
	tests := map[string]struct {
		candidate string
		wantErr   bool
	}{
		"empty string":            {candidate: "", wantErr: true},
		"non-existent":            {candidate: "no such file", wantErr: true},
		"not a dir":               {candidate: "not a directory", wantErr: true},
		"no .git":                 {candidate: "empty", wantErr: true},
		".git is not a directory": {candidate: "defective", wantErr: true},
		"happy path":              {candidate: "successful", wantErr: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := isAcceptableWorkingDir(tt.candidate); (err != nil) != tt.wantErr {
				t.Errorf("isAcceptableWorkingDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkingDir(t *testing.T) {
	originalFileSystem := fileSystem
	originalExit := exit
	originalDir, originalOk := os.LookupEnv("DIR")
	originalWorkingDir := workingDir
	defer func() {
		fileSystem = originalFileSystem
		exit = originalExit
		if originalOk {
			os.Setenv("DIR", originalDir)
		} else {
			os.Unsetenv("DIR")
		}
		workingDir = originalWorkingDir
	}()
	recordedCode := 0
	exit = func(code int) {
		recordedCode = code
	}
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll(filepath.Join("..", ".git"), 0o755)
	fileSystem.MkdirAll(filepath.Join("happy", ".git"), 0o755)
	tests := map[string]struct {
		workDir     string
		dirFromEnv  bool
		dirEnvValue string
		want        string
		wantCode    int
	}{
		"saved":   {workDir: "foo", want: "foo"},
		"no env":  {want: ".."},
		"env":     {dirFromEnv: true, dirEnvValue: "happy", want: "happy"},
		"bad env": {dirFromEnv: true, dirEnvValue: "", wantCode: 1},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			recordedCode = 0
			workingDir = tt.workDir
			if tt.dirFromEnv {
				os.Setenv("DIR", tt.dirEnvValue)
			} else {
				os.Unsetenv("DIR")
			}
			if got := WorkingDir(); got != tt.want {
				t.Errorf("WorkingDir() = %v, want %v", got, tt.want)
			}
			if recordedCode != tt.wantCode {
				t.Errorf("WorkingDir() = %d, want %d", recordedCode, tt.wantCode)
			}
		})
	}
}

func TestMakeCmdOptions(t *testing.T) {
	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
	}()
	workingDir = "work"
	stdBuffer := &bytes.Buffer{}
	tests := map[string]struct {
		b     *bytes.Buffer
		wantO []cmd.Option
	}{
		"typical": {
			b: stdBuffer,
			wantO: []cmd.Option{
				cmd.Dir("work"),
				cmd.Stderr(stdBuffer),
				cmd.Stdout(stdBuffer),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if gotO := MakeCmdOptions(tt.b); len(gotO) != len(tt.wantO) {
				t.Errorf("MakeCmdOptions() = %d, want %d", len(gotO), len(tt.wantO))
			}
		})
	}
}

func TestRunCommand(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	workingDir = "work"
	type args struct {
		a       *goyek.A
		command string
	}
	tests := map[string]struct {
		args
		shouldSucceed bool
		want          bool
	}{
		"fail":    {args: args{}},
		"succeed": {args: args{}, shouldSucceed: true, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			executor = func(_ *goyek.A, _ string, _ ...cmd.Option) bool {
				return tt.shouldSucceed
			}
			if got := RunCommand(tt.args.a, tt.args.command); got != tt.want {
				t.Errorf("RunCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	workingDir = "work"
	tests := map[string]struct {
		shouldSucceed bool
		want          bool
	}{
		"fail":    {},
		"succeed": {shouldSucceed: true, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var gotCmd string
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := Format(nil); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
			if gotCmd != "gofmt -e -l -s -w ." {
				t.Errorf("Format() = %q, want %q", gotCmd, "gofmt -e -l -s -w .")
			}
		})
	}
}

func TestGenerateCoverageReport(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	workingDir = "work"
	tests := map[string]struct {
		file               string
		unitTestsSucceed   bool
		displaySucceeds    bool
		wantDisplayCalled  bool
		wantTestCommand    string
		wantDisplayCommand string
		want               bool
	}{
		"tests fail": {
			file:            "coverage.txt",
			wantTestCommand: "go test -coverprofile=coverage.txt ./...",
		},
		"tests succeed, display fails": {
			file:               "coverage.txt",
			unitTestsSucceed:   true,
			wantDisplayCalled:  true,
			wantTestCommand:    "go test -coverprofile=coverage.txt ./...",
			wantDisplayCommand: "go tool cover -html=coverage.txt",
		},
		"success": {
			file:               "coverage.txt",
			unitTestsSucceed:   true,
			displaySucceeds:    true,
			wantDisplayCalled:  true,
			wantTestCommand:    "go test -coverprofile=coverage.txt ./...",
			wantDisplayCommand: "go tool cover -html=coverage.txt",
			want:               true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var gotDisplayCalled = false
			var gotTestCommand string
			var gotDisplayCommand string
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				switch {
				case strings.HasPrefix(cmd, "go test "):
					gotTestCommand = cmd
					return tt.unitTestsSucceed
				case strings.HasPrefix(cmd, "go tool "):
					gotDisplayCommand = cmd
					gotDisplayCalled = true
					return tt.displaySucceeds
				default:
					t.Errorf("GenerateCoverageReport() invoked command %q", cmd)
					return false
				}
			}
			if got := GenerateCoverageReport(nil, tt.file); got != tt.want {
				t.Errorf("GenerateCoverageReport() = %v, want %v", got, tt.want)
			}
			if gotDisplayCalled != tt.wantDisplayCalled {
				t.Errorf("GenerateCoverageReport() display called: %t, want %t", gotDisplayCalled, tt.wantDisplayCalled)
			}
			if gotDisplayCommand != tt.wantDisplayCommand {
				t.Errorf("GenerateCoverageReport() got display command %q, want %q", gotDisplayCommand, tt.wantDisplayCommand)
			}
			if gotTestCommand != tt.wantTestCommand {
				t.Errorf("GenerateCoverageReport() got test command %q, want %q", gotTestCommand, tt.wantTestCommand)
			}
		})
	}
}

func Test_allDirs(t *testing.T) {
	originalFileSystem := fileSystem
	defer func() {
		fileSystem = originalFileSystem
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("a/b/c", 0o755)
	fileSystem.Mkdir("a/b/c/d", 0o755)
	fileSystem.Mkdir("a/b/c/e", 0o755)
	afero.WriteFile(fileSystem, "a/b/c/f", []byte("data"), 0o644)
	afero.WriteFile(fileSystem, "a/b/c/e/x", []byte("data"), 0o644)
	tests := map[string]struct {
		top     string
		want    []string
		wantErr bool
	}{
		"error":   {top: "no such dir", want: nil, wantErr: true},
		"success": {top: "a", want: []string{"a", "a/b", "a/b/c", "a/b/c/d", "a/b/c/e"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := allDirs(tt.top)
			if (err != nil) != tt.wantErr {
				t.Errorf("allDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("allDirs() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testFileMode struct {
	name string
	mode fs.FileMode
}

func (tfm testFileMode) Name() string {
	return tfm.name
}

func (tfm testFileMode) Size() int64 {
	return 0
}

func (tfm testFileMode) Mode() fs.FileMode {
	return tfm.mode
}

func (tfm testFileMode) ModTime() time.Time {
	return time.Now()
}

func (tfm testFileMode) IsDir() bool {
	return tfm.Mode().IsDir()
}

func (tfm testFileMode) Sys() any {
	return nil
}

func Test_isGoSourceFile(t *testing.T) {
	tests := map[string]struct {
		entry fs.FileInfo
		want  bool
	}{
		"dir":     {entry: testFileMode{name: "dir.go", mode: fs.ModeDir}},
		"foo":     {entry: testFileMode{name: "foo", mode: 0}},
		"test1":   {entry: testFileMode{name: "t_test.go", mode: 0}},
		"test2":   {entry: testFileMode{name: "testing_foo.go", mode: 0}},
		"success": {entry: testFileMode{name: "foo.go", mode: 0}, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isGoSourceFile(tt.entry); got != tt.want {
				t.Errorf("isGoSourceFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_includesGoSource(t *testing.T) {
	originalFileSystem := fileSystem
	defer func() {
		fileSystem = originalFileSystem
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("a/b/c", 0o755)
	afero.WriteFile(fileSystem, "a/foo_test.go", []byte("test stuff"), 0o644)
	afero.WriteFile(fileSystem, "a/b/foo.go", []byte("source code"), 0o644)
	tests := map[string]struct {
		entries []fs.FileInfo
		dir     string
		want    bool
	}{
		"no dir":      {dir: "no such dir"},
		"no source":   {dir: "a"},
		"with source": {dir: "a/b", want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := includesGoSource(tt.dir); got != tt.want {
				t.Errorf("includesGoSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sourceDirs(t *testing.T) {
	originalWorkingDir := workingDir
	originalFileSystem := fileSystem
	defer func() {
		workingDir = originalWorkingDir
		fileSystem = originalFileSystem
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("a/b/c", 0o755)
	afero.WriteFile(fileSystem, "a/foo_test.go", []byte("test stuff"), 0o644)
	afero.WriteFile(fileSystem, "a/b/foo.go", []byte("source code"), 0o644)
	tests := map[string]struct {
		workDir string
		want    []string
		wantErr bool
	}{
		"error": {workDir: "no such dir", wantErr: true},
		"a":     {workDir: "a", want: []string{"b"}},
		"b":     {workDir: "a/b", want: []string{""}},
		"c":     {workDir: "a/b/c", want: []string{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			workingDir = tt.workDir
			got, err := sourceDirs()
			if (err != nil) != tt.wantErr {
				t.Errorf("sourceDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sourceDirs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateDocumentation(t *testing.T) {
	originalWorkingDir := workingDir
	originalFileSystem := fileSystem
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		fileSystem = originalFileSystem
		executor = originalExecutor
	}()
	fileSystem = afero.NewMemMapFs()
	fileSystem.MkdirAll("a/b/c", 0o755)
	afero.WriteFile(fileSystem, "a/foo_test.go", []byte("test stuff"), 0o644)
	afero.WriteFile(fileSystem, "a/b/foo.go", []byte("source code"), 0o644)
	type args struct {
		a            *goyek.A
		excludedDirs []string
	}
	tests := map[string]struct {
		args
		workDir             string
		wantExecutorSuccess bool
		wantCommands        []string
		want                bool
	}{
		"error": {workDir: "no such dir", wantCommands: []string{}, want: false},
		"a": {
			workDir:             "a",
			wantExecutorSuccess: true,
			wantCommands:        []string{"go doc -all ./b"},
			want:                true,
		},
		"a - exclude b": {
			args:                args{excludedDirs: []string{"b"}},
			workDir:             "a",
			wantExecutorSuccess: true,
			wantCommands:        []string{},
			want:                true,
		},
		"b": {
			workDir:             "a/b",
			wantExecutorSuccess: true,
			wantCommands:        []string{"go doc -all ./"},
			want:                true,
		},
		"c": {
			workDir:             "a/b/c",
			wantExecutorSuccess: true,
			wantCommands:        []string{},
			want:                true,
		},
		"cmd fail": {
			workDir:             "a",
			wantExecutorSuccess: false,
			wantCommands:        []string{"go doc -all ./b"},
			want:                false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			workingDir = tt.workDir
			gotCommands := []string{}
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				return tt.wantExecutorSuccess
			}
			if got := GenerateDocumentation(tt.args.a, tt.args.excludedDirs); got != tt.want {
				t.Errorf("GenerateDocumentation() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("GenerateDocumentation() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	// not used, and keeps WorkingDir() from getting exercised
	workingDir = "work"
	type args struct {
		a           *goyek.A
		packageName string
	}
	tests := map[string]struct {
		args
		installSucceeds bool
		wantCommand     string
		want            bool
	}{
		"failure": {
			args:            args{packageName: "foo/bar"},
			installSucceeds: false,
			wantCommand:     "go install -v foo/bar@latest",
			want:            false,
		},
		"success": {
			args:            args{packageName: "foo/bar/baz"},
			installSucceeds: true,
			wantCommand:     "go install -v foo/bar/baz@latest",
			want:            true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommand := ""
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommand = cmd
				return tt.installSucceeds
			}
			if got := Install(tt.args.a, tt.args.packageName); got != tt.want {
				t.Errorf("Install() = %v, want %v", got, tt.want)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("Install() command = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}

func TestLint(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	// not used, and keeps WorkingDir() from getting exercised
	workingDir = "work"
	tests := map[string]struct {
		installSucceeds bool
		lintSucceeds    bool
		wantCommands    []string
		want            bool
	}{
		"install fails": {
			installSucceeds: false,
			wantCommands: []string{
				"go install -v github.com/go-critic/go-critic/cmd/gocritic@latest",
			},
			want: false,
		},
		"lint fails": {
			installSucceeds: true,
			lintSucceeds:    false,
			wantCommands: []string{
				"go install -v github.com/go-critic/go-critic/cmd/gocritic@latest",
				"gocritic check -enableAll ./...",
			},
			want: false,
		},
		"lint succeeds": {
			installSucceeds: true,
			lintSucceeds:    true,
			wantCommands: []string{
				"go install -v github.com/go-critic/go-critic/cmd/gocritic@latest",
				"gocritic check -enableAll ./...",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommands := []string{}
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				if strings.Contains(cmd, " install ") {
					return tt.installSucceeds
				}
				if strings.HasPrefix(cmd, "gocritic check ") {
					return tt.lintSucceeds
				}
				t.Errorf("Lint() sent unexpected command: %q", cmd)
				return false
			}
			if got := Lint(nil); got != tt.want {
				t.Errorf("Lint() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("Lint() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestNilAway(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	// not used, and keeps WorkingDir() from getting exercised
	workingDir = "work"
	tests := map[string]struct {
		installSucceeds bool
		nilawaySucceeds bool
		wantCommands    []string
		want            bool
	}{
		"install fails": {
			installSucceeds: false,
			wantCommands: []string{
				"go install -v go.uber.org/nilaway/cmd/nilaway@latest",
			},
			want: false,
		},
		"nilaway fails": {
			installSucceeds: true,
			nilawaySucceeds: false,
			wantCommands: []string{
				"go install -v go.uber.org/nilaway/cmd/nilaway@latest",
				"nilaway ./...",
			},
			want: false,
		},
		"nilaway succeeds": {
			installSucceeds: true,
			nilawaySucceeds: true,
			wantCommands: []string{
				"go install -v go.uber.org/nilaway/cmd/nilaway@latest",
				"nilaway ./...",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommands := []string{}
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				if strings.Contains(cmd, " install ") {
					return tt.installSucceeds
				}
				if strings.HasPrefix(cmd, "nilaway ") {
					return tt.nilawaySucceeds
				}
				t.Errorf("NilAway() sent unexpected command: %q", cmd)
				return false
			}
			if got := NilAway(nil); got != tt.want {
				t.Errorf("NilAway() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("NilAway() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestUnitTests(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	workingDir = "work"
	tests := map[string]struct {
		shouldSucceed bool
		want          bool
	}{
		"fail":    {},
		"succeed": {shouldSucceed: true, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var gotCmd string
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := UnitTests(nil); got != tt.want {
				t.Errorf("UnitTests() = %v, want %v", got, tt.want)
			}
			if gotCmd != "go test -cover ./..." {
				t.Errorf("UnitTests() = %q, want %q", gotCmd, "go test -cover ./...")
			}
		})
	}
}

func TestVulnerabilityCheck(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	// not used, and keeps WorkingDir() from getting exercised
	workingDir = "work"
	tests := map[string]struct {
		installSucceeds            bool
		vulnerabilityCheckSucceeds bool
		wantCommands               []string
		want                       bool
	}{
		"install fails": {
			installSucceeds: false,
			wantCommands: []string{
				"go install -v golang.org/x/vuln/cmd/govulncheck@latest",
			},
			want: false,
		},
		"vulnerabilitycheck fails": {
			installSucceeds:            true,
			vulnerabilityCheckSucceeds: false,
			wantCommands: []string{
				"go install -v golang.org/x/vuln/cmd/govulncheck@latest",
				"govulncheck -show verbose ./...",
			},
			want: false,
		},
		"vulnerabilitycheck succeeds": {
			installSucceeds:            true,
			vulnerabilityCheckSucceeds: true,
			wantCommands: []string{
				"go install -v golang.org/x/vuln/cmd/govulncheck@latest",
				"govulncheck -show verbose ./...",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommands := []string{}
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				if strings.Contains(cmd, " install ") {
					return tt.installSucceeds
				}
				if strings.HasPrefix(cmd, "govulncheck ") {
					return tt.vulnerabilityCheckSucceeds
				}
				t.Errorf("VulnerabilityCheck() sent unexpected command: %q", cmd)
				return false
			}
			if got := VulnerabilityCheck(nil); got != tt.want {
				t.Errorf("VulnerabilityCheck() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("VulnerabilityCheck() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func Test_containsBackDir(t *testing.T) {
	tests := map[string]struct {
		f    string
		want bool
	}{
		"empty":     {f: "", want: false},
		"backdir1":  {f: "..", want: true},
		"backdir2":  {f: "../", want: true},
		"backdir3":  {f: "..\\", want: true},
		"backdir4":  {f: "\\..", want: true},
		"backdir5":  {f: "/..", want: true},
		"complex1":  {f: "a/b/c/../e/f/g", want: true},
		"complex2":  {f: "../a/b/c/e/f/g", want: true},
		"harmless1": {f: "a/b..c/d/e/f", want: false},
		"harmless2": {f: "a/b..", want: false},
		"harmless3": {f: "a/..b", want: false},
		"harmless4": {f: "a/b../c", want: false},
		"harmless5": {f: "a/..b/c", want: false},
		"harmless6": {f: "b../c", want: false},
		"harmless7": {f: "..b/c", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := containsBackDir(tt.f); got != tt.want {
				t.Errorf("containsBackDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_printBuffer(t *testing.T) {
	originalPrintLine := printLine
	defer func() {
		printLine = originalPrintLine
	}()
	tests := map[string]struct {
		data      string
		wantPrint bool
	}{
		"empty":     {data: "", wantPrint: false},
		"some data": {data: "123", wantPrint: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotPrint := false
			printLine = func(_ ...any) (int, error) {
				gotPrint = true
				return 0, nil
			}
			buffer := &bytes.Buffer{}
			buffer.WriteString(tt.data)
			printBuffer(buffer)
			if gotPrint != tt.wantPrint {
				t.Errorf("printBuffer got %t, want %t", gotPrint, tt.wantPrint)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	originalWorkingDir := workingDir
	originalExecutor := executor
	defer func() {
		workingDir = originalWorkingDir
		executor = originalExecutor
	}()
	workingDir = "work"
	tests := map[string]struct {
		shouldSucceed bool
		want          bool
	}{
		"fail":    {},
		"succeed": {shouldSucceed: true, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var gotCmd string
			executor = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := Generate(nil); got != tt.want {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
			if gotCmd != "go generate -v ./..." {
				t.Errorf("Generate() = %q, want %q", gotCmd, "go generate -v ./...")
			}
		})
	}
}
