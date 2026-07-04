package config

import "path/filepath"

// BranchMatches reports whether branch is matched by a list of glob patterns.
// An empty pattern list matches every branch (the hook is unconditional).
func BranchMatches(patterns []string, branch string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, branch); ok {
			return true
		}
	}
	return false
}

// CollectHooks returns the hooks to run for hookType on the given branch:
// user-level named hooks first, then the project's hooks (simple + named),
// filtered by each hook's optional branch globs and skipping disabled ones.
// A nil user or project config is treated as empty.
func CollectHooks(project *Config, user *UserConfig, hookType HookType, branch string) []NamedHook {
	var all []NamedHook
	if user != nil {
		all = append(all, user.GetNamedHooks(hookType)...)
	}
	if project != nil {
		all = append(all, project.GetAllHooks(hookType)...)
	}

	out := make([]NamedHook, 0, len(all))
	for _, h := range all {
		if h.Disabled {
			continue
		}
		if !BranchMatches(h.Branches, branch) {
			continue
		}
		out = append(out, h)
	}
	return out
}
