package audit

import (
	"os"
	"path/filepath"
	"testing"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent. It carries the Loader's stand-in for the
// helper of task 4b4f (assumption A7): TABLEAUX_CORPUS, else
// ../tableaux/corpus/build beside the checkout.
func corpus(t testing.TB) string {
	t.Helper()
	dir := os.Getenv("TABLEAUX_CORPUS")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "tableaux", "corpus", "build")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Skipf("the built corpus is absent at %s; set TABLEAUX_CORPUS", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
