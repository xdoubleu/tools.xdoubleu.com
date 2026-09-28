// Command kobo-sign creates the self-update signing key and signs release
// binaries:
//
//	kobo-sign keygen        prints a public key and private seed
//	kobo-sign sign <file>   writes <file>.sig using $KOBO_GATEWAY_SIGNING_KEY
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"tools.xdoubleu.com/kobo-gateway/internal/updatesig"
)

func main() {
	err := run(os.Args[1:], os.Getenv("KOBO_GATEWAY_SIGNING_KEY"), os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kobo-sign:", err)
		os.Exit(1)
	}
}

func run(args []string, signingKey string, stdout io.Writer) error {
	switch {
	case len(args) == 1 && args[0] == "keygen":
		pub, seed, err := updatesig.GenerateKey()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout,
			"public key (repo variable KOBO_GATEWAY_UPDATE_PUBKEY): %s\n"+
				"private key (repo secret KOBO_GATEWAY_SIGNING_KEY): %s\n",
			pub, seed)
		return err
	case len(args) == 2 && args[0] == "sign":
		data, err := os.ReadFile(args[1]) //nolint:gosec //CLI arg names the file to sign
		if err != nil {
			return err
		}
		sig, err := updatesig.Sign(signingKey, data)
		if err != nil {
			return err
		}
		//nolint:gosec //the signature is public
		return os.WriteFile(args[1]+".sig", []byte(sig+"\n"), 0o644)
	default:
		return errors.New("usage: kobo-sign keygen | kobo-sign sign <file>")
	}
}
