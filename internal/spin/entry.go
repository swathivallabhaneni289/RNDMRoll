package spin

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Entry limits. The phone enforces the same numbers, the server checks them
// again and the database holds them as check constraints.
const (
	// MaxTitleRunes is the longest title, counted in characters after
	// trimming.
	MaxTitleRunes = 80
	// MaxCommentRunes is the longest comment, counted in characters after line
	// breaks are made plain.
	MaxCommentRunes = 140
	// StarsMin and StarsMax bound the stored star value, a whole number from 1
	// to 10: 3.5 stars is stored as 7.
	StarsMin = 1
	StarsMax = 10
	// MinEntryPhotos and MaxEntryPhotos bound the photos of one entry.
	MinEntryPhotos = 1
	MaxEntryPhotos = 10
	// MaxPhotoKeyBytes is the longest storage key a photo may carry.
	MaxPhotoKeyBytes = 200
	// MaxPhotoSizeBytes is the largest size a phone may declare for a photo,
	// 50 MiB. The size is for information only: the server never reads the
	// file.
	MaxPhotoSizeBytes = 52428800
)

// Where a photo came from. It is stored with the photo and never shown.
const (
	PhotoSourceCamera  = "camera"
	PhotoSourceLibrary = "library"
)

// The JSON names of the entry fields, used in a FieldProblem.
const (
	fieldTitle    = "title"
	fieldStars    = "stars"
	fieldReaction = "reaction"
	fieldComment  = "comment"
	fieldPhotos   = "photos"
)

// Reactions is the fixed list of reaction names, in the order the phone shows
// them. The server and the database hold the names only, never an emoji.
var Reactions = []string{"love", "fire", "laughing", "wow", "sad", "sleepy"}

// IsReaction reports whether name is one of the six reaction names, spelled
// exactly.
func IsReaction(name string) bool {
	return slices.Contains(Reactions, name)
}

// Entry is a logged entry as it is stored. It belongs to one spin (the date's
// latest spin when it was posted); CategoryID and CategoryName are that
// spin's, as written on the day. Late is the posted day's late flag. Photos
// is one to ten photos in position order and position 0 is the cover.
type Entry struct {
	ID           string
	SpinID       string
	LocalDate    Date
	CategoryID   string
	CategoryName string
	Title        string
	Stars        int
	Reaction     *string
	Comment      *string
	Late         bool
	Archived     bool
	CommentsOff  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Photos       []EntryPhoto
}

// EntryPhoto is one stored photo of an entry. Key is the storage key (a
// folder and a file name), never a link. ContentType comes from the key's
// extension. SizeBytes is what the phone declared, or nil. Source is
// PhotoSourceCamera or PhotoSourceLibrary.
type EntryPhoto struct {
	ID          string
	Position    int
	Key         string
	ContentType string
	SizeBytes   *int
	Source      string
}

// NewPhoto is a photo a caller adds: the key of a file the person uploaded
// with a ticket, where it came from and, optionally, the size the phone
// declared.
type NewPhoto struct {
	Key       string
	Source    string
	SizeBytes *int
}

// EntryInput is a new entry as a caller sends it. Reaction and Comment are
// optional (nil or an empty string means none) and Photos is the new uploads
// in order; the first is the cover.
type EntryInput struct {
	Title    string
	Stars    int
	Reaction *string
	Comment  *string
	Photos   []NewPhoto
}

// PhotoRef is one item of the photo list an edit sends. It is exactly one of
// an existing photo of the entry, named by ID, or a new upload, named by Key
// with its Source (and optionally SizeBytes).
type PhotoRef struct {
	ID        string
	Key       string
	Source    string
	SizeBytes *int
}

// EntryPatch is an edit. Every field is a pointer and nil means unchanged. For
// Reaction and Comment a pointer to an empty string clears the value. Photos
// is the whole ordered list the entry should end with: a photo left out is
// removed.
type EntryPatch struct {
	Title    *string
	Stars    *int
	Reaction *string
	Comment  *string
	Photos   *[]PhotoRef
	// CommentsOff, when set, switches comments on the entry off or on.
	CommentsOff *bool
}

// CleanEntryInput checks and tidies a new entry before anything is written.
// It returns the cleaned copy (the title and comment trimmed, an empty
// reaction or comment as nil) or a *FieldProblem, as a plain error, for the
// first field that fails. Fields are checked in the order title, stars,
// reaction, comment, photos. The input is not changed.
//
//   - title: trimmed, 1 to 80 characters, no control characters;
//   - stars: 1 to 10;
//   - reaction: one of the six names, or empty for none;
//   - comment: trimmed, empty is none, at most 140 characters, no control
//     characters except line breaks (a CR LF pair becomes one LF);
//   - photos: 1 to 10 (none is required, more is too_many), each with a
//     non-empty key of at most 200 bytes, source camera or library and a size
//     that is nil or 1 to 52428800, and no key twice.
//
// The shape and owner of a key are checked by the HTTP layer, and the
// extension of a key by the service, which needs the content type.
func CleanEntryInput(in EntryInput) (EntryInput, error) {
	title, reason := cleanTitle(in.Title)
	if reason != "" {
		return EntryInput{}, problem(fieldTitle, reason)
	}
	if !validStars(in.Stars) {
		return EntryInput{}, problem(fieldStars, ProblemInvalid)
	}
	out := EntryInput{Title: title, Stars: in.Stars}
	if in.Reaction != nil {
		name, reason := cleanReaction(*in.Reaction)
		if reason != "" {
			return EntryInput{}, problem(fieldReaction, reason)
		}
		if name != "" {
			out.Reaction = &name
		}
	}
	if in.Comment != nil {
		text, reason := cleanComment(*in.Comment)
		if reason != "" {
			return EntryInput{}, problem(fieldComment, reason)
		}
		if text != "" {
			out.Comment = &text
		}
	}
	photos, reason := cleanNewPhotos(in.Photos)
	if reason != "" {
		return EntryInput{}, problem(fieldPhotos, reason)
	}
	out.Photos = photos
	return out, nil
}

// CleanEntryPatch checks and tidies an edit with the same rules as
// CleanEntryInput, for the fields that are present. A present Reaction or
// Comment that is empty (after trimming, for the comment) comes back as a
// pointer to an empty string, which means clear. A present photo list must
// hold 1 to 10 items and each item must be exactly one of an ID or a Key; no
// ID and no key may repeat. Whether an ID belongs to the entry is the
// service's check. The input is not changed.
func CleanEntryPatch(p EntryPatch) (EntryPatch, error) {
	var out EntryPatch
	if p.Title != nil {
		title, reason := cleanTitle(*p.Title)
		if reason != "" {
			return EntryPatch{}, problem(fieldTitle, reason)
		}
		out.Title = &title
	}
	if p.Stars != nil {
		if !validStars(*p.Stars) {
			return EntryPatch{}, problem(fieldStars, ProblemInvalid)
		}
		stars := *p.Stars
		out.Stars = &stars
	}
	if p.Reaction != nil {
		name, reason := cleanReaction(*p.Reaction)
		if reason != "" {
			return EntryPatch{}, problem(fieldReaction, reason)
		}
		out.Reaction = &name
	}
	if p.Comment != nil {
		text, reason := cleanComment(*p.Comment)
		if reason != "" {
			return EntryPatch{}, problem(fieldComment, reason)
		}
		out.Comment = &text
	}
	if p.CommentsOff != nil {
		off := *p.CommentsOff
		out.CommentsOff = &off
	}
	if p.Photos != nil {
		refs, reason := cleanPhotoRefs(*p.Photos)
		if reason != "" {
			return EntryPatch{}, problem(fieldPhotos, reason)
		}
		out.Photos = &refs
	}
	return out, nil
}

// cleanTitle returns the title as it is stored, or the reason it is refused.
func cleanTitle(s string) (string, string) {
	if !utf8.ValidString(s) {
		return "", ProblemInvalid
	}
	title := strings.TrimSpace(s)
	if hasControl(title, false) {
		return "", ProblemInvalid
	}
	switch n := utf8.RuneCountInString(title); {
	case n == 0:
		return "", ProblemRequired
	case n > MaxTitleRunes:
		return "", ProblemTooLong
	}
	return title, ""
}

// cleanReaction returns the reaction name ("" for none) or the reason it is
// refused. The name is not trimmed or lowered: it must be spelled exactly.
func cleanReaction(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	if !IsReaction(s) {
		return "", ProblemInvalid
	}
	return s, ""
}

// cleanComment returns the comment as it is stored ("" for none) or the
// reason it is refused. Line breaks are made plain first, so the length is
// the length of the stored text.
func cleanComment(s string) (string, string) {
	if !utf8.ValidString(s) {
		return "", ProblemInvalid
	}
	text := strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if text == "" {
		return "", ""
	}
	if hasControl(text, true) {
		return "", ProblemInvalid
	}
	if utf8.RuneCountInString(text) > MaxCommentRunes {
		return "", ProblemTooLong
	}
	return text, ""
}

// hasControl reports whether s holds a control character. A line feed is
// allowed when newlineOK is true.
func hasControl(s string, newlineOK bool) bool {
	for _, r := range s {
		if unicode.IsControl(r) && !(newlineOK && r == '\n') {
			return true
		}
	}
	return false
}

func validStars(n int) bool { return n >= StarsMin && n <= StarsMax }

// validPhotoKey is the cleaner's whole rule for a key: not empty, at most 200
// bytes, text and no control characters.
func validPhotoKey(key string) bool {
	return key != "" && len(key) <= MaxPhotoKeyBytes && utf8.ValidString(key) && !hasControl(key, false)
}

func validPhotoSource(s string) bool {
	return s == PhotoSourceCamera || s == PhotoSourceLibrary
}

// validPhotoSize accepts nil or a declared size from 1 byte to 50 MiB.
func validPhotoSize(n *int) bool {
	return n == nil || (*n >= 1 && *n <= MaxPhotoSizeBytes)
}

func copyInt(n *int) *int {
	if n == nil {
		return nil
	}
	v := *n
	return &v
}

// checkPhotoCount returns the reason a list of this length is refused, or "".
func checkPhotoCount(n int) string {
	switch {
	case n < MinEntryPhotos:
		return ProblemRequired
	case n > MaxEntryPhotos:
		return ProblemTooMany
	}
	return ""
}

// cleanNewPhotos checks the photo list of a new entry and returns a copy. The
// count is checked before any item.
func cleanNewPhotos(list []NewPhoto) ([]NewPhoto, string) {
	if reason := checkPhotoCount(len(list)); reason != "" {
		return nil, reason
	}
	out := make([]NewPhoto, len(list))
	keys := make(map[string]bool, len(list))
	for i, p := range list {
		if !validPhotoKey(p.Key) || !validPhotoSource(p.Source) || !validPhotoSize(p.SizeBytes) || keys[p.Key] {
			return nil, ProblemInvalid
		}
		keys[p.Key] = true
		out[i] = NewPhoto{Key: p.Key, Source: p.Source, SizeBytes: copyInt(p.SizeBytes)}
	}
	return out, ""
}

// cleanPhotoRefs checks the photo list of an edit and returns a copy in the
// order sent. An item with an ID names a kept photo and its other fields are
// not used; an item with a Key is a new upload and needs a source.
func cleanPhotoRefs(list []PhotoRef) ([]PhotoRef, string) {
	if reason := checkPhotoCount(len(list)); reason != "" {
		return nil, reason
	}
	out := make([]PhotoRef, len(list))
	ids := make(map[string]bool, len(list))
	keys := make(map[string]bool, len(list))
	for i, p := range list {
		hasID, hasKey := p.ID != "", p.Key != ""
		if hasID == hasKey || !validPhotoSize(p.SizeBytes) {
			return nil, ProblemInvalid
		}
		if hasID {
			if ids[p.ID] {
				return nil, ProblemInvalid
			}
			ids[p.ID] = true
		} else {
			if !validPhotoKey(p.Key) || !validPhotoSource(p.Source) || keys[p.Key] {
				return nil, ProblemInvalid
			}
			keys[p.Key] = true
		}
		out[i] = PhotoRef{ID: p.ID, Key: p.Key, Source: p.Source, SizeBytes: copyInt(p.SizeBytes)}
	}
	return out, ""
}

// ContentTypeOfKey returns the image content type a storage key's extension
// stands for: jpg, png and webp only, in lower case. ok is false for any
// other key. A stored photo's content type always comes from here, so the
// phone never has to say it twice.
func ContentTypeOfKey(key string) (contentType string, ok bool) {
	dot := strings.LastIndexByte(key, '.')
	if dot < 0 {
		return "", false
	}
	switch key[dot+1:] {
	case "jpg":
		return "image/jpeg", true
	case "png":
		return "image/png", true
	case "webp":
		return "image/webp", true
	}
	return "", false
}

// EntryRefOf builds the summary Today carries for an entry, or nil for nil.
// CoverKey is the key of the photo at position 0 (the lowest position, so the
// order of the slice does not matter) and PhotoCount is how many photos the
// entry has. The link is not built here.
func EntryRefOf(e *Entry) *EntryRef {
	if e == nil {
		return nil
	}
	ref := &EntryRef{
		ID:          e.ID,
		Late:        e.Late,
		Archived:    e.Archived,
		CommentsOff: e.CommentsOff,
		Title:       e.Title,
		Stars:       e.Stars,
		PhotoCount:  len(e.Photos),
	}
	if e.Reaction != nil {
		reaction := *e.Reaction
		ref.Reaction = &reaction
	}
	if len(e.Photos) > 0 {
		cover := e.Photos[0]
		for _, p := range e.Photos[1:] {
			if p.Position < cover.Position {
				cover = p
			}
		}
		ref.CoverKey = cover.Key
	}
	return ref
}
