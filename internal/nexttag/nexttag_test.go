package nexttag

import "testing"

func TestNext(t *testing.T) {
	tests := []struct {
		last     string
		subjects []string
		want     string
		wantErr  bool
	}{
		{"v0.3.0", nil, "v0.3.0", false},
		{"v0.3.0", []string{"chore: docs"}, "v0.3.1", false},
		{"v0.3.0", []string{"fix: windows path"}, "v0.3.1", false},
		{"v0.3.0", []string{"feat: hermetic CLIs"}, "v0.4.0", false},
		{"v0.3.0", []string{"chore: x", "feat: y"}, "v0.4.0", false},
		{"v0.3.0", []string{"feat!: break"}, "v0.4.0", false},
		{"v1.2.3", []string{"fix: n"}, "v1.2.4", false},
		{"0.3.0", []string{"feat: x"}, "v0.4.0", false},
		{"v0.9.7", []string{"feat: x"}, "v0.10.0", false},
		{"bogus", []string{"feat: x"}, "", true},
	}
	for _, tt := range tests {
		got, err := Next(tt.last, tt.subjects)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Next(%q) err=nil", tt.last)
			}
			continue
		}
		if err != nil {
			t.Errorf("Next(%q)=%v", tt.last, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Next(%q, %q)=%q want %q", tt.last, tt.subjects, got, tt.want)
		}
	}
}
