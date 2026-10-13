package spin

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func intPtr(n int) *int { return &n }

// testKey is a key shaped like a real one (a folder, 16 hex digits, an
// extension). The cleaning functions do not look at the shape.
func testKey(i int) string { return fmt.Sprintf("entries/u1/%016x.jpg", i) }

func photosOf(n int) []NewPhoto {
	list := make([]NewPhoto, n)
	for i := range list {
		list[i] = NewPhoto{Key: testKey(i + 1), Source: PhotoSourceLibrary}
	}
	return list
}

func goodInput() EntryInput {
	return EntryInput{Title: "Dune", Stars: 7, Photos: photosOf(1)}
}

func wantProblem(t *testing.T, err error, field, reason string) {
	t.Helper()
	var fp *FieldProblem
	if !errors.As(err, &fp) {
		t.Fatalf("error = %v (%T), want a *FieldProblem %s/%s", err, err, field, reason)
	}
	if fp.Field != field || fp.Reason != reason {
		t.Fatalf("problem = %s/%s, want %s/%s", fp.Field, fp.Reason, field, reason)
	}
}

// --- the six reactions ---

func TestReactions_TheSixNamesInOrder(t *testing.T) {
	want := []string{"love", "fire", "laughing", "wow", "sad", "sleepy"}
	if !reflect.DeepEqual(Reactions, want) {
		t.Fatalf("Reactions = %v, want %v", Reactions, want)
	}
	for _, name := range want {
		if !IsReaction(name) {
			t.Errorf("IsReaction(%q) = false", name)
		}
	}
	for _, name := range []string{"", "heart", "clap", "yum", "Love", "LOVE", " love", "love ", "thumbsdown", "\U0001F60D"} {
		if IsReaction(name) {
			t.Errorf("IsReaction(%q) = true, want false", name)
		}
	}
}

// Plan 03-02's check script reads entry.go as text and compares the quoted
// names of this one literal, so it must stay on one line, once.
func TestReactions_LiteralIsOneLineInEntryGo(t *testing.T) {
	src, err := os.ReadFile("entry.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if n := strings.Count(text, "Reactions = []string{"); n != 1 {
		t.Fatalf("entry.go has %d slice literals named Reactions, want exactly 1", n)
	}
	m := regexp.MustCompile(`(?m)^var Reactions = \[\]string\{([^}\n]*)\}$`).FindStringSubmatch(text)
	if m == nil {
		t.Fatal("the Reactions literal is not on one line of the form: var Reactions = []string{...}")
	}
	var names []string
	for _, q := range regexp.MustCompile(`"([a-z]+)"`).FindAllStringSubmatch(m[1], -1) {
		names = append(names, q[1])
	}
	if !reflect.DeepEqual(names, Reactions) {
		t.Fatalf("names read from the text = %v, Reactions = %v", names, Reactions)
	}
}

// --- ContentTypeOfKey ---

func TestContentTypeOfKey(t *testing.T) {
	cases := []struct {
		key  string
		want string
		ok   bool
	}{
		{"entries/u1/0123456789abcdef.jpg", "image/jpeg", true},
		{"entries/u1/0123456789abcdef.png", "image/png", true},
		{"entries/u1/0123456789abcdef.webp", "image/webp", true},
		{"a.jpg", "image/jpeg", true},
		{"entries/u1/0123456789abcdef.gif", "", false},
		{"entries/u1/0123456789abcdef.heic", "", false},
		{"entries/u1/0123456789abcdef.jpeg", "", false},
		{"entries/u1/0123456789abcdef.JPG", "", false},
		{"entries/u1/0123456789abcdef", "", false},
		{"entries/u1/0123456789abcdef.", "", false},
		{"entries/u1/0123456789abcdef.jpg.exe", "", false},
		{"entries.jpg/u1/0123456789abcdef", "", false},
		{"jpg", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := ContentTypeOfKey(c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("ContentTypeOfKey(%q) = %q, %v; want %q, %v", c.key, got, ok, c.want, c.ok)
		}
	}
}

// --- CleanEntryInput ---

func TestCleanEntryInput_ACompleteEntryIsTidied(t *testing.T) {
	in := EntryInput{
		Title:    "  Dune \n",
		Stars:    7,
		Reaction: ptr("fire"),
		Comment:  ptr("  So good\r\nSee it twice  "),
		Photos: []NewPhoto{
			{Key: testKey(1), Source: PhotoSourceCamera, SizeBytes: intPtr(2345678)},
			{Key: testKey(2), Source: PhotoSourceLibrary},
		},
	}
	got, err := CleanEntryInput(in)
	if err != nil {
		t.Fatalf("CleanEntryInput: %v", err)
	}
	if got.Title != "Dune" || got.Stars != 7 {
		t.Errorf("title %q stars %d", got.Title, got.Stars)
	}
	if got.Reaction == nil || *got.Reaction != "fire" {
		t.Errorf("reaction = %v", got.Reaction)
	}
	if got.Comment == nil || *got.Comment != "So good\nSee it twice" {
		t.Errorf("comment = %v", got.Comment)
	}
	if len(got.Photos) != 2 || got.Photos[0].Key != testKey(1) || got.Photos[1].Key != testKey(2) {
		t.Fatalf("photos = %+v, want the two keys in the order sent", got.Photos)
	}
	if got.Photos[0].Source != PhotoSourceCamera || got.Photos[1].Source != PhotoSourceLibrary {
		t.Errorf("sources = %q %q", got.Photos[0].Source, got.Photos[1].Source)
	}
	if got.Photos[0].SizeBytes == nil || *got.Photos[0].SizeBytes != 2345678 || got.Photos[1].SizeBytes != nil {
		t.Errorf("sizes = %v %v", got.Photos[0].SizeBytes, got.Photos[1].SizeBytes)
	}
}

func TestCleanEntryInput_DoesNotChangeTheInput(t *testing.T) {
	in := EntryInput{
		Title:    " Dune ",
		Stars:    7,
		Reaction: ptr("fire"),
		Comment:  ptr(" hi "),
		Photos:   []NewPhoto{{Key: testKey(1), Source: PhotoSourceCamera, SizeBytes: intPtr(10)}},
	}
	got, err := CleanEntryInput(in)
	if err != nil {
		t.Fatal(err)
	}
	got.Photos[0].Key = "changed"
	*got.Photos[0].SizeBytes = 99
	if in.Title != " Dune " || *in.Comment != " hi " || in.Photos[0].Key != testKey(1) || *in.Photos[0].SizeBytes != 10 {
		t.Fatalf("the input changed: %+v", in)
	}
}

func TestCleanEntryInput_Title(t *testing.T) {
	cases := []struct {
		name   string
		title  string
		reason string // "" means accepted
	}{
		{"empty", "", ProblemRequired},
		{"spaces and a tab only", "   \t ", ProblemRequired},
		{"a line break only", "\n", ProblemRequired},
		{"80 characters", strings.Repeat("a", 80), ""},
		{"81 characters", strings.Repeat("a", 81), ProblemTooLong},
		{"80 emoji count as 80 characters", strings.Repeat("\U0001F600", 80), ""},
		{"81 emoji", strings.Repeat("\U0001F600", 81), ProblemTooLong},
		{"80 characters after trimming", "  " + strings.Repeat("a", 80) + "  ", ""},
		{"a tab inside", "Du\tne", ProblemInvalid},
		{"a line break inside", "Du\nne", ProblemInvalid},
		{"a null inside", "Du\x00ne", ProblemInvalid},
		{"a delete character inside", "Du\x7fne", ProblemInvalid},
		{"a C1 control inside", "Du\u0085ne", ProblemInvalid},
		{"bytes that are not text", "Du\xffne", ProblemInvalid},
		{"accents and an emoji are fine", "Caf\u00e9 \U0001F600", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := goodInput()
			in.Title = c.title
			got, err := CleanEntryInput(in)
			if c.reason == "" {
				if err != nil {
					t.Fatalf("error = %v, want accepted", err)
				}
				if got.Title != strings.TrimSpace(c.title) {
					t.Errorf("title = %q, want %q", got.Title, strings.TrimSpace(c.title))
				}
				return
			}
			wantProblem(t, err, "title", c.reason)
		})
	}
}

func TestCleanEntryInput_Stars(t *testing.T) {
	for _, n := range []int{StarsMin, 2, 7, StarsMax} {
		in := goodInput()
		in.Stars = n
		got, err := CleanEntryInput(in)
		if err != nil || got.Stars != n {
			t.Errorf("stars %d: got %d, %v", n, got.Stars, err)
		}
	}
	for _, n := range []int{0, -1, StarsMin - 1, StarsMax + 1, 100} {
		in := goodInput()
		in.Stars = n
		_, err := CleanEntryInput(in)
		wantProblem(t, err, "stars", ProblemInvalid)
	}
}

func TestCleanEntryInput_Reaction(t *testing.T) {
	for _, name := range Reactions {
		in := goodInput()
		in.Reaction = ptr(name)
		got, err := CleanEntryInput(in)
		if err != nil || got.Reaction == nil || *got.Reaction != name {
			t.Errorf("reaction %q: got %v, %v", name, got.Reaction, err)
		}
	}
	// None: absent and empty both give nil.
	for _, r := range []*string{nil, ptr("")} {
		in := goodInput()
		in.Reaction = r
		got, err := CleanEntryInput(in)
		if err != nil || got.Reaction != nil {
			t.Errorf("no reaction: got %v, %v", got.Reaction, err)
		}
	}
	// The older names, and anything not spelled exactly, are refused.
	for _, name := range []string{"heart", "clap", "yum", "Love", "FIRE", " love", "love ", "thumbsdown", "\U0001F60D"} {
		in := goodInput()
		in.Reaction = ptr(name)
		_, err := CleanEntryInput(in)
		wantProblem(t, err, "reaction", ProblemInvalid)
	}
}

func TestCleanEntryInput_Comment(t *testing.T) {
	// Counted after the pair CR LF becomes one LF: 69 + 1 + 70 = 140.
	crlf140 := strings.Repeat("a", 69) + "\r\n" + strings.Repeat("b", 70)
	cases := []struct {
		name    string
		comment string
		want    string // the stored comment, "" for none
		reason  string // "" means accepted
	}{
		{"empty is none", "", "", ""},
		{"spaces and line breaks are none", "  \n \r\n ", "", ""},
		{"trimmed", "  hi  ", "hi", ""},
		{"140 characters", strings.Repeat("a", 140), strings.Repeat("a", 140), ""},
		{"141 characters", strings.Repeat("a", 141), "", ProblemTooLong},
		{"140 emoji", strings.Repeat("\U0001F600", 140), strings.Repeat("\U0001F600", 140), ""},
		{"141 emoji", strings.Repeat("\U0001F600", 141), "", ProblemTooLong},
		{"a CR LF pair becomes one LF", "a\r\nb", "a\nb", ""},
		{"length is counted after the pair is made one", crlf140, strings.Repeat("a", 69) + "\n" + strings.Repeat("b", 70), ""},
		{"blank lines are kept", "one\n\nthree", "one\n\nthree", ""},
		{"a tab", "a\tb", "", ProblemInvalid},
		{"a null", "a\x00b", "", ProblemInvalid},
		{"a delete character", "a\x7fb", "", ProblemInvalid},
		{"a lone CR inside is a control character", "a\rb", "", ProblemInvalid},
		{"bytes that are not text", "a\xffb", "", ProblemInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := goodInput()
			in.Comment = ptr(c.comment)
			got, err := CleanEntryInput(in)
			if c.reason != "" {
				wantProblem(t, err, "comment", c.reason)
				return
			}
			if err != nil {
				t.Fatalf("error = %v, want accepted", err)
			}
			switch {
			case c.want == "" && got.Comment != nil:
				t.Errorf("comment = %q, want none", *got.Comment)
			case c.want != "" && (got.Comment == nil || *got.Comment != c.want):
				t.Errorf("comment = %v, want %q", got.Comment, c.want)
			}
		})
	}
	// An absent comment is none.
	got, err := CleanEntryInput(goodInput())
	if err != nil || got.Comment != nil {
		t.Errorf("absent comment: %v, %v", got.Comment, err)
	}
}

func TestCleanEntryInput_PhotoCount(t *testing.T) {
	for _, n := range []int{1, 2, 10} {
		in := goodInput()
		in.Photos = photosOf(n)
		got, err := CleanEntryInput(in)
		if err != nil || len(got.Photos) != n {
			t.Errorf("%d photos: got %d, %v", n, len(got.Photos), err)
		}
	}

	in := goodInput()
	in.Photos = nil
	_, err := CleanEntryInput(in)
	wantProblem(t, err, "photos", ProblemRequired)

	in.Photos = []NewPhoto{}
	_, err = CleanEntryInput(in)
	wantProblem(t, err, "photos", ProblemRequired)

	in.Photos = photosOf(11)
	_, err = CleanEntryInput(in)
	wantProblem(t, err, "photos", ProblemTooMany)

	// The count is checked before any item: eleven with a bad one inside is
	// still too_many.
	in.Photos = photosOf(11)
	in.Photos[3].Source = "gallery"
	_, err = CleanEntryInput(in)
	wantProblem(t, err, "photos", ProblemTooMany)
}

func TestCleanEntryInput_PhotoOrderIsKept(t *testing.T) {
	in := goodInput()
	in.Photos = photosOf(10)
	// Put the keys in a scrambled order: it must come back as sent.
	in.Photos[0], in.Photos[9] = in.Photos[9], in.Photos[0]
	in.Photos[2], in.Photos[5] = in.Photos[5], in.Photos[2]
	got, err := CleanEntryInput(in)
	if err != nil {
		t.Fatal(err)
	}
	for i := range in.Photos {
		if got.Photos[i].Key != in.Photos[i].Key {
			t.Fatalf("position %d is %q, want %q", i, got.Photos[i].Key, in.Photos[i].Key)
		}
	}
}

func TestCleanEntryInput_PhotoItems(t *testing.T) {
	key200 := strings.Repeat("k", MaxPhotoKeyBytes)
	key201 := strings.Repeat("k", MaxPhotoKeyBytes+1)
	cases := []struct {
		name   string
		photos []NewPhoto
		reason string // "" means accepted
	}{
		{"a key twice", []NewPhoto{{Key: "k1", Source: "camera"}, {Key: "k1", Source: "library"}}, ProblemInvalid},
		{"the same key far apart", []NewPhoto{{Key: "k1", Source: "camera"}, {Key: "k2", Source: "camera"}, {Key: "k1", Source: "camera"}}, ProblemInvalid},
		{"an empty key", []NewPhoto{{Key: "", Source: "camera"}}, ProblemInvalid},
		{"a 200 byte key", []NewPhoto{{Key: key200, Source: "camera"}}, ""},
		{"a 201 byte key", []NewPhoto{{Key: key201, Source: "camera"}}, ProblemInvalid},
		{"200 bytes that are 100 two byte characters", []NewPhoto{{Key: strings.Repeat("\u00e9", 100), Source: "camera"}}, ""},
		{"101 two byte characters are over 200 bytes", []NewPhoto{{Key: strings.Repeat("\u00e9", 101), Source: "camera"}}, ProblemInvalid},
		{"a key with a control character", []NewPhoto{{Key: "k\x00", Source: "camera"}}, ProblemInvalid},
		{"a key that is not text", []NewPhoto{{Key: "k\xff", Source: "camera"}}, ProblemInvalid},
		{"source camera", []NewPhoto{{Key: "k1", Source: "camera"}}, ""},
		{"source library", []NewPhoto{{Key: "k1", Source: "library"}}, ""},
		{"no source", []NewPhoto{{Key: "k1", Source: ""}}, ProblemInvalid},
		{"another source", []NewPhoto{{Key: "k1", Source: "gallery"}}, ProblemInvalid},
		{"source in capitals", []NewPhoto{{Key: "k1", Source: "Camera"}}, ProblemInvalid},
		{"a bad second item", []NewPhoto{{Key: "k1", Source: "camera"}, {Key: "k2", Source: "x"}}, ProblemInvalid},
		{"no declared size", []NewPhoto{{Key: "k1", Source: "camera"}}, ""},
		{"size 1", []NewPhoto{{Key: "k1", Source: "camera", SizeBytes: intPtr(1)}}, ""},
		{"size 50 MiB", []NewPhoto{{Key: "k1", Source: "camera", SizeBytes: intPtr(52428800)}}, ""},
		{"size 0", []NewPhoto{{Key: "k1", Source: "camera", SizeBytes: intPtr(0)}}, ProblemInvalid},
		{"size below 0", []NewPhoto{{Key: "k1", Source: "camera", SizeBytes: intPtr(-5)}}, ProblemInvalid},
		{"size 50 MiB plus 1", []NewPhoto{{Key: "k1", Source: "camera", SizeBytes: intPtr(52428801)}}, ProblemInvalid},
		// The shape of a key is the HTTP layer's check and its extension is
		// the service's, so an opaque key passes here.
		{"a key with no shape is not judged here", []NewPhoto{{Key: "anything", Source: "camera"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := goodInput()
			in.Photos = c.photos
			_, err := CleanEntryInput(in)
			if c.reason == "" {
				if err != nil {
					t.Fatalf("error = %v, want accepted", err)
				}
				return
			}
			wantProblem(t, err, "photos", c.reason)
		})
	}
}

func TestCleanEntryInput_FirstProblemInFieldOrder(t *testing.T) {
	bad := EntryInput{Title: "", Stars: 0, Reaction: ptr("clap"), Comment: ptr(strings.Repeat("a", 141)), Photos: nil}
	_, err := CleanEntryInput(bad)
	wantProblem(t, err, "title", ProblemRequired)

	bad.Title = "Dune"
	_, err = CleanEntryInput(bad)
	wantProblem(t, err, "stars", ProblemInvalid)

	bad.Stars = 7
	_, err = CleanEntryInput(bad)
	wantProblem(t, err, "reaction", ProblemInvalid)

	bad.Reaction = nil
	_, err = CleanEntryInput(bad)
	wantProblem(t, err, "comment", ProblemTooLong)

	bad.Comment = nil
	_, err = CleanEntryInput(bad)
	wantProblem(t, err, "photos", ProblemRequired)

	bad.Photos = photosOf(1)
	if _, err = CleanEntryInput(bad); err != nil {
		t.Fatalf("a fixed entry is refused: %v", err)
	}
}

// --- CleanEntryPatch ---

func TestCleanEntryPatch_EmptyPatchChangesNothing(t *testing.T) {
	got, err := CleanEntryPatch(EntryPatch{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != nil || got.Stars != nil || got.Reaction != nil || got.Comment != nil || got.Photos != nil {
		t.Fatalf("an empty patch came back with fields: %+v", got)
	}
}

func TestCleanEntryPatch_PresentFieldsAreCleaned(t *testing.T) {
	refs := []PhotoRef{{ID: "p1"}, {Key: "k9", Source: PhotoSourceCamera, SizeBytes: intPtr(12)}}
	p := EntryPatch{
		Title:    ptr("  New title "),
		Stars:    intPtr(3),
		Reaction: ptr("sleepy"),
		Comment:  ptr(" a\r\nb "),
		Photos:   &refs,
	}
	got, err := CleanEntryPatch(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title == nil || *got.Title != "New title" {
		t.Errorf("title = %v", got.Title)
	}
	if got.Stars == nil || *got.Stars != 3 {
		t.Errorf("stars = %v", got.Stars)
	}
	if got.Reaction == nil || *got.Reaction != "sleepy" {
		t.Errorf("reaction = %v", got.Reaction)
	}
	if got.Comment == nil || *got.Comment != "a\nb" {
		t.Errorf("comment = %v", got.Comment)
	}
	if got.Photos == nil || len(*got.Photos) != 2 || (*got.Photos)[0].ID != "p1" || (*got.Photos)[1].Key != "k9" || *(*got.Photos)[1].SizeBytes != 12 {
		t.Errorf("photos = %+v", got.Photos)
	}
	// The input is not changed.
	if *p.Title != "  New title " || *p.Comment != " a\r\nb " {
		t.Errorf("the input changed: %q %q", *p.Title, *p.Comment)
	}
}

func TestCleanEntryPatch_EmptyReactionAndCommentClear(t *testing.T) {
	for _, comment := range []string{"", "   ", "\n"} {
		got, err := CleanEntryPatch(EntryPatch{Reaction: ptr(""), Comment: ptr(comment)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Reaction == nil || *got.Reaction != "" {
			t.Errorf("reaction = %v, want a pointer to an empty string (clear)", got.Reaction)
		}
		if got.Comment == nil || *got.Comment != "" {
			t.Errorf("comment %q = %v, want a pointer to an empty string (clear)", comment, got.Comment)
		}
	}
}

func TestCleanEntryPatch_FieldProblems(t *testing.T) {
	refs := func(list ...PhotoRef) *[]PhotoRef { return &list }
	cases := []struct {
		name   string
		patch  EntryPatch
		field  string
		reason string
	}{
		{"an empty title cannot clear it", EntryPatch{Title: ptr("")}, "title", ProblemRequired},
		{"a blank title", EntryPatch{Title: ptr("  ")}, "title", ProblemRequired},
		{"a long title", EntryPatch{Title: ptr(strings.Repeat("a", 81))}, "title", ProblemTooLong},
		{"a title with a tab", EntryPatch{Title: ptr("a\tb")}, "title", ProblemInvalid},
		{"stars 0", EntryPatch{Stars: intPtr(0)}, "stars", ProblemInvalid},
		{"stars 11", EntryPatch{Stars: intPtr(11)}, "stars", ProblemInvalid},
		{"an older reaction", EntryPatch{Reaction: ptr("clap")}, "reaction", ProblemInvalid},
		{"a long comment", EntryPatch{Comment: ptr(strings.Repeat("a", 141))}, "comment", ProblemTooLong},
		{"a comment with a tab", EntryPatch{Comment: ptr("a\tb")}, "comment", ProblemInvalid},
		{"an empty photo list", EntryPatch{Photos: refs()}, "photos", ProblemRequired},
		{"a title is checked before photos", EntryPatch{Title: ptr(""), Photos: refs()}, "title", ProblemRequired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CleanEntryPatch(c.patch)
			wantProblem(t, err, c.field, c.reason)
		})
	}
}

func TestCleanEntryPatch_PhotoList(t *testing.T) {
	refs := func(list ...PhotoRef) EntryPatch { return EntryPatch{Photos: &list} }
	ids := func(n int) []PhotoRef {
		list := make([]PhotoRef, n)
		for i := range list {
			list[i] = PhotoRef{ID: fmt.Sprintf("p%d", i+1)}
		}
		return list
	}
	cases := []struct {
		name   string
		patch  EntryPatch
		reason string // "" means accepted
	}{
		{"one kept photo", refs(PhotoRef{ID: "p1"}), ""},
		{"one new photo", refs(PhotoRef{Key: "k1", Source: "library"}), ""},
		{"kept and new", refs(PhotoRef{ID: "p1"}, PhotoRef{Key: "k1", Source: "camera"}, PhotoRef{ID: "p2"}), ""},
		{"ten kept", EntryPatch{Photos: func() *[]PhotoRef { l := ids(10); return &l }()}, ""},
		{"eleven", EntryPatch{Photos: func() *[]PhotoRef { l := ids(11); return &l }()}, ProblemTooMany},
		{"an item with both an id and a key", refs(PhotoRef{ID: "p1", Key: "k1", Source: "camera"}), ProblemInvalid},
		{"an item with neither", refs(PhotoRef{ID: "p1"}, PhotoRef{Source: "camera"}), ProblemInvalid},
		{"an empty item", refs(PhotoRef{}), ProblemInvalid},
		{"an id twice", refs(PhotoRef{ID: "p1"}, PhotoRef{ID: "p2"}, PhotoRef{ID: "p1"}), ProblemInvalid},
		{"a key twice", refs(PhotoRef{Key: "k1", Source: "camera"}, PhotoRef{Key: "k1", Source: "camera"}), ProblemInvalid},
		{"a new photo with no source", refs(PhotoRef{Key: "k1"}), ProblemInvalid},
		{"a new photo with another source", refs(PhotoRef{Key: "k1", Source: "gallery"}), ProblemInvalid},
		{"a key of 201 bytes", refs(PhotoRef{Key: strings.Repeat("k", 201), Source: "camera"}), ProblemInvalid},
		{"a key of 200 bytes", refs(PhotoRef{Key: strings.Repeat("k", 200), Source: "camera"}), ""},
		{"a new photo with size 0", refs(PhotoRef{Key: "k1", Source: "camera", SizeBytes: intPtr(0)}), ProblemInvalid},
		{"a new photo with size 50 MiB plus 1", refs(PhotoRef{Key: "k1", Source: "camera", SizeBytes: intPtr(52428801)}), ProblemInvalid},
		// A kept photo is named by its id alone; what else it carries is not used.
		{"a kept photo that also carries a source", refs(PhotoRef{ID: "p1", Source: "camera"}), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CleanEntryPatch(c.patch)
			if c.reason == "" {
				if err != nil {
					t.Fatalf("error = %v, want accepted", err)
				}
				if got.Photos == nil || len(*got.Photos) != len(*c.patch.Photos) {
					t.Fatalf("photos = %+v", got.Photos)
				}
				return
			}
			wantProblem(t, err, "photos", c.reason)
		})
	}
}

func TestCleanEntryPatch_PhotoOrderIsKeptAsSent(t *testing.T) {
	list := []PhotoRef{{ID: "c"}, {Key: "new1", Source: "camera"}, {ID: "a"}, {ID: "b"}, {Key: "new2", Source: "library"}}
	got, err := CleanEntryPatch(EntryPatch{Photos: &list})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range *got.Photos {
		order = append(order, p.ID+p.Key)
	}
	want := []string{"c", "new1", "a", "b", "new2"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	// The cleaned list is a copy.
	(*got.Photos)[0].ID = "changed"
	if list[0].ID != "c" {
		t.Errorf("the input list changed: %+v", list[0])
	}
}

func TestCleanEntryPatch_AbsentPhotoListIsNotTheEmptyList(t *testing.T) {
	// Absent leaves the photos alone; an empty list would remove them all, so
	// it is refused.
	got, err := CleanEntryPatch(EntryPatch{Title: ptr("New")})
	if err != nil || got.Photos != nil {
		t.Fatalf("absent list: %+v, %v", got.Photos, err)
	}
	empty := []PhotoRef{}
	_, err = CleanEntryPatch(EntryPatch{Photos: &empty})
	wantProblem(t, err, "photos", ProblemRequired)
}

// --- the errors ---

func TestFieldProblem_IsAPlainErrorWithFixedText(t *testing.T) {
	var err error = &FieldProblem{Field: "photos", Reason: ProblemInvalid}
	if got := err.Error(); got != "spin: entry field photos is not valid (invalid)" {
		t.Errorf("text = %q", got)
	}
	var fp *FieldProblem
	if !errors.As(err, &fp) || fp.Field != "photos" || fp.Reason != "invalid" {
		t.Errorf("errors.As failed: %+v", fp)
	}
	// The cleaning functions give a real nil on success, not a typed nil.
	if _, err := CleanEntryInput(goodInput()); err != nil {
		t.Errorf("CleanEntryInput on a good entry: %v", err)
	}
	if _, err := CleanEntryPatch(EntryPatch{}); err != nil {
		t.Errorf("CleanEntryPatch on an empty patch: %v", err)
	}
}

func TestEntrySentinelsAreDistinctAndPlain(t *testing.T) {
	all := []error{ErrNoSpin, ErrSpinChanged, ErrEntryNotFound, ErrClosed, ErrDaySettled, ErrNotOpen}
	for i, a := range all {
		if a == nil || a.Error() == "" {
			t.Fatalf("sentinel %d is empty", i)
		}
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("%q matches %q", a, b)
			}
		}
	}
}

// --- the Today summary ---

func TestEntryRefOf(t *testing.T) {
	if EntryRefOf(nil) != nil {
		t.Fatal("EntryRefOf(nil) is not nil")
	}

	reaction := "fire"
	e := &Entry{
		ID:        "e1",
		SpinID:    "s1",
		LocalDate: Date{Year: 2026, Month: time.October, Day: 7},
		Title:     "Dune",
		Stars:     7,
		Reaction:  &reaction,
		Comment:   ptr("a comment the summary leaves out"),
		Late:      true,
		Photos: []EntryPhoto{
			{ID: "p2", Position: 1, Key: "entries/u1/bbbbbbbbbbbbbbbb.jpg"},
			{ID: "p3", Position: 2, Key: "entries/u1/cccccccccccccccc.png"},
			{ID: "p1", Position: 0, Key: "entries/u1/aaaaaaaaaaaaaaaa.webp"},
		},
	}
	ref := EntryRefOf(e)
	if ref == nil {
		t.Fatal("EntryRefOf returned nil")
	}
	if ref.ID != "e1" || !ref.Late || ref.Title != "Dune" || ref.Stars != 7 {
		t.Errorf("ref = %+v", ref)
	}
	if ref.PhotoCount != 3 {
		t.Errorf("PhotoCount = %d, want 3", ref.PhotoCount)
	}
	if ref.CoverKey != "entries/u1/aaaaaaaaaaaaaaaa.webp" {
		t.Errorf("CoverKey = %q, want the key at position 0", ref.CoverKey)
	}
	if ref.PhotoURL != "" {
		t.Errorf("PhotoURL = %q, want empty until the HTTP layer fills it", ref.PhotoURL)
	}
	if ref.Reaction == nil || *ref.Reaction != "fire" {
		t.Errorf("Reaction = %v", ref.Reaction)
	}
	// The summary does not share the entry's reaction pointer.
	*ref.Reaction = "wow"
	if reaction != "fire" {
		t.Error("changing the summary changed the entry")
	}

	// No reaction and a single photo.
	one := EntryRefOf(&Entry{ID: "e2", Photos: []EntryPhoto{{Position: 0, Key: "k"}}})
	if one.Reaction != nil || one.PhotoCount != 1 || one.CoverKey != "k" {
		t.Errorf("one = %+v", one)
	}
}
