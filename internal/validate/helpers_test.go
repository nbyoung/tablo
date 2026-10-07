package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent. It stands in for the helper of task 4b4f, as
// the Loader's tests do: TABLEAUX_CORPUS, else ../tableaux/corpus/build
// beside the checkout.
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

// beside returns a path of the corpus beside its build directory: the rule
// table RULES.md and the entries with their expected.yaml. It skips the test
// when the path is absent.
func beside(t testing.TB, path ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{filepath.Dir(corpus(t))}, path...)...)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("the corpus holds no %s beside its build", filepath.Join(path...))
	}
	return p
}
