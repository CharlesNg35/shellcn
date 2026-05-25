// Command agent is the shellcn-agent reverse-tunnel proxy.
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		server      string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&server, "server", "", "gateway URL to dial back to")
	flag.Parse()

	if showVersion {
		fmt.Printf("shellcn-agent %s\n", version)
		return
	}

	fmt.Fprintf(os.Stderr, "shellcn-agent %s — server=%q (agent not yet implemented)\n", version, server)
}
