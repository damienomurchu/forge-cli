package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDatabasePath(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{
			name: "override takes precedence on Linux",
			goos: "linux",
			env: map[string]string{
				"FORGE_DATA_DIR": "/srv/forge-data",
				"XDG_DATA_HOME":  "/home/test/.xdg-data",
				"HOME":           "/home/test",
			},
			want: "/srv/forge-data/forge.db",
		},
		{
			name: "override takes precedence on unsupported platform",
			goos: "windows",
			env:  map[string]string{"FORGE_DATA_DIR": "/forge-data"},
			want: "/forge-data/forge.db",
		},
		{
			name: "Linux XDG data home",
			goos: "linux",
			env: map[string]string{
				"XDG_DATA_HOME": "/home/test/.xdg-data",
				"HOME":          "/home/test",
			},
			want: "/home/test/.xdg-data/forge/forge.db",
		},
		{
			name: "Linux home fallback",
			goos: "linux",
			env:  map[string]string{"HOME": "/home/test"},
			want: "/home/test/.local/share/forge/forge.db",
		},
		{
			name: "macOS application support",
			goos: "darwin",
			env: map[string]string{
				"XDG_DATA_HOME": "/ignored",
				"HOME":          "/Users/test",
			},
			want: "/Users/test/Library/Application Support/forge/forge.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveDatabasePath(tt.goos, mapEnvironment(tt.env))
			if err != nil {
				t.Fatalf("ResolveDatabasePath() error = %v", err)
			}
			if got != filepath.Clean(tt.want) {
				t.Errorf("ResolveDatabasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDatabasePathRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		env     map[string]string
		wantErr string
	}{
		{name: "relative override", goos: "linux", env: map[string]string{"FORGE_DATA_DIR": "data"}, wantErr: "FORGE_DATA_DIR must be an absolute path"},
		{name: "relative XDG data home", goos: "linux", env: map[string]string{"XDG_DATA_HOME": ".data", "HOME": "/home/test"}, wantErr: "XDG_DATA_HOME must be an absolute path"},
		{name: "missing Linux home", goos: "linux", env: nil, wantErr: "HOME is required to resolve the Forge database path"},
		{name: "relative Linux home", goos: "linux", env: map[string]string{"HOME": "home/test"}, wantErr: "HOME must be an absolute path"},
		{name: "missing macOS home", goos: "darwin", env: nil, wantErr: "HOME is required to resolve the Forge database path"},
		{name: "relative macOS home", goos: "darwin", env: map[string]string{"HOME": "Users/test"}, wantErr: "HOME must be an absolute path"},
		{name: "unsupported platform", goos: "windows", env: nil, wantErr: "unsupported operating system \"windows\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveDatabasePath(tt.goos, mapEnvironment(tt.env))
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ResolveDatabasePath() error = %v, want %q", err, tt.wantErr)
			}
			if got != "" {
				t.Errorf("ResolveDatabasePath() = %q, want empty", got)
			}
		})
	}
}

func TestResolveDatabasePathDoesNotCreateResolvedPath(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "not-created")
	got, err := ResolveDatabasePath("linux", mapEnvironment(map[string]string{
		"FORGE_DATA_DIR": dataDir,
	}))
	if err != nil {
		t.Fatalf("ResolveDatabasePath() error = %v", err)
	}
	if got != filepath.Join(dataDir, databaseName) {
		t.Errorf("ResolveDatabasePath() = %q", got)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("resolved data directory state error = %v, want not exist", err)
	}
}

func mapEnvironment(values map[string]string) func(string) string {
	return func(name string) string {
		return values[name]
	}
}
