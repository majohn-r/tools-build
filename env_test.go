package tools_build

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestRestoreEnvVars(t *testing.T) {
	originalSetenvFn := SetenvFn
	originalUnsetenvFn := UnsetenvFn
	defer func() {
		SetenvFn = originalSetenvFn
		UnsetenvFn = originalUnsetenvFn
	}()
	var sets int
	var unsets int
	SetenvFn = func(_, _ string) error {
		sets++
		return nil
	}
	UnsetenvFn = func(_ string) error {
		unsets++
		return nil
	}
	tests := map[string]struct {
		saved     []EnvVarMemento
		wantSet   int
		wantUnset int
	}{
		"mix": {
			saved: []EnvVarMemento{
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
			RestoreEnvVars(tt.saved)
			if sets != tt.wantSet {
				t.Errorf("RestoreEnvVars set %d, want %d", sets, tt.wantSet)
			}
			if unsets != tt.wantUnset {
				t.Errorf("RestoreEnvVars Unset %d, want %d", unsets, tt.wantUnset)
			}
		})
	}
}

func TestSetupEnvVars(t *testing.T) {
	var1 := "VAR1"
	var2 := "VAR2"
	var3 := "VAR3"
	vars := []string{var1, var2, var3}
	originalVars := make([]EnvVarMemento, 3)
	for k, s := range vars {
		val, defined := os.LookupEnv(s)
		originalVars[k] = EnvVarMemento{
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
		input  []EnvVarMemento
		want   []EnvVarMemento
		wantOk bool
	}{
		"error case": {
			input: []EnvVarMemento{
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
			input: []EnvVarMemento{
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
			want: []EnvVarMemento{
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
			got, gotOk := SetupEnvVars(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetupEnvVars() = %v, want %v", got, tt.want)
			}
			if gotOk != tt.wantOk {
				t.Errorf("SetupEnvVars() = %t, want %t", gotOk, tt.wantOk)
			}
		})
	}
}

func Test_checkEnvVars(t *testing.T) {
	tests := map[string]struct {
		input []EnvVarMemento
		want  bool
	}{
		"degenerate": {input: nil, want: true},
		"typical": {
			input: []EnvVarMemento{
				{Name: "VAR1", Value: "val1"},
				{Name: "VAR2", Value: "val2"},
			},
			want: true,
		},
		"oops": {
			input: []EnvVarMemento{
				{Name: "VAR1", Value: ""},
				{Name: "VAR2", Value: ""},
				{Name: "VAR1", Value: ""},
			},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := checkEnvVars(tt.input); got != tt.want {
				t.Errorf("checkEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_printFormerEnvVarState(t *testing.T) {
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
			c, e := buffer.WriteString(item.(string))
			if e != nil {
				return count, e
			}
			count += c
		}
		return count, nil
	}
	tests := map[string]struct {
		name    string
		value   string
		defined bool
		want    string
	}{
		"defined": {
			name:    "VAR1",
			value:   "val1",
			defined: true,
			want:    "VAR1 was set to val1",
		},
		"undefined": {
			name:    "VAR2",
			defined: false,
			want:    "VAR2 was not set",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buffer.Reset()
			printFormerEnvVarState(tt.name, tt.value, tt.defined)
			if got := buffer.String(); got != tt.want {
				t.Errorf("printFormerEnvVarState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_printRestoration(t *testing.T) {
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
			c, e := buffer.WriteString(item.(string))
			if e != nil {
				return count, e
			}
			count += c
		}
		return count, nil
	}
	tests := map[string]struct {
		v    EnvVarMemento
		want string
	}{
		"set": {v: EnvVarMemento{
			Name:  "VAR1",
			Value: "val1",
			Unset: false,
		},
			want: "restoring (resetting): VAR1 <- val1",
		},
		"unset": {v: EnvVarMemento{
			Name:  "VAR1",
			Value: "val2",
			Unset: true,
		},
			want: "restoring (unsetting): VAR1",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buffer.Reset()
			printRestoration(tt.v)
			if got := buffer.String(); got != tt.want {
				t.Errorf("printRestoration() = %v, want %v", got, tt.want)
			}
		})
	}
}
