// Command 8118 is the validator prototype: it reads a Tableaux project
// with a small YAML reader, checks each file against the embedded schemas,
// then applies the structural rules of README.md as code.
//
//	go run ./prototype/8118 <repository>
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: 8118 <repository>")
		os.Exit(2)
	}
	diags, err := Validate(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := 0
	for _, d := range diags {
		fmt.Println(d)
		if d.Severity == "error" {
			code = 1
		}
	}
	os.Exit(code)
}
