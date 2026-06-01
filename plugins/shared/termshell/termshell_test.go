package termshell

import (
	"errors"
	"strings"
	"testing"
)

func TestCommandsFallbackOnlyForDefaultTTY(t *testing.T) {
	defaults := Commands("", true)
	if len(defaults) != 3 || defaults[0][0] != "/bin/sh" || defaults[1][0] != "/bin/bash" || defaults[2][0] != "/busybox/sh" {
		t.Fatalf("default commands = %v", defaults)
	}
	explicit := Commands("/bin/zsh", true)
	if len(explicit) != 1 || explicit[0][0] != "/bin/zsh" {
		t.Fatalf("explicit commands = %v", explicit)
	}
	plain := Commands("", false)
	if len(plain) != 1 || plain[0][0] != "/bin/sh" {
		t.Fatalf("non-TTY commands = %v", plain)
	}
}

func TestDisplayErrorRemovesRepeatedKubernetesInternalPrefix(t *testing.T) {
	err := errors.New(`Internal error occurred: Internal error occurred: error executing command in container: failed`)
	if got := DisplayError(err); got != "error executing command in container: failed" {
		t.Fatalf("DisplayError = %q", got)
	}
}

func TestWriteExecErrorKeepsFullMessage(t *testing.T) {
	msg := strings.Repeat("failed to exec in container: ", 8) + `exec: "/bin/sh": stat /bin/sh: no such file or directory`
	var out strings.Builder
	WriteExecError(&out, errors.New(msg))
	if !strings.Contains(out.String(), msg) {
		t.Fatalf("terminal error = %q, want full message", out.String())
	}
}
