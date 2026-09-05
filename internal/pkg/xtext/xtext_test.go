package xtext

import "testing"

func TestIsCJKName(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  bool
	}{
		{"empty", []string{"", ""}, false},
		{"western", []string{"John", "Doe"}, false},
		{"chinese first", []string{"伟", "张"}, true},
		{"chinese last", []string{"Wei", "张"}, true},
		{"japanese", []string{"太郎", "山田"}, true},
		{"korean", []string{"민수", "김"}, true},
		{"mixed", []string{"Wei", "Zhang"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCJKName(tt.names...); got != tt.want {
				t.Errorf("IsCJKName(%v) = %v, want %v", tt.names, got, tt.want)
			}
		})
	}
}

func TestFormatUserName(t *testing.T) {
	tests := []struct {
		first, last string
		want        string
	}{
		{"John", "Doe", "John Doe"},
		{"伟", "张", "张伟"},
		{"太郎", "山田", "山田太郎"},
		{"민수", "김", "김민수"},
		{"John", "", "John"},
		{"", "Doe", "Doe"},
		{"", "", ""},
		{"  John  ", "  Doe  ", "John Doe"},
	}

	for _, tt := range tests {
		t.Run(tt.first+"_"+tt.last, func(t *testing.T) {
			if got := FormatUserName(tt.first, tt.last); got != tt.want {
				t.Errorf("FormatUserName(%q, %q) = %q, want %q", tt.first, tt.last, got, tt.want)
			}
		})
	}
}
