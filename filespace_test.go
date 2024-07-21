package tools_build

import (
	"github.com/spf13/afero"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
)

func TestAllDirs(t *testing.T) {
	originalBuildFS := BuildFS
	defer func() {
		BuildFS = originalBuildFS
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("a/b/c", dirMode)
	_ = BuildFS.Mkdir("a/b/c/d", dirMode)
	_ = BuildFS.Mkdir("a/b/c/e", dirMode)
	_ = afero.WriteFile(BuildFS, "a/b/c/f", []byte("data"), fileMode)
	_ = afero.WriteFile(BuildFS, "a/b/c/e/x", []byte("data"), fileMode)
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
			got, err := AllDirs(tt.top)
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

func TestClean(t *testing.T) {
	// note: cannot use memory mapped filesystem; Clean relies on using the os
	// filesystem to make sure all files are within the working directory
	originalCachedWorkingDir := CachedWorkingDir
	originalExitFn := ExitFn
	CachedWorkingDir = "a/b/c"
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		ExitFn = originalExitFn
		_ = BuildFS.RemoveAll("a")
	}()
	_ = BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(BuildFS, "a/b/c/myFile", []byte("foo"), fileMode)
	_ = afero.WriteFile(BuildFS, "a/b/c/myOtherFile", []byte(""), fileMode)
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
			ExitFn = func(_ int) {
				gotExitCalled = true
			}
			Clean(tt.files)
			if gotExitCalled != tt.wantExitCalled {
				t.Errorf("Clean exit called %t, want %t", gotExitCalled, tt.wantExitCalled)
			}
			if !gotExitCalled {
				for _, file := range tt.files {
					f := filepath.Join(CachedWorkingDir, file)
					if fileExists, _ := afero.Exists(BuildFS, f); fileExists {
						t.Errorf("Clean failed to delete %q", f)
					}
				}
			}
		})
	}
}

func TestIncludesRelevantFiles(t *testing.T) {
	originalBuildFS := BuildFS
	defer func() {
		BuildFS = originalBuildFS
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
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
			if got := IncludesRelevantFiles(tt.dir, MatchGoSource); got != tt.want {
				t.Errorf("IncludesRelevantFiles() = %v, want %v", got, tt.want)
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
			if got := IsMalformedFileName(tt.f); got != tt.want {
				t.Errorf("IsMalformedFileName() = %v, want %v", got, tt.want)
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
			if got := IsRelevantFile(tt.entry, MatchGoSource); got != tt.want {
				t.Errorf("IsRelevantFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchGoSource(t *testing.T) {
	tests := map[string]struct {
		name string
		want bool
	}{
		"non-starter": {
			name: "README.md",
			want: false,
		},
		"test file 1": {
			name: "file_test.go",
			want: false,
		},
		"test file 2": {
			name: "testing_file.go",
			want: false,
		},
		"good file": {
			name: "file.go",
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := MatchGoSource(tt.name); got != tt.want {
				t.Errorf("MatchGoSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelevantDirs(t *testing.T) {
	originalCachedWorkingDir := CachedWorkingDir
	originalBuildFS := BuildFS
	defer func() {
		CachedWorkingDir = originalCachedWorkingDir
		BuildFS = originalBuildFS
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("x/y/z", dirMode)
	_ = afero.WriteFile(BuildFS, "x/goo.go", []byte("goo"), fileMode)
	_ = afero.WriteFile(BuildFS, "x/y/goop.go", []byte("goop"), fileMode)
	_ = afero.WriteFile(BuildFS, "x/y/z/good.go", []byte("good"), fileMode)
	_ = BuildFS.MkdirAll("a/b/c", dirMode)
	_ = afero.WriteFile(BuildFS, "a/foo_test.go", []byte("test stuff"), fileMode)
	_ = afero.WriteFile(BuildFS, "a/b/foo.go", []byte("source code"), fileMode)
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
			CachedWorkingDir = tt.workDir
			got, err := RelevantDirs(MatchGoSource)
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

func TestUnacceptableWorkingDir(t *testing.T) {
	originalBuildFS := BuildFS
	defer func() {
		BuildFS = originalBuildFS
	}()
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll("successful/.git", dirMode)
	_ = BuildFS.Mkdir("empty", dirMode)
	_ = BuildFS.Mkdir("defective", dirMode)
	_ = afero.WriteFile(BuildFS, filepath.Join("defective", ".git"), []byte("data"), fileMode)
	_ = afero.WriteFile(BuildFS, "not a directory", []byte("gibberish"), fileMode)
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
			if got := UnacceptableWorkingDir(tt.candidate); got != tt.want {
				t.Errorf("UnacceptableWorkingDir() %t, want %t", got, tt.want)
			}
		})
	}
}

func TestWorkingDir(t *testing.T) {
	originalBuildFS := BuildFS
	originalExitFn := ExitFn
	originalDirValue, originalDirExists := os.LookupEnv("DIR")
	originalCachedWorkingDir := CachedWorkingDir
	defer func() {
		BuildFS = originalBuildFS
		ExitFn = originalExitFn
		if originalDirExists {
			_ = os.Setenv("DIR", originalDirValue)
		} else {
			_ = os.Unsetenv("DIR")
		}
		CachedWorkingDir = originalCachedWorkingDir
	}()
	recordedCode := 0
	ExitFn = func(code int) {
		recordedCode = code
	}
	BuildFS = afero.NewMemMapFs()
	_ = BuildFS.MkdirAll(filepath.Join("..", ".git"), dirMode)
	_ = BuildFS.MkdirAll(filepath.Join("happy", ".git"), dirMode)
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
			CachedWorkingDir = tt.workDir
			if tt.dirFromEnv {
				_ = os.Setenv("DIR", tt.dirEnvValue)
			} else {
				_ = os.Unsetenv("DIR")
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

func Test_canonicalPath(t *testing.T) {
	tests := map[string]struct {
		path string
		want string
	}{
		"with backslashes": {
			path: "\\foo\\bar\\baz\\",
			want: "/foo/bar/baz/",
		},
		"without backslashes": {
			path: "/foo/bar/baz/",
			want: "/foo/bar/baz/",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := canonicalPath(tt.path); got != tt.want {
				t.Errorf("canonicalPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_endsIn(t *testing.T) {
	type args struct {
		s      string
		suffix string
	}
	tests := map[string]struct {
		args
		want bool
	}{
		"ends in .go": {
			args: args{
				s:      "foo.go",
				suffix: ".go",
			},
			want: true,
		},
		"doesn't end in .go": {
			args: args{
				s:      "foo_go",
				suffix: ".go",
			},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := endsIn(tt.args.s, tt.args.suffix); got != tt.want {
				t.Errorf("endsIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isIllegalFileName(t *testing.T) {
	tests := map[string]struct {
		path string
		want bool
	}{
		"empty":                    {path: "", want: true},
		"back dir 1":               {path: "..", want: true},
		"back dir 2":               {path: "../", want: true},
		"back dir 3":               {path: "..\\", want: true},
		"back dir 4":               {path: "\\..", want: true},
		"back dir 5":               {path: "/..", want: true},
		"complex1":                 {path: "a/b/c/../e/f/g", want: true},
		"complex2":                 {path: "../a/b/c/e/f/g", want: true},
		"startsWithBackslash":      {path: "\\a\\b\\c", want: true},
		"startsWithSlash":          {path: "/a/b/c", want: true},
		"startsWithOldSchoolDrive": {path: "c:/foo/bar/txt", want: true},
		"harmless1":                {path: "a/b..c/d/e/f", want: false},
		"harmless2":                {path: "a/b..", want: false},
		"harmless3":                {path: "a/..b", want: false},
		"harmless4":                {path: "a/b../c", want: false},
		"harmless5":                {path: "a/..b/c", want: false},
		"harmless6":                {path: "b../c", want: false},
		"harmless7":                {path: "..b/c", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isIllegalFileName(tt.path); got != tt.want {
				t.Errorf("isIllegalFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isInvalidDir(t *testing.T) {
	originalBuildFS := BuildFS
	defer func() {
		BuildFS = originalBuildFS
	}()
	BuildFS = afero.NewMemMapFs()
	_ = afero.WriteFile(BuildFS, "not a directory", []byte("gibberish"), fileMode)
	_ = BuildFS.Mkdir("empty", dirMode)
	tests := map[string]struct {
		path string
		want bool
	}{
		"no such file": {
			path: "empty/foo",
			want: true,
		},
		"not an actual directory": {
			path: "not a directory",
			want: true,
		},
		"an actual directory": {
			path: "empty",
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isInvalidDir(tt.path); got != tt.want {
				t.Errorf("isInvalidDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isParentDir(t *testing.T) {
	type args struct {
		targetDir      string
		possibleParent string
	}
	tests := map[string]struct {
		args
		want bool
	}{
		"impossible": {
			args: args{
				targetDir:      "foo",
				possibleParent: "bar",
			},
			want: false,
		},
		"identical": {
			args: args{
				targetDir:      "foo",
				possibleParent: "foo",
			},
			want: true,
		},
		"possible parent ends in /": {
			args: args{
				targetDir:      "foo/bar",
				possibleParent: "foo/",
			},
			want: true,
		},
		"false match": {
			args: args{
				targetDir:      "foo/bar",
				possibleParent: "f",
			},
			want: false,
		},
		"good match": {
			args: args{
				targetDir:      "foo/bar",
				possibleParent: "foo",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isParentDir(tt.args.targetDir, tt.args.possibleParent); got != tt.want {
				t.Errorf("isParentDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_matchAnyGoFile(t *testing.T) {
	tests := map[string]struct {
		name string
		want bool
	}{
		"match": {
			name: "match.go",
			want: true,
		},
		"even a test file": {
			name: "match_test.go",
			want: true,
		},
		"not a match": {
			name: "README.md",
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := matchAnyGoFile(tt.name); got != tt.want {
				t.Errorf("matchAnyGoFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_matchModuleFile(t *testing.T) {
	tests := map[string]struct {
		name string
		want bool
	}{
		"yes": {
			name: "go.mod",
			want: true,
		},
		"no": {
			name: "go.sum",
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := matchModuleFile(tt.name); got != tt.want {
				t.Errorf("matchModuleFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_startsWith(t *testing.T) {
	type args struct {
		s      string
		prefix string
	}
	tests := map[string]struct {
		args
		want bool
	}{
		"match": {
			args: args{
				s:      "foo.go",
				prefix: "foo",
			},
			want: true,
		},
		"not match": {
			args: args{
				s:      "foo.go",
				prefix: "food",
			},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := startsWith(tt.args.s, tt.args.prefix); got != tt.want {
				t.Errorf("startsWith() = %v, want %v", got, tt.want)
			}
		})
	}
}
