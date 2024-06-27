package tools_build_test

import (
	"bytes"
	build "github.com/majohn-r/tools-build"
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

const (
	dirMode  = 0o755
	fileMode = 0o644
)

func TestClean(t *testing.T) {
	// note: cannot use memory mapped filesystem; Clean relies on using the os
	// filesystem to make sure all files are within the working directory
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExitFn := build.ExitFn
	build.CachedWorkingDir = "a/b/c"
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExitFn = originalExitFn
		_ = build.BuildFS.RemoveAll("a")
	}()
	_ = build.BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/c/myFile", []byte("foo"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/c/myOtherFile", []byte(""), fileMode)
	tests := map[string]struct {
		files          []string
		wantExitCalled bool
	}{
		"no files": {
			files:          nil,
			wantExitCalled: false,
		},
		"empty file": {
			files:          []string{""},
			wantExitCalled: true,
		},
		"file with drive letter": {
			files:          []string{"c:\\myNonExistentFile"},
			wantExitCalled: true,
		},
		"no problems": {
			files:          []string{"myFile", "myOtherFile"},
			wantExitCalled: false,
		},
		"illegal path": {
			files:          []string{"foo/../../bar"},
			wantExitCalled: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotExitCalled := false
			build.ExitFn = func(_ int) {
				gotExitCalled = true
			}
			build.Clean(tt.files)
			if gotExitCalled != tt.wantExitCalled {
				t.Errorf("Clean exit called %t, want %t", gotExitCalled, tt.wantExitCalled)
			}
			if !gotExitCalled {
				for _, file := range tt.files {
					f := filepath.Join(build.CachedWorkingDir, file)
					if fileExists, _ := afero.Exists(build.BuildFS, f); fileExists {
						t.Errorf("Clean failed to delete %q", f)
					}
				}
			}
		})
	}
}

func TestUnacceptableWorkingDir(t *testing.T) {
	originalBuildFS := build.BuildFS
	defer func() {
		build.BuildFS = originalBuildFS
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("successful/.git", dirMode)
	_ = build.BuildFS.Mkdir("empty", dirMode)
	_ = build.BuildFS.Mkdir("defective", dirMode)
	_ = afero.WriteFile(build.BuildFS, filepath.Join("defective", ".git"), []byte("data"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "not a directory", []byte("gibberish"), fileMode)
	tests := map[string]struct {
		candidate string
		want      bool
	}{
		"empty string":            {candidate: "", want: true},
		"non-existent":            {candidate: "no such file", want: true},
		"not a dir":               {candidate: "not a directory", want: true},
		"no .git":                 {candidate: "empty", want: true},
		".git is not a directory": {candidate: "defective", want: true},
		"happy path":              {candidate: "successful", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := build.UnacceptableWorkingDir(tt.candidate); got != tt.want {
				t.Errorf("UnacceptableWorkingDir() %t, want %t", got, tt.want)
			}
		})
	}
}

func TestWorkingDir(t *testing.T) {
	originalBuildFS := build.BuildFS
	originalExitFn := build.ExitFn
	originalDirValue, originalDirExists := os.LookupEnv("DIR")
	originalCachedWorkingDir := build.CachedWorkingDir
	defer func() {
		build.BuildFS = originalBuildFS
		build.ExitFn = originalExitFn
		if originalDirExists {
			_ = os.Setenv("DIR", originalDirValue)
		} else {
			_ = os.Unsetenv("DIR")
		}
		build.CachedWorkingDir = originalCachedWorkingDir
	}()
	recordedCode := 0
	build.ExitFn = func(code int) {
		recordedCode = code
	}
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll(filepath.Join("..", ".git"), dirMode)
	_ = build.BuildFS.MkdirAll(filepath.Join("happy", ".git"), dirMode)
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
			build.CachedWorkingDir = tt.workDir
			if tt.dirFromEnv {
				_ = os.Setenv("DIR", tt.dirEnvValue)
			} else {
				_ = os.Unsetenv("DIR")
			}
			if got := build.WorkingDir(); got != tt.want {
				t.Errorf("WorkingDir() = %v, want %v", got, tt.want)
			}
			if recordedCode != tt.wantCode {
				t.Errorf("WorkingDir() = %d, want %d", recordedCode, tt.wantCode)
			}
		})
	}
}

func TestRunCommand(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	build.CachedWorkingDir = "work"
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
			build.ExecFn = func(_ *goyek.A, _ string, _ ...cmd.Option) bool {
				return tt.shouldSucceed
			}
			if got := build.RunCommand(tt.args.a, tt.args.command); got != tt.want {
				t.Errorf("RunCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	build.CachedWorkingDir = "work"
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
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := build.Format(nil); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
			if gotCmd != "gofmt -e -l -s -w ." {
				t.Errorf("Format() = %q, want %q", gotCmd, "gofmt -e -l -s -w .")
			}
		})
	}
}

func TestGenerateCoverageReport(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	build.CachedWorkingDir = "work"
	tests := map[string]struct {
		file               string
		unitTestsSucceed   bool
		displaySucceeds    bool
		wantDisplayCalled  bool
		wantTestCommand    string
		wantDisplayCommand string
		want               bool
	}{
		"empty file name": {
			file:               "",
			unitTestsSucceed:   false,
			displaySucceeds:    false,
			wantDisplayCalled:  false,
			wantTestCommand:    "",
			wantDisplayCommand: "",
			want:               false,
		},
		"malicious file name": {
			file:               "../../bar",
			unitTestsSucceed:   false,
			displaySucceeds:    false,
			wantDisplayCalled:  false,
			wantTestCommand:    "",
			wantDisplayCommand: "",
			want:               false,
		},
		"absolute file 1": {
			file:               "/bar",
			unitTestsSucceed:   false,
			displaySucceeds:    false,
			wantDisplayCalled:  false,
			wantTestCommand:    "",
			wantDisplayCommand: "",
			want:               false,
		},
		"absolute file 2": {
			file:               "c:/bar",
			unitTestsSucceed:   false,
			displaySucceeds:    false,
			wantDisplayCalled:  false,
			wantTestCommand:    "",
			wantDisplayCommand: "",
			want:               false,
		},
		"tests fail": {
			file:            "coverage.txt",
			wantTestCommand: "go test -coverprofile=coverage.txt ./...",
			want:            false,
		},
		"tests succeed, display fails": {
			file:               "coverage.txt",
			unitTestsSucceed:   true,
			wantDisplayCalled:  true,
			wantTestCommand:    "go test -coverprofile=coverage.txt ./...",
			wantDisplayCommand: "go tool cover -html=coverage.txt",
			want:               false,
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
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
			if got := build.GenerateCoverageReport(nil, tt.file); got != tt.want {
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

func TestAllDirs(t *testing.T) {
	originalBuildFS := build.BuildFS
	defer func() {
		build.BuildFS = originalBuildFS
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("a/b/c", dirMode)
	_ = build.BuildFS.Mkdir("a/b/c/d", dirMode)
	_ = build.BuildFS.Mkdir("a/b/c/e", dirMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/c/f", []byte("data"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/c/e/x", []byte("data"), fileMode)
	tests := map[string]struct {
		top     string
		want    []string
		wantErr bool
	}{
		"error":     {top: "no such dir", want: nil, wantErr: true},
		"not a dir": {top: "a/b/c/f", want: nil, wantErr: true},
		"success":   {top: "a", want: []string{"a", "a/b", "a/b/c", "a/b/c/d", "a/b/c/e"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := build.AllDirs(tt.top)
			if (err != nil) != tt.wantErr {
				t.Errorf("AllDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AllDirs() = %v, want %v", got, tt.want)
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

func TestIsRelevantFile(t *testing.T) {
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
			if got := build.IsRelevantFile(tt.entry, build.MatchGoSource); got != tt.want {
				t.Errorf("IsRelevantFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncludesRelevantFiles(t *testing.T) {
	originalBuildFS := build.BuildFS
	defer func() {
		build.BuildFS = originalBuildFS
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(build.BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
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
			if got := build.IncludesRelevantFiles(tt.dir, build.MatchGoSource); got != tt.want {
				t.Errorf("IncludesRelevantFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelevantDirs(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalBuildFS := build.BuildFS
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.BuildFS = originalBuildFS
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("x/y/z", dirMode)
	_ = afero.WriteFile(build.BuildFS, "x/goo.go", []byte("goo"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "x/y/goop.go", []byte("goop"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "x/y/z/good.go", []byte("good"), fileMode)
	_ = build.BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(build.BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
	tests := map[string]struct {
		workDir string
		want    []string
		wantErr bool
	}{
		"many dirs - including the work dir": {
			workDir: "x",
			want:    []string{"", "y", "y/z"},
			wantErr: false,
		},
		"error": {
			workDir: "no such dir",
			want:    nil,
			wantErr: true,
		},
		"a": {
			workDir: "a",
			want:    []string{"b"},
			wantErr: false,
		},
		"b": {
			workDir: "a/b",
			want:    []string{""},
			wantErr: false,
		},
		"c": {
			workDir: "a/b/c",
			want:    []string{},
			wantErr: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build.CachedWorkingDir = tt.workDir
			got, err := build.RelevantDirs(build.MatchGoSource)
			if (err != nil) != tt.wantErr {
				t.Errorf("RelevantDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RelevantDirs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateDocumentation(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalBuildFS := build.BuildFS
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.BuildFS = originalBuildFS
		build.ExecFn = originalExecFn
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(build.BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
	_ = build.BuildFS.MkdirAll("workDir/dir1/dir2/dir3", dirMode)
	_ = afero.WriteFile(build.BuildFS, "workDir/dir1/dir2/dir3/bar.go", []byte("some code"), fileMode)
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
		"no dir1": {
			args:                args{excludedDirs: []string{"dir1/"}},
			workDir:             "workDir",
			wantExecutorSuccess: false,
			wantCommands:        []string{},
			want:                true,
		},
		"no dir1/dir2": {
			args:                args{excludedDirs: []string{"dir1/dir2"}},
			workDir:             "workDir",
			wantExecutorSuccess: false,
			wantCommands:        []string{},
			want:                true,
		},
		"ok workdir": {
			args:                args{excludedDirs: []string{"a"}},
			workDir:             "workDir",
			wantExecutorSuccess: true,
			wantCommands:        []string{"go doc -all ./dir1/dir2/dir3"},
			want:                true,
		},
		"error": {
			workDir:             "no such dir",
			wantExecutorSuccess: false,
			wantCommands:        []string{},
			want:                false,
		},
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
			build.CachedWorkingDir = tt.workDir
			gotCommands := make([]string, 0)
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				return tt.wantExecutorSuccess
			}
			if got := build.GenerateDocumentation(tt.args.a, tt.args.excludedDirs); got != tt.want {
				t.Errorf("GenerateDocumentation() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("GenerateDocumentation() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	build.CachedWorkingDir = "work"
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
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommand = cmd
				return tt.installSucceeds
			}
			if got := build.Install(tt.args.a, tt.args.packageName); got != tt.want {
				t.Errorf("Install() = %v, want %v", got, tt.want)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("Install() command = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}

func TestLint(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	build.CachedWorkingDir = "work"
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
			gotCommands := make([]string, 0)
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
			if got := build.Lint(nil); got != tt.want {
				t.Errorf("Lint() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("Lint() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestNilAway(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	build.CachedWorkingDir = "work"
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
			gotCommands := make([]string, 0)
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
			if got := build.NilAway(nil); got != tt.want {
				t.Errorf("NilAway() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("NilAway() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestUnitTests(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	build.CachedWorkingDir = "work"
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
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := build.UnitTests(nil); got != tt.want {
				t.Errorf("UnitTests() = %v, want %v", got, tt.want)
			}
			if gotCmd != "go test -cover ./..." {
				t.Errorf("UnitTests() = %q, want %q", gotCmd, "go test -cover ./...")
			}
		})
	}
}

func TestVulnerabilityCheck(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	build.CachedWorkingDir = "work"
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
		"vulnerability check fails": {
			installSucceeds:            true,
			vulnerabilityCheckSucceeds: false,
			wantCommands: []string{
				"go install -v golang.org/x/vuln/cmd/govulncheck@latest",
				"govulncheck -show verbose ./...",
			},
			want: false,
		},
		"vulnerability check succeeds": {
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
			gotCommands := make([]string, 0)
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
			if got := build.VulnerabilityCheck(nil); got != tt.want {
				t.Errorf("VulnerabilityCheck() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("VulnerabilityCheck() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestIsMalformedFileName(t *testing.T) {
	tests := map[string]struct {
		f    string
		want bool
	}{
		"empty":                    {f: "", want: false},
		"back dir 1":               {f: "..", want: true},
		"back dir 2":               {f: "../", want: true},
		"back dir 3":               {f: "..\\", want: true},
		"back dir 4":               {f: "\\..", want: true},
		"back dir 5":               {f: "/..", want: true},
		"complex1":                 {f: "a/b/c/../e/f/g", want: true},
		"complex2":                 {f: "../a/b/c/e/f/g", want: true},
		"startsWithBackslash":      {f: "\\a\\b\\c", want: true},
		"startsWithSlash":          {f: "/a/b/c", want: true},
		"startsWithOldSchoolDrive": {f: "c:/foo/bar/txt", want: true},
		"harmless1":                {f: "a/b..c/d/e/f", want: false},
		"harmless2":                {f: "a/b..", want: false},
		"harmless3":                {f: "a/..b", want: false},
		"harmless4":                {f: "a/b../c", want: false},
		"harmless5":                {f: "a/..b/c", want: false},
		"harmless6":                {f: "b../c", want: false},
		"harmless7":                {f: "..b/c", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := build.IsMalformedFileName(tt.f); got != tt.want {
				t.Errorf("IsMalformedFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintBuffer(t *testing.T) {
	originalPrintlnFn := build.PrintlnFn
	defer func() {
		build.PrintlnFn = originalPrintlnFn
	}()
	tests := map[string]struct {
		data      string
		wantPrint bool
	}{
		"empty":     {data: "", wantPrint: false},
		"some data": {data: "123", wantPrint: true},
		"newlines":  {data: "\n\r", wantPrint: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotPrint := false
			build.PrintlnFn = func(_ ...any) (int, error) {
				gotPrint = true
				return 0, nil
			}
			buffer := &bytes.Buffer{}
			buffer.WriteString(tt.data)
			build.PrintBuffer(buffer)
			if gotPrint != tt.wantPrint {
				t.Errorf("PrintBuffer got %t, want %t", gotPrint, tt.wantPrint)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
	}()
	build.CachedWorkingDir = "work"
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
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := build.Generate(nil); got != tt.want {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
			if gotCmd != "go generate -x ./..." {
				t.Errorf("Generate() = %q, want %q", gotCmd, "go generate -x ./...")
			}
		})
	}
}

func TestEatTrailingEOL(t *testing.T) {
	tests := map[string]struct {
		s    string
		want string
	}{
		"empty string": {
			s:    "",
			want: "",
		},
		"embedded newlines": {
			s:    "abc\ndef\rghi",
			want: "abc\ndef\rghi",
		},
		"many newlines": {
			s:    "abc\n\n\r\r\n\r\n",
			want: "abc",
		},
		"nothing but newlines": {
			s:    "\n\r\n\r\n\r\r\r\n\n\n\r",
			want: "",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := build.EatTrailingEOL(tt.s); got != tt.want {
				t.Errorf("EatTrailingEOL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateDependencies(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalExecFn := build.ExecFn
	originalBuildFS := build.BuildFS
	originalAggressiveFlag := build.AggressiveFlag
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.ExecFn = originalExecFn
		build.BuildFS = originalBuildFS
		build.AggressiveFlag = originalAggressiveFlag
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll(filepath.Join("work", "build"), dirMode)
	_ = build.BuildFS.Mkdir("empty", dirMode)
	_ = afero.WriteFile(build.BuildFS, "badDir", []byte("garbage"), fileMode)
	_ = afero.WriteFile(build.BuildFS, filepath.Join("work", "go.mod"), []byte("module github.com/majohn-r/tools-build"), fileMode)
	_ = afero.WriteFile(build.BuildFS, filepath.Join("work", "build", "go.mod"), []byte("module github.com/majohn-r/tools-build"), fileMode)
	tests := map[string]struct {
		workDir              string
		getCommandAggressive bool
		getSucceeds          bool
		tidySucceeds         bool
		wantCommands         []string
		want                 bool
	}{
		"bad dir": {
			workDir:      "badDir",
			wantCommands: []string{},
			want:         false,
		},
		"empty dir": {
			workDir:      "empty",
			wantCommands: []string{},
			want:         true,
		},
		"go get fails": {
			workDir:      "work",
			getSucceeds:  false,
			wantCommands: []string{"go get -u ./..."},
			want:         false,
		},
		"go mod tidy fails": {
			workDir:      "work",
			getSucceeds:  true,
			tidySucceeds: false,
			wantCommands: []string{"go get -u ./...", "go mod tidy"},
			want:         false,
		},
		"go mod tidy succeeds": {
			workDir:              "work",
			getCommandAggressive: true,
			getSucceeds:          true,
			tidySucceeds:         true,
			wantCommands: []string{
				"go get -u ./...",
				"go mod tidy",
				"go get -u ./...",
				"go mod tidy",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommands := make([]string, 0)
			build.CachedWorkingDir = tt.workDir
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				if strings.HasPrefix(cmd, "go get") {
					return tt.getSucceeds
				}
				if strings.HasPrefix(cmd, "go mod") {
					return tt.tidySucceeds
				}
				t.Errorf("UpdateDependencies() sent unexpected command: %q", cmd)
				return false
			}
			a := tt.getCommandAggressive
			build.AggressiveFlag = &a
			if got := build.UpdateDependencies(nil); got != tt.want {
				t.Errorf("UpdateDependencies() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("UpdateDependencies() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestSetupEnvVars(t *testing.T) {
	var1 := "VAR1"
	var2 := "VAR2"
	var3 := "VAR3"
	vars := []string{var1, var2, var3}
	originalVars := make([]build.EnvVarMemento, 3)
	for k, s := range vars {
		val, defined := os.LookupEnv(s)
		originalVars[k] = build.EnvVarMemento{
			Name:  s,
			Value: val,
			Unset: !defined,
		}
	}
	defer func() {
		for _, ev := range originalVars {
			if ev.Unset {
				_ = os.Unsetenv(ev.Name)
			} else {
				_ = os.Setenv(ev.Name, ev.Value)
			}
		}
	}()
	val := "foo"
	_ = os.Setenv(var1, val)
	_ = os.Unsetenv(var2)
	tests := map[string]struct {
		input  []build.EnvVarMemento
		want   []build.EnvVarMemento
		wantOk bool
	}{
		"error case": {
			input: []build.EnvVarMemento{
				{
					Name:  var3,
					Value: "foo",
					Unset: false,
				},
				{
					Name:  var3,
					Value: "bar",
					Unset: false,
				},
			},
			want:   nil,
			wantOk: false,
		},
		"thorough": {
			input: []build.EnvVarMemento{
				{
					Name:  var1,
					Value: "",
					Unset: true,
				},
				{
					Name:  var2,
					Value: "foo",
					Unset: false,
				},
			},
			want: []build.EnvVarMemento{
				{
					Name:  var1,
					Value: val,
					Unset: false,
				},
				{
					Name:  var2,
					Value: "",
					Unset: true,
				},
			},
			wantOk: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, gotOk := build.SetupEnvVars(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetupEnvVars() = %v, want %v", got, tt.want)
			}
			if gotOk != tt.wantOk {
				t.Errorf("SetupEnvVars() = %t, want %t", gotOk, tt.wantOk)
			}
		})
	}
}

func TestRestoreEnvVars(t *testing.T) {
	originalSetenvFn := build.SetenvFn
	originalUnsetenvFn := build.UnsetenvFn
	defer func() {
		build.SetenvFn = originalSetenvFn
		build.UnsetenvFn = originalUnsetenvFn
	}()
	var sets int
	var unsets int
	build.SetenvFn = func(_, _ string) error {
		sets++
		return nil
	}
	build.UnsetenvFn = func(_ string) error {
		unsets++
		return nil
	}
	tests := map[string]struct {
		saved     []build.EnvVarMemento
		wantSet   int
		wantUnset int
	}{
		"mix": {
			saved: []build.EnvVarMemento{
				{
					Name:  "v1",
					Value: "val1",
					Unset: false,
				},
				{
					Name:  "v2",
					Value: "",
					Unset: true,
				},
				{
					Name:  "v3",
					Value: "val3",
					Unset: false,
				},
				{
					Name:  "v4",
					Value: "",
					Unset: true,
				},
				{
					Name:  "v5",
					Value: "",
					Unset: true,
				},
			},
			wantSet:   2,
			wantUnset: 3,
		},
		"empty": {
			saved:     nil,
			wantSet:   0,
			wantUnset: 0,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sets = 0
			unsets = 0
			build.RestoreEnvVars(tt.saved)
			if sets != tt.wantSet {
				t.Errorf("RestoreEnvVars set %d, want %d", sets, tt.wantSet)
			}
			if unsets != tt.wantUnset {
				t.Errorf("RestoreEnvVars Unset %d, want %d", unsets, tt.wantUnset)
			}
		})
	}
}

func TestFormatSelective(t *testing.T) {
	originalCachedWorkingDir := build.CachedWorkingDir
	originalBuildFS := build.BuildFS
	originalExecFn := build.ExecFn
	defer func() {
		build.CachedWorkingDir = originalCachedWorkingDir
		build.BuildFS = originalBuildFS
		build.ExecFn = originalExecFn
	}()
	build.BuildFS = afero.NewMemMapFs()
	_ = build.BuildFS.MkdirAll("work/x/y/z", dirMode)
	_ = build.BuildFS.MkdirAll("work/go", dirMode)
	_ = afero.WriteFile(build.BuildFS, "work/foo.go", []byte("foo"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "work/foo_test.go", []byte("foo"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "work/x/a.go", []byte("a"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "work/x/y/b_test.go", []byte("b_test"), fileMode)
	_ = afero.WriteFile(build.BuildFS, "work/x/y/z/c.go", []byte("c"), fileMode)
	_ = build.BuildFS.MkdirAll("work/.idea/fileTemplates/code", dirMode)
	_ = afero.WriteFile(build.BuildFS, "work/.idea/fileTemplates/code/Go Table Test.go", []byte("not a good file"), fileMode)
	type args struct {
		a          *goyek.A
		exclusions []string
	}
	tests := map[string]struct {
		args
		workingDir          string
		wantExecutorSuccess bool
		wantCommand         string
		want                bool
	}{
		"file error": {
			args:                args{exclusions: []string{"foo"}},
			workingDir:          "wonk",
			wantExecutorSuccess: false,
			wantCommand:         "",
			want:                false,
		},
		"no exclusions succeeds": {
			args:                args{exclusions: nil},
			workingDir:          "work",
			wantExecutorSuccess: true,
			wantCommand:         "gofmt -e -l -s -w .",
			want:                true,
		},
		"no exclusions fails": {
			args:                args{exclusions: nil},
			workingDir:          "work",
			wantExecutorSuccess: false,
			wantCommand:         "gofmt -e -l -s -w .",
			want:                false,
		},
		"exclusions succeeds": {
			args:                args{exclusions: []string{".idea"}},
			workingDir:          "work",
			wantExecutorSuccess: true,
			wantCommand:         "gofmt -e -l -s -w foo.go foo_test.go x x/y x/y/z",
			want:                true,
		},
		"exclusions fails": {
			args:                args{exclusions: []string{".idea"}},
			workingDir:          "work",
			wantExecutorSuccess: false,
			wantCommand:         "gofmt -e -l -s -w foo.go foo_test.go x x/y x/y/z",
			want:                false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build.CachedWorkingDir = tt.workingDir
			var gotCommand string
			build.ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommand = cmd
				return tt.wantExecutorSuccess
			}
			if got := build.FormatSelective(tt.args.a, tt.args.exclusions); got != tt.want {
				t.Errorf("FormatSelective() = %v, want %v", got, tt.want)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("FormatSelective() = %v, want %v", gotCommand, tt.wantCommand)
			}
		})
	}
}
