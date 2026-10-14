// Package httpapi: entry.go serves the daily entry: the photo upload ticket,
// post, read, edit and delete of an entry, and the free skip. Every route is
// scoped to the caller: the person comes only from the token (finishedUser),
// an entry id is parsed in Go and always combined with that person inside the
// service, and the server decides which spin the entry belongs to, whether it
// is late and whether the day is still open.
//
// Request bodies are read with ShouldBindBodyWith and checked by hand, so
// every refusal names its field. The shape of the photo list and the owner of
// every photo key are decided here. The rules for the text fields, the counts,
// repeats and sources are the spin package's cleaners, and whether a kept
// photo id belongs to the entry is the service's. No storage key is ever
// written to a response, except the upload ticket's own photo_key.
package httpapi

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
)

// fileRemovalTimeout bounds the whole batch of file removals made after an
// edit or a delete has been saved.
const fileRemovalTimeout = 3 * time.Second

// EntryHandler serves POST /entry-photos/upload-url, POST /entries,
// GET and PATCH /entries/:id, POST /entries/:id/delete and POST /spin/skip.
type EntryHandler struct {
	svc    *spin.Service
	photos storage.EntryPhotoStore
}

// NewEntryHandler constructs an EntryHandler over the spin service (which owns
// the entry rules) and the photo store (tickets, links and file removal).
func NewEntryHandler(svc *spin.Service, photos storage.EntryPhotoStore) *EntryHandler {
	return &EntryHandler{svc: svc, photos: photos}
}

// Register mounts the routes on rg, which must carry RequireAuth and
// RequireUser (NewServer applies both to the signed-in group). Limits are per
// person, a minute: 30 upload tickets, 10 posts, 60 reads, 20 edits, 20
// deletes and 10 skips.
func (h *EntryHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/entry-photos/upload-url", perSubject(30), h.ticket)
	rg.POST("/entries", perSubject(10), h.create)
	rg.GET("/entries/:id", perSubject(60), h.get)
	rg.PATCH("/entries/:id", perSubject(20), h.patch)
	rg.POST("/entries/:id/delete", perSubject(20), h.deleteEntry)
	rg.POST("/entries/:id/archive", perSubject(20), h.archive)
	rg.POST("/entries/:id/unarchive", perSubject(20), h.unarchive)
	rg.GET("/me/archive", perSubject(60), h.archiveList)
	rg.POST("/spin/skip", perSubject(10), h.skip)
}

// EntryPhotoTicketRequest is the POST /entry-photos/upload-url body. One
// ticket is made per photo.
type EntryPhotoTicketRequest struct {
	ContentType   *string `json:"content_type"`
	ContentLength *int64  `json:"content_length"`
}

// CreateEntryRequest is the POST /entries body. Photos is read by hand: a list
// of one to ten items, each {"photo_key", "photo_source"} and optionally
// "size_bytes", in order (the first is the cover).
type CreateEntryRequest struct {
	ForDate  *string         `json:"for_date"`
	SpinSeq  *int            `json:"spin_seq"`
	Title    *string         `json:"title"`
	Stars    *int            `json:"stars"`
	Reaction *string         `json:"reaction"`
	Comment  *string         `json:"comment"`
	Photos   json.RawMessage `json:"photos"`
}

// UpdateEntryRequest is the PATCH /entries/:id body. A missing (or null) field
// is unchanged; an empty reaction or comment clears it. Photos, when present,
// is the whole ordered list the entry should end with: {"id"} keeps a photo,
// {"photo_key", "photo_source"} adds one, and a photo left out is removed.
type UpdateEntryRequest struct {
	Title    *string         `json:"title"`
	Stars    *int            `json:"stars"`
	Reaction *string         `json:"reaction"`
	Comment  *string         `json:"comment"`
	Photos   json.RawMessage `json:"photos"`
	// CommentsOff is the owner's comments switch; missing or null is unchanged.
	CommentsOff *bool `json:"comments_off"`
}

// DeleteEntryRequest is the POST /entries/:id/delete body. Nothing is read
// from it, but an oversize or malformed one is refused like on every route.
type DeleteEntryRequest struct{}

// SkipRequest is the POST /spin/skip body. Both fields are required: for_date
// is the date the phone believes it is, spin_seq the number of the spin it
// saw, so a retry or a stale screen never skips the wrong spin.
type SkipRequest struct {
	ForDate *string `json:"for_date"`
	SpinSeq *int    `json:"spin_seq"`
}

// bindEntryBody reads the JSON body into req. An empty body counts as {} (the
// bind answers io.EOF), as on the spin route. A member of the wrong JSON type
// is refused by name (400, field and reason invalid); any other failure writes
// the generic answer (400, or 413 over the size cap). On a refusal the answer
// is written and false is returned.
func bindEntryBody(c *gin.Context, req any) bool {
	err := c.ShouldBindBodyWith(req, binding.JSON)
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	// The field name comes from the request struct's own tags, never from the
	// caller's text.
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		RespondFieldError(c, typeErr.Field, reasonInvalid)
		return false
	}
	RespondValidationError(c, err)
	return false
}

// canonicalID returns the lower case form of a UUID written exactly as 36
// characters, and false for anything else. It is the same rule the service
// applies, so the two layers agree.
func canonicalID(s string) (string, bool) {
	if len(s) != 36 {
		return "", false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// entryIDOf reads the {id} of the path. Only a canonical UUID can be an
// entry's id: anything else is 404 not_found, the same answer as an entry that
// does not exist or is someone else's. It is checked right after the account
// check and before the body is read.
func entryIDOf(c *gin.Context) (string, bool) {
	id, ok := canonicalID(c.Param("id"))
	if !ok {
		respondEntryError(c, spin.ErrEntryNotFound)
		return "", false
	}
	return id, true
}

// forDateField reads a required for_date. A missing one is required and a
// value that is not YYYY-MM-DD is invalid; the answer is written on failure.
func forDateField(c *gin.Context, raw *string) (spin.Date, bool) {
	if raw == nil {
		RespondFieldError(c, "for_date", reasonRequired)
		return spin.Date{}, false
	}
	d, err := spin.ParseDate(*raw)
	if err != nil {
		RespondFieldError(c, "for_date", reasonInvalid)
		return spin.Date{}, false
	}
	return d, true
}

// spinSeqField reads a required spin_seq. A missing one is required and a
// negative one is invalid; the answer is written on failure.
func spinSeqField(c *gin.Context, raw *int) (int, bool) {
	if raw == nil {
		RespondFieldError(c, "spin_seq", reasonRequired)
		return 0, false
	}
	if *raw < 0 {
		RespondFieldError(c, "spin_seq", reasonInvalid)
		return 0, false
	}
	return *raw, true
}

// photoItem is one item of a photos list as sent. A pointer is nil when the
// member is missing or null; a member of the wrong JSON type fails the decode.
type photoItem struct {
	ID        *string `json:"id"`
	PhotoKey  *string `json:"photo_key"`
	Source    *string `json:"photo_source"`
	SizeBytes *int    `json:"size_bytes"`
}

// isPresent reports whether a raw JSON member was sent and is not null.
func isPresent(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

// decodePhotoList reads a photos list that was sent. It returns the items in
// order, or the reason the list is refused: required (an empty or missing
// list), too_many (over ten) or invalid. Here only the shape of the list and
// of each item and the owner of each key are decided: the list is an array of
// objects; a create item has a photo_key and no id; an edit item has exactly
// one of id (a canonical UUID) and photo_key; every photo_key is exactly one
// the caller's own ticket could have made. Repeats, sources and sizes are the
// cleaner's checks in the spin package, and whether an id is one of the
// entry's photos is the service's.
func decodePhotoList(raw json.RawMessage, caller uuid.UUID, edit bool) ([]spin.PhotoRef, string) {
	if !isPresent(raw) {
		return nil, spin.ProblemRequired
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, spin.ProblemInvalid
	}
	// The count comes before any item, as in the cleaner.
	switch {
	case len(items) == 0:
		return nil, spin.ProblemRequired
	case len(items) > spin.MaxEntryPhotos:
		return nil, spin.ProblemTooMany
	}

	refs := make([]spin.PhotoRef, len(items))
	for i, item := range items {
		trimmed := bytes.TrimSpace(item)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return nil, spin.ProblemInvalid
		}
		var it photoItem
		if err := json.Unmarshal(trimmed, &it); err != nil {
			return nil, spin.ProblemInvalid
		}

		hasID, hasKey := it.ID != nil, it.PhotoKey != nil
		switch {
		case edit && hasID == hasKey:
			// Exactly one of id and photo_key.
			return nil, spin.ProblemInvalid
		case !edit && (hasID || !hasKey):
			// A create holds only new uploads.
			return nil, spin.ProblemInvalid
		}

		ref := spin.PhotoRef{SizeBytes: it.SizeBytes}
		if hasID {
			id, ok := canonicalID(*it.ID)
			if !ok {
				return nil, spin.ProblemInvalid
			}
			ref.ID = id
		} else {
			if !storage.EntryPhotoKeyOwnedBy(*it.PhotoKey, caller) {
				return nil, spin.ProblemInvalid
			}
			ref.Key = *it.PhotoKey
			if it.Source != nil {
				ref.Source = *it.Source
			}
		}
		refs[i] = ref
	}
	return refs, ""
}

// newPhotosOf turns the items of a create's photo list into new uploads.
func newPhotosOf(refs []spin.PhotoRef) []spin.NewPhoto {
	out := make([]spin.NewPhoto, len(refs))
	for i, r := range refs {
		out[i] = spin.NewPhoto{Key: r.Key, Source: r.Source, SizeBytes: r.SizeBytes}
	}
	return out
}

// --- the shape sent to the phone ---

// entryPhotoView is one photo of an entry as the phone sees it: its id, its
// link and its place in the list. The storage key is never part of it.
type entryPhotoView struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Position int    `json:"position"`
}

// entryView is an entry as the phone sees it. PhotoURL and PhotoSource are the
// cover's (the first photo's), kept beside the list so older readers do not
// break.
type entryView struct {
	ID          string           `json:"id"`
	LocalDate   spin.Date        `json:"local_date"`
	Category    spin.CategoryRef `json:"category"`
	Title       string           `json:"title"`
	Stars       int              `json:"stars"`
	Reaction    *string          `json:"reaction"`
	Comment     *string          `json:"comment"`
	Photos      []entryPhotoView `json:"photos"`
	PhotoURL    string           `json:"photo_url"`
	PhotoSource string           `json:"photo_source"`
	Late        bool             `json:"late"`
	Archived    bool             `json:"archived"`
	CommentsOff bool             `json:"comments_off"`
	CreatedAt   spin.Instant     `json:"created_at"`
	UpdatedAt   spin.Instant     `json:"updated_at"`
}

// newEntryView builds the phone's view of an entry. Links come from photoURL
// (the photo store's PublicURL); photos are listed in position order.
func newEntryView(e spin.Entry, photoURL func(string) string) entryView {
	photos := slices.Clone(e.Photos)
	slices.SortStableFunc(photos, func(a, b spin.EntryPhoto) int {
		return cmp.Compare(a.Position, b.Position)
	})
	views := make([]entryPhotoView, len(photos))
	for i, p := range photos {
		views[i] = entryPhotoView{ID: p.ID, URL: photoURL(p.Key), Position: p.Position}
	}
	v := entryView{
		ID:          e.ID,
		LocalDate:   e.LocalDate,
		Category:    spin.CategoryRef{ID: e.CategoryID, Name: e.CategoryName},
		Title:       e.Title,
		Stars:       e.Stars,
		Reaction:    e.Reaction,
		Comment:     e.Comment,
		Photos:      views,
		Late:        e.Late,
		Archived:    e.Archived,
		CommentsOff: e.CommentsOff,
		CreatedAt:   spin.Instant(e.CreatedAt),
		UpdatedAt:   spin.Instant(e.UpdatedAt),
	}
	if len(views) > 0 {
		v.PhotoURL = views[0].URL
		v.PhotoSource = photos[0].Source
	}
	return v
}

// linkToday fills the link of the cover photo in Today's entry summary. Below
// the HTTP layer the summary carries only the storage key of the cover; the
// link is built here, just before an answer is written, so every Today the
// server sends has a link and no key. It does nothing when the entry is null
// or photoURL is nil.
func linkToday(t *spin.Today, photoURL func(string) string) {
	if t == nil || t.Entry == nil || photoURL == nil {
		return
	}
	t.Entry.PhotoURL = photoURL(t.Entry.CoverKey)
}

// removeFiles asks the store to remove the files of photos an edit or a delete
// dropped, once the change is saved. It is best effort: a failure is logged
// and the rest still run, and the request never fails because of it. The whole
// batch shares one short deadline, and a caller that hangs up does not cancel
// it.
func (h *EntryHandler) removeFiles(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fileRemovalTimeout)
	defer cancel()
	for _, key := range keys {
		if err := h.photos.DeleteEntryPhoto(ctx, key); err != nil {
			log.Printf("httpapi: could not remove entry photo file %s: %v", key, err)
		}
	}
}

// --- the routes ---

// ticket answers POST /entry-photos/upload-url with {"upload_url",
// "photo_key", "expires_in"}: one ticket for one photo, for the caller's own
// folder.
func (h *EntryHandler) ticket(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req EntryPhotoTicketRequest
	if !bindEntryBody(c, &req) {
		return
	}
	if req.ContentType == nil || *req.ContentType == "" {
		RespondFieldError(c, "content_type", reasonInvalid)
		return
	}
	if req.ContentLength == nil || *req.ContentLength <= 0 {
		RespondFieldError(c, "content_length", reasonRequired)
		return
	}
	ticket, err := h.photos.PresignEntryPhotoUpload(c.Request.Context(), u.ID, *req.ContentType, *req.ContentLength)
	switch {
	case errors.Is(err, storage.ErrEntryPhotoType):
		RespondFieldError(c, "content_type", reasonInvalid)
		return
	case errors.Is(err, storage.ErrEntryPhotoTooLarge):
		RespondFieldError(c, "content_length", reasonTooLong)
		return
	case err != nil:
		RespondError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{
		"upload_url": ticket.UploadURL,
		"photo_key":  ticket.Key,
		"expires_in": ticket.ExpiresIn,
	})
}

// create answers POST /entries with 201 for a new entry and 200 for the same
// entry again (the same ordered photo keys), both with {"already", "entry",
// "today"}.
func (h *EntryHandler) create(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req CreateEntryRequest
	if !bindEntryBody(c, &req) {
		return
	}
	forDate, ok := forDateField(c, req.ForDate)
	if !ok {
		return
	}
	spinSeq, ok := spinSeqField(c, req.SpinSeq)
	if !ok {
		return
	}
	if req.Title == nil {
		RespondFieldError(c, "title", reasonRequired)
		return
	}
	if req.Stars == nil {
		RespondFieldError(c, "stars", reasonRequired)
		return
	}
	refs, reason := decodePhotoList(req.Photos, u.ID, false)
	if reason != "" {
		respondPhotosProblem(c, reason)
		return
	}

	out, err := h.svc.PostEntry(c.Request.Context(), u.ID.String(), forDate, spinSeq, spin.EntryInput{
		Title:    *req.Title,
		Stars:    *req.Stars,
		Reaction: req.Reaction,
		Comment:  req.Comment,
		Photos:   newPhotosOf(refs),
	})
	if err != nil {
		respondEntryError(c, err)
		return
	}
	linkToday(&out.Today, h.photos.PublicURL)
	status := http.StatusCreated
	if out.Already {
		status = http.StatusOK
	}
	Respond(c, status, gin.H{
		"already": out.Already,
		"entry":   newEntryView(out.Entry, h.photos.PublicURL),
		"today":   out.Today,
	})
}

// get answers GET /entries/:id with {"entry"}.
func (h *EntryHandler) get(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := entryIDOf(c)
	if !ok {
		return
	}
	e, err := h.svc.GetEntry(c.Request.Context(), u.ID.String(), id)
	if err != nil {
		respondEntryError(c, err)
		return
	}
	Respond(c, http.StatusOK, gin.H{"entry": newEntryView(e, h.photos.PublicURL)})
}

// patch answers PATCH /entries/:id with {"entry"}. An empty body changes
// nothing. After a saved edit that dropped photos, their files are removed.
func (h *EntryHandler) patch(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := entryIDOf(c)
	if !ok {
		return
	}
	var req UpdateEntryRequest
	if !bindEntryBody(c, &req) {
		return
	}
	patch := spin.EntryPatch{
		Title:       req.Title,
		Stars:       req.Stars,
		Reaction:    req.Reaction,
		Comment:     req.Comment,
		CommentsOff: req.CommentsOff,
	}
	if isPresent(req.Photos) {
		refs, reason := decodePhotoList(req.Photos, u.ID, true)
		if reason != "" {
			respondPhotosProblem(c, reason)
			return
		}
		patch.Photos = &refs
	}

	out, err := h.svc.EditEntry(c.Request.Context(), u.ID.String(), id, patch)
	if err != nil {
		respondEntryError(c, err)
		return
	}
	h.removeFiles(c.Request.Context(), out.Dropped)
	Respond(c, http.StatusOK, gin.H{"entry": newEntryView(out.Entry, h.photos.PublicURL)})
}

// deleteEntry answers POST /entries/:id/delete with {"today"}. After a saved
// delete the files of all its photos are removed.
func (h *EntryHandler) deleteEntry(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := entryIDOf(c)
	if !ok {
		return
	}
	var req DeleteEntryRequest
	if !bindEntryBody(c, &req) {
		return
	}

	out, err := h.svc.DeleteEntry(c.Request.Context(), u.ID.String(), id)
	if err != nil {
		respondEntryError(c, err)
		return
	}
	h.removeFiles(c.Request.Context(), out.Dropped)
	linkToday(&out.Today, h.photos.PublicURL)
	Respond(c, http.StatusOK, gin.H{"today": out.Today})
}

// archive answers POST /entries/:id/archive with {"entry", "today"}. It is
// idempotent. The day still counts: Today stays posted and carries the entry
// with archived true.
func (h *EntryHandler) archive(c *gin.Context) {
	h.setArchived(c, true)
}

// unarchive answers POST /entries/:id/unarchive with {"entry", "today"}.
func (h *EntryHandler) unarchive(c *gin.Context) {
	h.setArchived(c, false)
}

func (h *EntryHandler) setArchived(c *gin.Context, archived bool) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	id, ok := entryIDOf(c)
	if !ok {
		return
	}
	var req DeleteEntryRequest
	if !bindEntryBody(c, &req) {
		return
	}
	var out spin.ArchiveOutcome
	var err error
	if archived {
		out, err = h.svc.ArchiveEntry(c.Request.Context(), u.ID.String(), id)
	} else {
		out, err = h.svc.UnarchiveEntry(c.Request.Context(), u.ID.String(), id)
	}
	if err != nil {
		respondEntryError(c, err)
		return
	}
	linkToday(&out.Today, h.photos.PublicURL)
	Respond(c, http.StatusOK, gin.H{
		"entry": newEntryView(out.Entry, h.photos.PublicURL),
		"today": out.Today,
	})
}

// archiveList answers GET /me/archive with {"entries"}: the caller's archived
// entries, newest date first.
func (h *EntryHandler) archiveList(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	list, err := h.svc.ArchivedEntries(c.Request.Context(), u.ID.String())
	if err != nil {
		respondEntryError(c, err)
		return
	}
	views := make([]entryView, len(list))
	for i, e := range list {
		views[i] = newEntryView(e, h.photos.PublicURL)
	}
	Respond(c, http.StatusOK, gin.H{"entries": views})
}

// skip answers POST /spin/skip with 200 and {"already", "today"}, whether the
// skip is new or a repeat.
func (h *EntryHandler) skip(c *gin.Context) {
	u, ok := finishedUser(c)
	if !ok {
		return
	}
	var req SkipRequest
	if !bindEntryBody(c, &req) {
		return
	}
	forDate, ok := forDateField(c, req.ForDate)
	if !ok {
		return
	}
	spinSeq, ok := spinSeqField(c, req.SpinSeq)
	if !ok {
		return
	}

	out, err := h.svc.SkipDay(c.Request.Context(), u.ID.String(), forDate, spinSeq)
	if err != nil {
		respondEntryError(c, err)
		return
	}
	linkToday(&out.Today, h.photos.PublicURL)
	Respond(c, http.StatusOK, gin.H{"already": out.Already, "today": out.Today})
}
