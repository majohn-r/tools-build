package tools_build

import (
	"bytes"
	"fmt"
	"strings"
)

var (
	// PrintlnFn is the fmt.Println function, set as a variable so that unit tests can override
	PrintlnFn = fmt.Println
)

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

// PrintBuffer sends the buffer contents to stdout, but first strips trailing EOL characters, and then only prints the
// remaining content if that content is not empty
func PrintBuffer(b *bytes.Buffer) {
	s := EatTrailingEOL(b.String())
	if s != "" {
		printIt(s)
	}
}

func printIt(a ...any) {
	_, _ = PrintlnFn(a...)
}
