package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func promptYesNo(message string) bool {
	return promptYesNoIO(os.Stdin, os.Stdout, message)
}

func promptYesNoIO(in io.Reader, out io.Writer, message string) bool {
	return promptYesNoWithDefaultIO(in, out, message, false)
}

func promptYesNoWithDefault(message string, defaultYes bool) bool {
	return promptYesNoWithDefaultIO(os.Stdin, os.Stdout, message, defaultYes)
}

func promptYesNoWithDefaultIO(in io.Reader, out io.Writer, message string, defaultYes bool) bool {
	if out != nil {
		fmt.Fprint(out, message)
	}

	text, err := readPromptLine(in)
	if err != nil {
		return false
	}

	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return defaultYes
	}
	return text == "y" || text == "yes"
}

// readPromptLine reads until either LF or CR so Enter works in normal and raw terminal modes.
func readPromptLine(in io.Reader) (string, error) {
	if in == nil {
		return "", io.EOF
	}

	var buf []byte
	var one [1]byte

	for {
		n, err := in.Read(one[:])
		if n > 0 {
			switch one[0] {
			case '\n', '\r':
				return string(buf), nil
			default:
				buf = append(buf, one[0])
			}
		}

		if err != nil {
			if err == io.EOF && len(buf) > 0 {
				return string(buf), nil
			}
			return string(buf), err
		}
	}
}
