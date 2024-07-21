package tools_build

import "os"

var (
	// SetenvFn is the os.Setenv function, set as a variable so that unit tests can override
	SetenvFn = os.Setenv
	// UnsetenvFn is the os.Unsetenv function, set as a variable so that unit tests can override
	UnsetenvFn = os.Unsetenv
)

// EnvVarMemento captures an environment variable's desired state
type EnvVarMemento struct {
	// Name is the variable's name
	Name string
	// Value is what the variable should be set to
	Value string
	// Unset, if true, means the variable should be unset
	Unset bool
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

func printRestoration(v EnvVarMemento) {
	if v.Unset {
		printIt("restoring (unsetting):", v.Name)
	} else {
		printIt("restoring (resetting):", v.Name, "<-", v.Value)
	}
}
