package herdr

// catalog_invariant_test.go pins the invariant the jetkvm, cua, punktfunk, gh and omarchy
// siblings keep and this plugin needs for the same reason: every method the SCHEMA advertises
// must be reachable — routed by runVerbHerdrResolved's dispatch — and every method the
// dispatch routes must be in the schema. Without it a method can sit in the catalog as a
// phantom (advertised but unimplemented): the schema validates the authored `herdr: …` step,
// and the step then fails only at dispatch with `unknown herdr method`, so the catalog and
// the skill that documents it actively mislead the agent that dispatches on it.
//
// The test reads the REAL sources — the SERVED enum (through the CUE API,
// schema_served_test.go) and verb.go's switch — rather than restating either, so they cannot
// drift alongside the code they guard.

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// methodArm matches one `case "a", "b":` arm. The arms are scoped to runVerbHerdrResolved
// first, so the `switch req.GetClass()` in plugin.go and any other switch in the package are
// never mistaken for the verb's dispatch.
var (
	verbSwitch = regexp.MustCompile(`(?s)func runVerbHerdrResolved\(.*?\n\}`)
	caseArm    = regexp.MustCompile(`(?m)^[ \t]*case[ \t]+((?:"(?:[^"\\]|\\.)*"[ \t]*,?[ \t]*)+):`)
	quotedWord = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
)

// routedMethods returns the methods runVerbHerdrResolved routes.
func routedMethods(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("verb.go")
	if err != nil {
		t.Fatalf("read the verb: %v", err)
	}
	fn := verbSwitch.FindString(string(b))
	if fn == "" {
		t.Fatal("could not locate runVerbHerdrResolved in verb.go")
	}
	seen := map[string]bool{}
	for _, arm := range caseArm.FindAllStringSubmatch(fn, -1) {
		for _, q := range quotedWord.FindAllStringSubmatch(arm[1], -1) {
			seen[q[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for w := range seen {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// schemaMethods returns the sorted method names the SERVED #HerdrInput.method enum accepts.
// It reads the schema through the CUE API rather than its text: the enum is now §10.1 S3 (a
// vocabulary table plus a derived word list), and a text parse of the union would find ZERO
// methods and leave every invariant below passing vacuously.
func schemaMethods(t *testing.T) []string {
	t.Helper()
	return fieldWordList(t, servedSchema(t), "HerdrInput", "method")
}

// TestCatalogHasNoPhantomMethods asserts every schema method is dispatched — no method is
// advertised that the verb cannot act on.
func TestCatalogHasNoPhantomMethods(t *testing.T) {
	reachable := map[string]bool{}
	for _, m := range routedMethods(t) {
		reachable[m] = true
	}
	if len(reachable) == 0 {
		t.Fatal("the dispatch surface is empty — the invariant would pass over nothing")
	}
	for _, m := range schemaMethods(t) {
		if !reachable[m] {
			t.Errorf("schema method %q is a PHANTOM: runVerbHerdrResolved does not route it — add an implementation or remove it from the schema", m)
		}
	}
}

// TestDispatchHasNoPhantomArms is the other direction: an arm for a word the schema no longer
// accepts is dead code, and it hides a rename that only half landed — the arm would keep an
// unreachable path compiling.
func TestDispatchHasNoPhantomArms(t *testing.T) {
	advertised := map[string]bool{}
	for _, m := range schemaMethods(t) {
		advertised[m] = true
	}
	for _, m := range routedMethods(t) {
		if !advertised[m] {
			t.Errorf("runVerbHerdrResolved routes %q, which the served #HerdrInput.method enum does NOT accept — the arm is unreachable", m)
		}
	}
}

// TestEverySchemaMethodIsRouted is the two-way set equality in one assertion, so a reader sees
// the whole invariant at once.
func TestEverySchemaMethodIsRouted(t *testing.T) {
	schema := schemaMethods(t)
	routed := routedMethods(t)
	if len(schema) != len(routed) {
		t.Fatalf("served enum has %d methods but runVerbHerdrResolved routes %d — the catalog and the dispatch have drifted", len(schema), len(routed))
	}
	for i := range schema {
		if schema[i] != routed[i] {
			t.Errorf("served method %q has no matching routed arm (routed=%v)", schema[i], routed)
			break
		}
	}
}
