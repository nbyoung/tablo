package cli

import _ "embed"

// usage is the text that tablo --help prints, with every command, option and
// exit code.
//
//go:embed usage.txt
var usage string
