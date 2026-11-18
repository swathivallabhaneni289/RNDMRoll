package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
)

// A removal's item sends the reason the admin gave as the report reason's
// key, never words, and leaves the key out when there is none, as every
// other kind does (developer, 2026-11-18).
func TestNotificationItemView_SendsTheReasonOnlyWhenThereIsOne(t *testing.T) {
	link := func(key string) string { return "https://photos.example.test/" + key }
	text := "Dune"
	at := time.Date(2026, 11, 18, 21, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name string
		item notifications.Item
		// want is the reason sent, "" for no key at all.
		want string
	}{
		{"a removal with a reason", notifications.Item{ID: "post_removed:a", Kind: notifications.KindPostRemoved, At: at, Text: &text,
			Ref: "1a2b3c4d", Reason: "hate"}, "hate"},
		{"a comment's removal with a reason", notifications.Item{ID: "comment_removed:b", Kind: notifications.KindCommentRemoved, At: at,
			Text: &text, Ref: "5e6f7a8b", Reason: "harassment"}, "harassment"},
		{"a removal with none", notifications.Item{ID: "post_removed:c", Kind: notifications.KindPostRemoved, At: at, Text: &text,
			Ref: "9c8d7e6f"}, ""},
		{"a post back", notifications.Item{ID: "post_back:d", Kind: notifications.KindPostBack, At: at, Text: &text}, ""},
	} {
		body, err := json.Marshal(newItemView(c.item, link, link))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var sent map[string]any
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		reason, has := sent["reason"]
		if has != (c.want != "") || (has && reason != c.want) {
			t.Fatalf("%s: %s, want the reason %q (none: no key)", c.name, body, c.want)
		}
	}
}
