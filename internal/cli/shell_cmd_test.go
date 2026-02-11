package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitShellArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single word",
			input: "status",
			want:  []string{"status"},
		},
		{
			name:  "double quoted phrase",
			input: `ask "can you update project OU10 to make weeks 1 to 17 as done"`,
			want:  []string{"ask", "can you update project OU10 to make weeks 1 to 17 as done"},
		},
		{
			name:  "single quoted phrase",
			input: "ask 'what should I work on?'",
			want:  []string{"ask", "what should I work on?"},
		},
		{
			name:  "flags with quoted value",
			input: `work update w1 --title "Deep work block"`,
			want:  []string{"work", "update", "w1", "--title", "Deep work block"},
		},
		{
			name:  "empty quoted arg",
			input: `ask ""`,
			want:  []string{"ask", ""},
		},
		{
			name:    "unterminated quote",
			input:   `ask "oops`,
			wantErr: true,
		},
		{
			name:    "unterminated escape",
			input:   `ask hi\`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := splitShellArgs(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrepareShellCobraArgs_PassthroughForAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"ask", "mark OU10 done"}, "")
	assert.Equal(t, []string{"ask", "mark OU10 done"}, got)
}

func TestPrepareShellCobraArgs_DoesNotChangeNonAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"project", "list"}, "")
	assert.Equal(t, []string{"project", "list"}, got)
}
