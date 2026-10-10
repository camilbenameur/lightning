package language

import (
	"regexp"
	"slices"
	"testing"
	"time"
)

func ids(langs []Language) []string {
	out := make([]string, len(langs))
	for i, l := range langs {
		out[i] = l.ID
	}
	return out
}

func TestAllMatchesContract(t *testing.T) {
	want := []string{"python", "javascript", "typescript", "go", "rust", "c", "cpp", "java", "ruby", "php", "bash"}
	if got := ids(All()); !slices.Equal(got, want) {
		t.Fatalf("language ids = %v, want %v", got, want)
	}
	validID := regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	for _, l := range All() {
		if !validID.MatchString(l.ID) || l.Name == "" || l.Version == "" || l.Monaco == "" || l.FileName == "" {
			t.Errorf("incomplete language definition: %+v", l)
		}
	}
}

func TestAllReturnsCopy(t *testing.T) {
	langs := All()
	langs[0].ID = "mutated"
	if All()[0].ID != "python" {
		t.Fatal("All must not expose the package-level slice")
	}
}

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		want    []string
		wantErr bool
	}{
		{name: "empty enables all", ids: nil, want: ids(All())},
		{name: "keeps canonical order", ids: []string{"bash", "python", "go"}, want: []string{"python", "go", "bash"}},
		{name: "duplicates are harmless", ids: []string{"c", "c"}, want: []string{"c"}},
		{name: "unknown id", ids: []string{"python", "cobol"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRegistry(tt.ids)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(r.List()); !slices.Equal(got, tt.want) {
				t.Fatalf("List() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryGet(t *testing.T) {
	r, err := NewRegistry([]string{"python", "java"})
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := r.Get("java"); !ok || l.FileName != "Main.java" {
		t.Fatalf("Get(java) = %+v, %v", l, ok)
	}
	if _, ok := r.Get("go"); ok {
		t.Fatal("disabled language must not be found")
	}
	if _, ok := r.Get(""); ok {
		t.Fatal("empty id must not be found")
	}
}

func TestLimits(t *testing.T) {
	python, _ := (&Registry{langs: All()}).Get("python")
	java, _ := (&Registry{langs: All()}).Get("java")

	if got := python.MemoryMB(256); got != 256 {
		t.Errorf("python memory = %d, want 256", got)
	}
	if got := java.MemoryMB(256); got != 512 {
		t.Errorf("java memory = %d, want 512", got)
	}
	if got := java.MemoryMB(1024); got != 1024 {
		t.Errorf("java memory with larger default = %d, want 1024", got)
	}
	if got := python.Timeout(10 * time.Second); got != 10*time.Second {
		t.Errorf("python timeout = %v", got)
	}
	if got := java.Timeout(10 * time.Second); got != 15*time.Second {
		t.Errorf("java timeout = %v", got)
	}
	if got := python.Image("ghcr.io/x/lightning-runtime-", "v1"); got != "ghcr.io/x/lightning-runtime-python:v1" {
		t.Errorf("image = %q", got)
	}
}

func TestMaxCompileTimeout(t *testing.T) {
	interpreted, _ := NewRegistry([]string{"python", "ruby"})
	if got := interpreted.MaxCompileTimeout(); got != 0 {
		t.Errorf("interpreted only = %v, want 0", got)
	}
	all, _ := NewRegistry(nil)
	if got := all.MaxCompileTimeout(); got != compileAllowance {
		t.Errorf("all = %v, want %v", got, compileAllowance)
	}
}
