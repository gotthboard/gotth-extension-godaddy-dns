package main

import (
	"fmt"
	"os"

	"github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns"
	extensioncore "github.com/gotthboard/gotth-extensions/pkg/extensions"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gotth-extension-godaddy-dns-manifest <version>")
		os.Exit(2)
	}
	manifest, err := godaddydns.Manifest(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid extension version")
		os.Exit(1)
	}
	encoded, err := extensioncore.CanonicalManifest(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "manifest generation failed")
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(encoded)
}
