package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileLoggerInitPermissions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing bool
		filePerm os.FileMode
		dirPerm  os.FileMode
	}{
		{"new defaults", false, 0, 0},
		{"new private file", false, 0600, 0},
		{"new private directory", false, 0600, 0700},
		{"new group directory", false, 0640, 0750},
		{"existing defaults", true, 0, 0},
		{"existing private file", true, 0600, 0750},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "logs")
			if tc.existing {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			l, err := NewFileLogger(&FileLoggerConfig{Path: dir, FilePerm: tc.filePerm, DirPerm: tc.dirPerm})
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Init(); err != nil {
				t.Fatalf("could not initialize usable log directory: %v", err)
			}
			t.Cleanup(func() { _ = l.logFile.Close() })
			entry := NewLogEntry(InfoLogLevel, "permissions", "persisted", "", nil)
			l.handler(&entry)
			if err := l.logFile.Sync(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(l.logFile.Name())
			if err != nil {
				t.Fatal(err)
			}
			var persisted LogEntry
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Message != entry.Message {
				t.Fatalf("persisted message %q, want %q", persisted.Message, entry.Message)
			}
			if runtime.GOOS == "windows" {
				return
			}
			dirInfo, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			wantDir := tc.dirPerm
			if wantDir == 0 {
				wantDir = 0755
			}
			if tc.existing {
				wantDir = 0700
			}
			if mode := dirInfo.Mode().Perm(); mode&0700 != 0700 || mode&^wantDir != 0 {
				t.Errorf("directory permissions %04o, want searchable permissions within %04o", mode, wantDir)
			}
			fileInfo, err := l.logFile.Stat()
			if err != nil {
				t.Fatal(err)
			}
			wantFile := tc.filePerm
			if wantFile == 0 {
				wantFile = 0644
			}
			if mode := fileInfo.Mode().Perm(); mode&0600 != 0600 || mode&^wantFile != 0 {
				t.Errorf("file permissions %04o, want read/write permissions within %04o", mode, wantFile)
			}
		})
	}
}
