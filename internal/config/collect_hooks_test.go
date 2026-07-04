package config

import "testing"

func TestBranchMatches(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		branch   string
		want     bool
	}{
		{"empty patterns always match", nil, "anything", true},
		{"exact match", []string{"main"}, "main", true},
		{"glob match", []string{"feature/*"}, "feature/x", true},
		{"glob no match", []string{"feature/*"}, "main", false},
		{"one of several", []string{"main", "release/*"}, "release/1", true},
		{"none match", []string{"feature/*"}, "fix/y", false},
	}
	for _, c := range cases {
		if got := BranchMatches(c.patterns, c.branch); got != c.want {
			t.Errorf("%s: BranchMatches(%v, %q) = %v, want %v", c.name, c.patterns, c.branch, got, c.want)
		}
	}
}

func cmds(hooks []NamedHook) []string {
	out := make([]string, len(hooks))
	for i, h := range hooks {
		out[i] = h.Command
	}
	return out
}

func TestCollectHooks(t *testing.T) {
	project := &Config{
		Hooks: Hooks{PostCreate: "project-simple"},
		NamedHooks: ProjectNamedHooks{
			PostCreate: []NamedHook{
				{Name: "feat-only", Command: "feat-cmd", Branches: []string{"feature/*"}},
				{Name: "disabled", Command: "no", Disabled: true},
			},
		},
	}
	user := &UserConfig{
		NamedHooks: NamedHooksConfig{
			PostCreate: []NamedHook{{Name: "user-global", Command: "user-cmd"}},
		},
	}

	t.Run("feature branch: user first, then project, disabled skipped", func(t *testing.T) {
		got := CollectHooks(project, user, HookPostCreate, "feature/x")
		want := []string{"user-cmd", "project-simple", "feat-cmd"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", cmds(got), want)
		}
		for i := range want {
			if got[i].Command != want[i] {
				t.Errorf("hook %d = %q, want %q (order %v)", i, got[i].Command, want[i], cmds(got))
			}
		}
	})

	t.Run("main: branch-filtered hook excluded", func(t *testing.T) {
		got := CollectHooks(project, user, HookPostCreate, "main")
		if want := []string{"user-cmd", "project-simple"}; len(got) != len(want) {
			t.Fatalf("got %v, want %v", cmds(got), want)
		}
	})

	t.Run("nil user config is fine", func(t *testing.T) {
		got := CollectHooks(project, nil, HookPostCreate, "feature/x")
		if len(got) != 2 { // project-simple + feat-cmd
			t.Errorf("got %v, want 2 project hooks", cmds(got))
		}
	})
}
