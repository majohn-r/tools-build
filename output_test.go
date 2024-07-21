package tools_build

import (
	"bytes"
	"fmt"
	"testing"
)

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
			if got := EatTrailingEOL(tt.s); got != tt.want {
				t.Errorf("EatTrailingEOL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintBuffer(t *testing.T) {
	originalPrintlnFn := PrintlnFn
	defer func() {
		PrintlnFn = originalPrintlnFn
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
			PrintlnFn = func(_ ...any) (int, error) {
				gotPrint = true
				return 0, nil
			}
			buffer := &bytes.Buffer{}
			buffer.WriteString(tt.data)
			PrintBuffer(buffer)
			if gotPrint != tt.wantPrint {
				t.Errorf("PrintBuffer got %t, want %t", gotPrint, tt.wantPrint)
			}
		})
	}
}

func Test_printIt(t *testing.T) {
	originalPrintLnFn := PrintlnFn
	defer func() {
		PrintlnFn = originalPrintLnFn
	}()
	var buffer bytes.Buffer
	PrintlnFn = func(a ...any) (int, error) {
		count := 0
		for index, item := range a {
			if index != 0 {
				e := buffer.WriteByte(' ')
				if e != nil {
					return count, e
				}
				count++
			}
			c, e := fmt.Fprintf(&buffer, "%v", item)
			if e != nil {
				return count, e
			}
			count += c
		}
		return count, nil
	}
	tests := map[string]struct {
		a    []any
		want string
	}{
		"no args": {
			a:    nil,
			want: "",
		},
		"plenty of args": {
			a:    []any{1, 2, 3},
			want: "1 2 3",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buffer.Reset()
			printIt(tt.a...)
			if got := buffer.String(); got != tt.want {
				t.Errorf("printIt() = %v, want %v", got, tt.want)
			}
		})
	}
}
