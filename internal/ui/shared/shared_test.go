package shared

import (
	"testing"
	"time"

	"github.com/jinguo998/agent-sessions/internal/domain"
)

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{name: "just now", when: time.Now().Add(-30 * time.Second), want: "just now"},
		{name: "minutes", when: time.Now().Add(-5 * time.Minute), want: "5m ago"},
		{name: "hours", when: time.Now().Add(-3 * time.Hour), want: "3h ago"},
		{name: "days", when: time.Now().Add(-49 * time.Hour), want: "2d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelativeTime(tt.when); got != tt.want {
				t.Fatalf("RelativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayWidthAndFormatting(t *testing.T) {
	if got := DisplayWidth("a你b"); got != 4 {
		t.Fatalf("DisplayWidth() = %d, want 4", got)
	}
	if got := TruncateToWidth("你好世界", 5); got != "你..." {
		t.Fatalf("TruncateToWidth(cjk) = %q, want %q", got, "你...")
	}
	if got := TruncateToWidth("hello", 0); got != "" {
		t.Fatalf("TruncateToWidth(0) = %q, want empty", got)
	}
	if got := TruncateToWidth("hello", 2); got != ".." {
		t.Fatalf("TruncateToWidth(2) = %q, want ..", got)
	}
	if got := PadToWidth("你a", 5); got != "你a  " {
		t.Fatalf("PadToWidth() = %q, want %q", got, "你a  ")
	}
	if got := WrapText("hello world", 6); got != "hello \nworld" {
		t.Fatalf("WrapText() = %q, want %q", got, "hello \nworld")
	}
}

func TestSourceColorsFallback(t *testing.T) {
	if got := SourceColors(domain.SourceClaude); got.Light != "27" || got.Dark != "75" {
		t.Fatalf("Claude palette = %#v", got)
	}
	if got := SourceColors(domain.Source("unknown")); got != fallbackSourcePalette {
		t.Fatalf("unknown palette = %#v, want fallback %#v", got, fallbackSourcePalette)
	}
}
