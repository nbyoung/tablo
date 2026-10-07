// Package validate is the Validator: it applies the rules of the method's
// RULES.md to a project the Loader read and returns diagnostics, each with a
// rule id, a severity and a position. Files applies the rules that the files
// alone decide, and Derived the rules that read derived facts, which arrive as
// plain data. The Loader keeps the rules of reading, L1 to L4, and P4. The
// package reads no file, no history, no environment and no clock, and both
// functions are safe for concurrent use.
package validate
