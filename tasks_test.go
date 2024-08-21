package tools_build

import (
	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
	"github.com/spf13/afero"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDeadcode(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	originalNoFormatFlag := NoFormatFlag
	originalNoTestFlag := NoTestFlag
	originalTemplateFlag := TemplateFlag
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
		NoFormatFlag = originalNoFormatFlag
		NoTestFlag = originalNoTestFlag
		TemplateFlag = originalTemplateFlag
	}()
	// not used, and keeps WorkingDir() from getting exercised
	CachedWorkingDir = "work"
	tests := map[string]struct {
		installSucceeds  bool
		deadcodeSucceeds bool
		noFormatFlag     bool
		noTestFlag       bool
		templateFlag     string
		wantCommands     []string
		want             bool
	}{
		"install fails": {
			installSucceeds:  false,
			deadcodeSucceeds: false,
			noFormatFlag:     false,
			noTestFlag:       false,
			templateFlag:     `{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
			},
			want: false,
		},
		"deadcode fails": {
			installSucceeds:  true,
			deadcodeSucceeds: false,
			noFormatFlag:     false,
			noTestFlag:       false,
			templateFlag:     `{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
				`deadcode -f='{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}' -test .`,
			},
			want: false,
		},
		"deadcode succeeds": {
			installSucceeds:  true,
			deadcodeSucceeds: true,
			noFormatFlag:     false,
			noTestFlag:       false,
			templateFlag:     `{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
				`deadcode -f='{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}' -test .`,
			},
			want: true,
		},
		"no formatting": {
			installSucceeds:  true,
			deadcodeSucceeds: true,
			noFormatFlag:     true,
			noTestFlag:       false,
			templateFlag:     `{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
				`deadcode -test .`,
			},
			want: true,
		},
		"no test": {
			installSucceeds:  true,
			deadcodeSucceeds: true,
			noFormatFlag:     false,
			noTestFlag:       true,
			templateFlag:     `{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
				`deadcode -f='{{println .Path}}{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}' .`,
			},
			want: true,
		},
		"alternate template": {
			installSucceeds:  true,
			deadcodeSucceeds: true,
			noFormatFlag:     false,
			noTestFlag:       false,
			templateFlag:     `{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}`,
			wantCommands: []string{
				"go install -v golang.org/x/tools/cmd/deadcode@latest",
				`deadcode -f='{{range .Funcs}}{{printf "\t%s\t%s\n" .Position .Name}}{{end}}{{println}}' -test .`,
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotCommands := make([]string, 0)
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommands = append(gotCommands, cmd)
				if strings.Contains(cmd, " install ") {
					return tt.installSucceeds
				}
				if strings.HasPrefix(cmd, "deadcode ") {
					return tt.deadcodeSucceeds
				}
				t.Errorf("Deadcode() sent unexpected command: %q", cmd)
				return false
			}
			nff := tt.noFormatFlag
			NoFormatFlag = &nff
			ntf := tt.noTestFlag
			NoTestFlag = &ntf
			tf := tt.templateFlag
			TemplateFlag = &tf
			if got := Deadcode(nil); got != tt.want {
				t.Errorf("Deadcode() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("Deadcode() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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

func TestFormatSelective(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalBuildFS := BuildFS
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		BuildFS = originalBuildFS
		ExecFn = originalExecFn
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("work/x/y/z", dirMode)
	_ = BuildFS.MkdirAll("work/go", dirMode)
	_ = afero.WriteFile(BuildFS, "work/foo.go", []byte("foo"), fileMode)
	_ = afero.WriteFile(BuildFS, "work/foo_test.go", []byte("foo"), fileMode)
	_ = afero.WriteFile(BuildFS, "work/x/a.go", []byte("a"), fileMode)
	_ = afero.WriteFile(BuildFS, "work/x/y/b_test.go", []byte("b_test"), fileMode)
	_ = afero.WriteFile(BuildFS, "work/x/y/z/c.go", []byte("c"), fileMode)
	_ = BuildFS.MkdirAll("work/.idea/fileTemplates/code", dirMode)
	_ = afero.WriteFile(BuildFS, "work/.idea/fileTemplates/code/Go Table Test.go", []byte("not a good file"), fileMode)
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
			CachedWorkingDir = tt.workingDir
			var gotCommand string
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCommand = cmd
				return tt.wantExecutorSuccess
			}
			if got := FormatSelective(tt.args.a, tt.args.exclusions); got != tt.want {
				t.Errorf("FormatSelective() = %v, want %v", got, tt.want)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("FormatSelective() = %v, want %v", gotCommand, tt.wantCommand)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				gotCmd = cmd
				return tt.shouldSucceed
			}
			if got := Generate(nil); got != tt.want {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
			if gotCmd != "go generate -x ./..." {
				t.Errorf("Generate() = %q, want %q", gotCmd, "go generate -x ./...")
			}
		})
	}
}

func TestGenerateCoverageReport(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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

func TestGenerateDocumentation(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalBuildFS := BuildFS
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		BuildFS = originalBuildFS
		ExecFn = originalExecFn
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
	_ = BuildFS.MkdirAll("workDir/dir1/dir2/dir3", dirMode)
	_ = afero.WriteFile(BuildFS, "workDir/dir1/dir2/dir3/bar.go", []byte("some code"), fileMode)
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
			CachedWorkingDir = tt.workDir
			gotCommands := make([]string, 0)
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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

func TestRunCommand(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, _ string, _ ...cmd.Option) bool {
				return tt.shouldSucceed
			}
			if got := RunCommand(tt.args.a, tt.args.command); got != tt.want {
				t.Errorf("RunCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnitTests(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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

func TestUpdateDependencies(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	originalBuildFS := BuildFS
	originalAggressiveFlag := AggressiveFlag
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
		BuildFS = originalBuildFS
		AggressiveFlag = originalAggressiveFlag
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll(filepath.Join("work", "build"), dirMode)
	_ = BuildFS.Mkdir("empty", dirMode)
	_ = afero.WriteFile(BuildFS, "badDir", []byte("garbage"), fileMode)
	_ = afero.WriteFile(BuildFS, filepath.Join("work", "go.mod"), []byte("module github.com/majohn-r/tools-build"), fileMode)
	_ = afero.WriteFile(BuildFS, filepath.Join("work", "build", "go.mod"), []byte("module github.com/majohn-r/tools-build"), fileMode)
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
			CachedWorkingDir = tt.workDir
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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
			AggressiveFlag = &a
			if got := UpdateDependencies(nil); got != tt.want {
				t.Errorf("UpdateDependencies() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotCommands, tt.wantCommands) {
				t.Errorf("UpdateDependencies() commands = %v, want %v", gotCommands, tt.wantCommands)
			}
		})
	}
}

func TestVulnerabilityCheck(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalExecFn := ExecFn
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExecFn = originalExecFn
	}()
	// not used, and keeps WorkingDir() from getting exercised
	CachedWorkingDir = "work"
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
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
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

func Test_directedCommand_execute(t *testing.T) {
	originalExecFn := ExecFn
	defer func() {
		ExecFn = originalExecFn
	}()
	type fields struct {
		command string
		dir     string
		envVars []EnvVarMemento
	}
	tests := map[string]struct {
		fields
		execSucceeds bool
		execRan      bool
		want         bool
	}{
		"happy": {
			fields:       fields{},
			execRan:      true,
			execSucceeds: true,
			want:         true,
		},
		"bad set up": {
			fields: fields{
				envVars: []EnvVarMemento{
					{Name: "HOME", Value: "/home"},
					{Name: "HOME", Value: "/home"},
				},
			},
			execRan:      false,
			execSucceeds: true,
			want:         false,
		},
		"sad": {
			fields:       fields{},
			execRan:      true,
			execSucceeds: false,
			want:         false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dC := directedCommand{
				command: tt.fields.command,
				dir:     tt.fields.dir,
				envVars: tt.fields.envVars,
			}
			var ran bool
			ExecFn = func(_ *goyek.A, cmd string, _ ...cmd.Option) bool {
				ran = true
				return tt.execSucceeds
			}
			if got := dC.execute(nil); got != tt.want {
				t.Errorf("execute() = %v, want %v", got, tt.want)
			}
			if got := ran; got != tt.execRan {
				t.Errorf("execute() ran = %v, want %v", got, tt.execRan)
			}
		})
	}
}

func TestTaskDisabled(t *testing.T) {
	originalDisableFlag := disableFlag
	defer func() {
		disableFlag = originalDisableFlag
	}()
	tests := map[string]struct {
		taskName     string
		disableValue string
		wantDisabled bool
	}{
		"typical": {
			taskName:     "nilaway",
			disableValue: "",
			wantDisabled: false,
		},
		"real use case": {
			taskName:     "nilaway",
			disableValue: "task1, task2, NilAway ",
			wantDisabled: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			flag := tt.disableValue
			disableFlag = &flag
			if gotDisabled := TaskDisabled(tt.taskName); gotDisabled != tt.wantDisabled {
				t.Errorf("TaskDisabled() = %v, want %v", gotDisabled, tt.wantDisabled)
			}
		})
	}
}
