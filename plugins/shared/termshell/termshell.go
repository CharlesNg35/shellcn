// Package termshell builds the command for an interactive container/pod shell.
package termshell

import (
	"fmt"
	"io"
	"strings"
)

// Launch prefers an interactive bash (most images ship it) and falls back to
// POSIX sh. It sets a sane TERM and, since minimal images carry no terminfo,
// aliases clear/reset to raw ANSI so screen-clearing works regardless. When the
// container runs as a UID with no /etc/passwd entry (arbitrary-UID images), it
// names the user so prompts/tools don't report "I have no name!".
const Launch = `export TERM="${TERM:-xterm-256color}"
if ! id -un >/dev/null 2>&1 && [ -w /etc/passwd ]; then
echo "shellcn:x:$(id -u):$(id -g):shellcn:${HOME:-/}:/bin/sh" >>/etc/passwd
fi
if command -v bash >/dev/null 2>&1; then
rc="$(mktemp 2>/dev/null || echo /tmp/.shellcn_bashrc)"
cat >"$rc" <<'SHRC'
alias clear='printf "\033[H\033[2J\033[3J"'
alias reset='printf "\033c"'
PS1='\w \$ '
SHRC
exec bash --rcfile "$rc"
fi
exec sh`

// Command resolves an exec command. An explicit request runs verbatim (split on
// whitespace); otherwise a TTY session gets the friendly interactive shell and a
// non-TTY session a plain shell.
func Command(request string, tty bool) []string {
	if c := strings.TrimSpace(request); c != "" {
		return strings.Fields(c)
	}
	if tty {
		return []string{"/bin/sh", "-c", Launch}
	}
	return []string{"/bin/sh"}
}

func Commands(request string, tty bool) [][]string {
	if strings.TrimSpace(request) != "" || !tty {
		return [][]string{Command(request, tty)}
	}
	return [][]string{
		Command("", true),
		{"/bin/bash", "-lc", Launch},
		{"/busybox/sh", "-c", Launch},
	}
}

func MissingExecutableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "stat /bin/sh") ||
		strings.Contains(msg, "stat /bin/bash")
}

func DisplayError(err error) string {
	msg := strings.TrimSpace(err.Error())
	for {
		next := strings.TrimSpace(strings.TrimPrefix(msg, "Internal error occurred:"))
		if next == msg {
			return msg
		}
		msg = next
	}
}

func WriteExecError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "\r\nexec failed: %s\r\n", DisplayError(err))
}
