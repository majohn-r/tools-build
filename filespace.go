package tools_build

import (
	"fmt"
	"github.com/spf13/afero"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// BuildFS is the file system used; accessible so that tests can override it
	BuildFS = afero.NewOsFs()
	// CachedWorkingDir is the cached working directory
	CachedWorkingDir = ""
)

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

// IsRelevantFile returns true if the entry is a file and its name is validated by the provided fileMatcher
func IsRelevantFile(entry fs.FileInfo, fileMatcher func(string) bool) bool {
	if !entry.Mode().IsRegular() {
		return false
	}
	name := entry.Name()
	return fileMatcher(name)
}

// MatchGoSource matches a file name ending in '.go', but does not match test files.
func MatchGoSource(name string) bool {
	return endsIn(name, ".go") && !endsIn(name, "_test.go") && !startsWith(name, "testing")
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

func canonicalPath(path string) string {
	if strings.Contains(path, "\\") {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

func endsIn(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}

func isIllegalFileName(path string) bool {
	return path == "" || IsMalformedFileName(path)
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

func matchAnyGoFile(name string) bool {
	return endsIn(name, ".go")
}

func matchModuleFile(name string) bool {
	return name == "go.mod"
}

func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}
