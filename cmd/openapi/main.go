// Command openapi writes the OpenAPI document of the HTTP API to stdout, without starting the server.
package main

import (
	"fmt"
	"os"

	"pkgsilo/app"
)

func main() {
	// The trailing newline keeps the output stable under the end-of-file-fixer hook.
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", app.Docs().JSON()); err != nil {
		// The process exits either way, so a failed write has nowhere left to be reported.
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
