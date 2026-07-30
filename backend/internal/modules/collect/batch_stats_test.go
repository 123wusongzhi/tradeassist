package collect

import (
	"strings"
	"testing"
)

func TestCollectFailureHintSameURLSucceededDistinguishesTaskType(t *testing.T) {
	t.Run("single task does not suggest batch tuning", func(t *testing.T) {
		hint := collectFailureHint("PARSE_FAILED", "taobao_tmall", true, false)

		if strings.Contains(hint, "批量") {
			t.Fatalf("single-task hint must not mention batch collection: %q", hint)
		}
		if !strings.Contains(hint, "采集引擎") {
			t.Fatalf("single-task hint should direct the user to the selected engine: %q", hint)
		}
	})

	t.Run("batch task keeps batch-specific recovery advice", func(t *testing.T) {
		hint := collectFailureHint("PARSE_FAILED", "taobao_tmall", true, true)

		if !strings.Contains(hint, "批量失败") || !strings.Contains(hint, "降低批量并发") {
			t.Fatalf("batch-task hint should retain batch recovery advice: %q", hint)
		}
	})
}
