// Package conformance holds the conformance test of tablo: it runs the built
// command over every entry of the tableaux conformance corpus and compares
// what the command emits with the entry's expected.yaml, the findings by rule
// and location and then each derived fact the file states.
//
// The package holds tests only. TestCorpus drives the command as a black box
// and reads its JSON alone; observe_test.go turns that JSON into a document of
// expected.yaml's own shape, and compare_test.go compares the two generically,
// so the assumptions about the command's data sit in one table. known.yaml
// lists the entries that wait for a fix. The environment variables are
// TABLO_CORPUS (another corpus directory), TABLO_BIN (a built command),
// TABLO_KEEP (keep the work directory) and TABLO_REPORT (a file for the report).
package conformance
