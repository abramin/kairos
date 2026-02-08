package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptYesNoIO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "yes lowercase lf", input: "y\n", want: true},
		{name: "yes word lf", input: "yes\n", want: true},
		{name: "yes mixed case lf", input: "YeS\n", want: true},
		{name: "yes lowercase cr", input: "y\r", want: true},
		{name: "yes word cr", input: "yes\r", want: true},
		{name: "no default lf", input: "\n", want: false},
		{name: "no explicit cr", input: "n\r", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			got := promptYesNoIO(strings.NewReader(tc.input), &out, "Confirm? [y/N]: ")
			assert.Equal(t, tc.want, got)
			assert.Equal(t, "Confirm? [y/N]: ", out.String())
		})
	}
}

func TestPromptYesNoWithDefaultIO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{name: "empty input defaults yes", input: "\n", defaultYes: true, want: true},
		{name: "empty input defaults no", input: "\n", defaultYes: false, want: false},
		{name: "explicit no overrides yes default", input: "n\n", defaultYes: true, want: false},
		{name: "explicit yes with no default", input: "yes\n", defaultYes: false, want: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			got := promptYesNoWithDefaultIO(strings.NewReader(tc.input), &out, "Confirm? [Y/n]: ", tc.defaultYes)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, "Confirm? [Y/n]: ", out.String())
		})
	}
}

func TestReadPromptLine_EOFWithoutNewline(t *testing.T) {
	t.Parallel()

	got, err := readPromptLine(strings.NewReader("yes"))
	assert.NoError(t, err)
	assert.Equal(t, "yes", got)
}
