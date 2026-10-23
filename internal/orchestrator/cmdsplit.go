// Shell-free command splitting (R2c): tests.md command cells and
// hook command_templates are written as shell-ish strings, but the
// harness NEVER routes through a shell (anti-injection rule). This
// splitter turns such a string into an argv with POSIX-ish quoting
// (single quotes literal, double quotes with \" and \\ escapes,
// backslash escapes outside quotes) and REFUSES shell operators
// instead of silently misinterpreting them: a command that needs a
// pipe, a redirect, command substitution or chaining is reported
// with autotest_shell_syntax_unsupported so a human rewrites it as
// a plain argv (or a script file invoked as argv[0]).
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
)

// SplitCommandWords splits one command string into argv. Empty
// input yields an empty argv and no error.
func SplitCommandWords(s string) ([]string, error) {
	var (
		argv    []string
		cur     strings.Builder
		started bool // current word has begun (quotes can start an empty word)
	)
	flush := func() {
		if started {
			argv = append(argv, cur.String())
			cur.Reset()
			started = false
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		case c == '\'':
			started = true
			j := i + 1
			for j < len(runes) && runes[j] != '\'' {
				cur.WriteRune(runes[j])
				j++
			}
			if j >= len(runes) {
				return nil, core.NewError(core.ReasonAutoTestUnparseable,
					"SplitCommandWords", "unbalanced single quote in command: "+s)
			}
			i = j
		case c == '"':
			started = true
			j := i + 1
			closed := false
			for j < len(runes) {
				if runes[j] == '\\' && j+1 < len(runes) && (runes[j+1] == '"' || runes[j+1] == '\\') {
					cur.WriteRune(runes[j+1])
					j += 2
					continue
				}
				if runes[j] == '"' {
					closed = true
					break
				}
				if runes[j] == '`' || (runes[j] == '$' && j+1 < len(runes) && runes[j+1] == '(') {
					return nil, core.NewError(core.ReasonAutoTestShellSyntax,
						"SplitCommandWords", "command substitution is shell syntax and is not executed: "+s)
				}
				cur.WriteRune(runes[j])
				j++
			}
			if !closed {
				return nil, core.NewError(core.ReasonAutoTestUnparseable,
					"SplitCommandWords", "unbalanced double quote in command: "+s)
			}
			i = j
		case c == '\\':
			if i+1 >= len(runes) {
				return nil, core.NewError(core.ReasonAutoTestUnparseable,
					"SplitCommandWords", "trailing backslash in command: "+s)
			}
			started = true
			cur.WriteRune(runes[i+1])
			i++
		case c == '|' || c == '&' || c == ';' || c == '<' || c == '>' || c == '`':
			return nil, core.NewError(core.ReasonAutoTestShellSyntax,
				"SplitCommandWords", "shell operator '"+string(c)+"' is not executed (no shell): "+s)
		case c == '$' && i+1 < len(runes) && runes[i+1] == '(':
			return nil, core.NewError(core.ReasonAutoTestShellSyntax,
				"SplitCommandWords", "command substitution is shell syntax and is not executed: "+s)
		default:
			started = true
			cur.WriteRune(c)
		}
	}
	flush()
	return argv, nil
}
