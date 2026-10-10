// Package language is the registry of programming languages Lightning can run.
//
// Language IDs are the contract shared by the API, the web UI and the runtime
// images: an ID maps to the image <prefix><id>:<tag>.
package language

import (
	"fmt"
	"slices"
	"time"
)

// Language describes a runnable language.
type Language struct {
	ID       string // stable identifier, e.g. "python"
	Name     string // display name
	Version  string // toolchain version baked into the runtime image
	Monaco   string // Monaco editor language id
	FileName string // source file name inside the sandbox

	// MinMemoryMB raises the sandbox memory limit for toolchains that need
	// more than the configured default (compilers, the JVM).
	MinMemoryMB int
	// CompileTimeout is added to the run timeout for languages with a build step.
	CompileTimeout time.Duration
}

// Image returns the runtime image reference for the language.
func (l Language) Image(prefix, tag string) string {
	return prefix + l.ID + ":" + tag
}

// MemoryMB returns the sandbox memory limit given the configured default.
func (l Language) MemoryMB(defaultMB int) int {
	return max(defaultMB, l.MinMemoryMB)
}

// Timeout returns the wall-clock limit of a run given the configured base timeout.
func (l Language) Timeout(base time.Duration) time.Duration {
	return base + l.CompileTimeout
}

const compileAllowance = 5 * time.Second

// all lists every supported language in display order.
// Versions must match the runtime Dockerfiles under runtimes/.
var all = []Language{
	{ID: "python", Name: "Python", Version: "3.14", Monaco: "python", FileName: "main.py"},
	{ID: "javascript", Name: "JavaScript", Version: "Node.js 24", Monaco: "javascript", FileName: "main.js"},
	{ID: "typescript", Name: "TypeScript", Version: "Bun 1.4", Monaco: "typescript", FileName: "main.ts"},
	{ID: "go", Name: "Go", Version: "1.27", Monaco: "go", FileName: "main.go", MinMemoryMB: 512, CompileTimeout: compileAllowance},
	{ID: "rust", Name: "Rust", Version: "1.98", Monaco: "rust", FileName: "main.rs", MinMemoryMB: 512, CompileTimeout: compileAllowance},
	{ID: "c", Name: "C", Version: "GCC 15", Monaco: "c", FileName: "main.c", CompileTimeout: compileAllowance},
	{ID: "cpp", Name: "C++", Version: "GCC 15", Monaco: "cpp", FileName: "main.cpp", CompileTimeout: compileAllowance},
	{ID: "java", Name: "Java", Version: "25", Monaco: "java", FileName: "Main.java", MinMemoryMB: 512, CompileTimeout: compileAllowance},
	{ID: "ruby", Name: "Ruby", Version: "3.4", Monaco: "ruby", FileName: "main.rb"},
	{ID: "php", Name: "PHP", Version: "8.4", Monaco: "php", FileName: "main.php"},
	{ID: "bash", Name: "Bash", Version: "5.3", Monaco: "shell", FileName: "main.sh"},
}

// All returns every supported language in display order.
func All() []Language {
	return slices.Clone(all)
}

// Registry is the set of languages enabled for a deployment.
type Registry struct {
	langs []Language
}

// NewRegistry enables the languages with the given IDs, or all of them when
// ids is empty. Languages keep the canonical display order.
func NewRegistry(ids []string) (*Registry, error) {
	if len(ids) == 0 {
		return &Registry{langs: All()}, nil
	}
	for _, id := range ids {
		if !slices.ContainsFunc(all, func(l Language) bool { return l.ID == id }) {
			return nil, fmt.Errorf("unknown language %q", id)
		}
	}
	var langs []Language
	for _, l := range all {
		if slices.Contains(ids, l.ID) {
			langs = append(langs, l)
		}
	}
	return &Registry{langs: langs}, nil
}

// List returns the enabled languages in display order.
func (r *Registry) List() []Language {
	return slices.Clone(r.langs)
}

// Get returns the enabled language with the given ID.
func (r *Registry) Get(id string) (Language, bool) {
	i := slices.IndexFunc(r.langs, func(l Language) bool { return l.ID == id })
	if i < 0 {
		return Language{}, false
	}
	return r.langs[i], true
}

// MaxCompileTimeout returns the largest compile allowance among enabled languages.
func (r *Registry) MaxCompileTimeout() time.Duration {
	var d time.Duration
	for _, l := range r.langs {
		d = max(d, l.CompileTimeout)
	}
	return d
}
