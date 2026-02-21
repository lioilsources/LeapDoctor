package exec

import "fmt"

// MockResponse holds a predefined response for a command.
type MockResponse struct {
	Output string
	Err    error
}

// MockRunner returns predefined responses for testing.
// Commands are matched by joining name and args with spaces.
type MockRunner struct {
	Responses map[string]MockResponse
	Calls     []string // recorded calls for assertions
}

func (m *MockRunner) Run(name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + joinArgs(args)
	}
	m.Calls = append(m.Calls, key)

	if resp, ok := m.Responses[key]; ok {
		return resp.Output, resp.Err
	}
	// Try prefix match for flexible matching
	for pattern, resp := range m.Responses {
		if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
			prefix := pattern[:len(pattern)-1]
			if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				return resp.Output, resp.Err
			}
		}
	}
	return "", fmt.Errorf("mock: no response for %q", key)
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}
