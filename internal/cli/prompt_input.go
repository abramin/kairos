package cli

import (
	"bufio"
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
// When the input is a *bufio.Reader, it correctly handles \r\n sequences by consuming the
// trailing \n so it doesn't leak into the next read call.
func readPromptLine(in io.Reader) (string, error) {
	if in == nil {
		return "", io.EOF
	}

	br, isBufio := in.(*bufio.Reader)

	var buf []byte
	var one [1]byte

	for {
		n, err := in.Read(one[:])
		if n > 0 {
			switch one[0] {
			case '\n':
				return string(buf), nil
			case '\r':
				// Consume a trailing \n if already buffered (\r\n line ending).
				if isBufio && br.Buffered() > 0 {
					if next, peekErr := br.Peek(1); peekErr == nil && len(next) > 0 && next[0] == '\n' {
						br.ReadByte()
					}
				}
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
