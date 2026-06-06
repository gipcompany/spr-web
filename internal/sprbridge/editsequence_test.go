package sprbridge

import "testing"

func TestRewriteRebaseTodo(t *testing.T) {
	todo := "pick aaaa111 first\npick bbbb222 second\npick cccc333 third\n"
	got := RewriteRebaseTodo(todo, "bbbb222")
	want := "pick aaaa111 first\nedit bbbb222 second\npick cccc333 third\n"
	if got != want {
		t.Errorf("RewriteRebaseTodo = %q, want %q", got, want)
	}
}

func TestRewriteRebaseTodoNoMatch(t *testing.T) {
	todo := "pick aaaa111 first\n"
	if got := RewriteRebaseTodo(todo, "ffff999"); got != todo {
		t.Errorf("RewriteRebaseTodo rewrote unexpectedly: %q", got)
	}
}

func TestRewriteRebaseTodoPreservesComments(t *testing.T) {
	todo := "pick aaaa111 first\n\n# Rebase instructions\n"
	got := RewriteRebaseTodo(todo, "aaaa111")
	want := "edit aaaa111 first\n\n# Rebase instructions\n"
	if got != want {
		t.Errorf("RewriteRebaseTodo = %q, want %q", got, want)
	}
}
