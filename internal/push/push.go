// Package push sends phone notifications while the app is closed, for five
// events: a comment on your post, a reply to your comment, a like on your
// post, a follow request to you, and your follow request accepted. The same
// rules as the bell decide who gets what (blocks, gone accounts, who can
// still see the post, never yourself). In the notification settings the
// comments switch turns comment and reply pushes off, and the Likes switch
// (the stored "reactions" value) turns like pushes off. A like is pushed
// once per person and post: only a new like, and not again when it is
// undone and made again within a day (Notifier.Liked).
//
// Sending goes through Expo's push service when the server runs with
// PUSH_DRIVER=expo, and is a no-op otherwise (see docs/PUSH-SETUP.md). It is
// always best effort and in the background: a request never waits on it and
// never fails because of it.
package push

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
)

// Title is the title of every push.
const Title = "RNDMRoll"

// Kinds carried in a push's data, the same names as the bell's.
const (
	KindComment        = notifications.KindComment
	KindReply          = notifications.KindReply
	KindFollowRequest  = notifications.KindFollowRequest
	KindFollowAccepted = notifications.KindFollowAccepted
	KindLike           = notifications.KindLike
)

// Platforms a token may come from.
const (
	PlatformIOS     = "ios"
	PlatformAndroid = "android"
)

// MaxTokenLen is the longest token kept.
const MaxTokenLen = 200

// tokenPattern is the shape of an Expo push token.
var tokenPattern = regexp.MustCompile(`^(ExponentPushToken|ExpoPushToken)\[[A-Za-z0-9_\-]{1,180}\]$`)

// ValidToken reports whether s looks like an Expo push token.
func ValidToken(s string) bool {
	return len(s) <= MaxTokenLen && tokenPattern.MatchString(s)
}

// ValidPlatform reports whether p is "ios" or "android".
func ValidPlatform(p string) bool {
	return p == PlatformIOS || p == PlatformAndroid
}

// Message is one push to one person (to every phone they signed in on).
type Message struct {
	UserID uuid.UUID
	Title  string
	Body   string
	// Data lets a tap open the right page: kind, and entry_id, own
	// ("true"/"false": the post is the person's own) or actor_id.
	Data map[string]string
}

// Sender delivers messages. It never returns an error: failures are logged.
type Sender interface {
	Send(ctx context.Context, msgs []Message)
}

// NoopSender sends nothing (PUSH_DRIVER is not "expo").
type NoopSender struct{}

// Send implements Sender.
func (NoopSender) Send(context.Context, []Message) {}

// Target is one person a new comment notifies, by the bell's rules.
type Target struct {
	UserID  uuid.UUID
	Kind    string // KindComment or KindReply
	ActorID uuid.UUID
	EntryID uuid.UUID
	Body    string
	// Own: the post is the target's own.
	Own bool
}

// Repository is the storage behind pushes.
type Repository interface {
	// SaveToken stores token for userID, moving it from any other account,
	// and ties it to session: the refresh token the saving request's access
	// token was issued with (uuid.Nil for a token from before sessions were
	// named, which ties it to none). A token is sent to only while its
	// session lasts, so a phone that left without its DELETE reaching the
	// server stops getting pushes all the same. A token tied to a live
	// session is moved only by a save from a live session: a late save from
	// a session that has ended, or one that names none, leaves it alone, and
	// so does a late save from an older live session of the same person
	// while the token is tied to a newer one (its refresh token was made
	// later). A save from the newer session always takes it.
	SaveToken(ctx context.Context, userID, session uuid.UUID, token, platform string, at time.Time) error
	// DeleteToken deletes userID's token when it is tied to session (the
	// caller's own), to a session that has ended, or to none. A token tied to
	// another live session stays, so a late DELETE from an older session
	// never takes a newer sign in's pushes. Absent is fine.
	DeleteToken(ctx context.Context, userID, session uuid.UUID, token string) error
	// Tokens are the tokens of userID whose session is live: not logged out,
	// not expired and not ended by a password change (none for a disabled
	// account, none tied to no session).
	Tokens(ctx context.Context, userID uuid.UUID) ([]string, error)
	// RemoveTokens deletes tokens Expo said are no longer valid.
	RemoveTokens(ctx context.Context, tokens []string) error
	// CommentTargets are the people comment commentID notifies (the post's
	// owner for a comment, the top-level comment's writer for a reply, and
	// the owner for a reply too), with the bell's block and visibility rules
	// and never the writer.
	CommentTargets(ctx context.Context, commentID uuid.UUID) ([]Target, error)
	// RequestPair reads a follow row's follower and followee by its id.
	RequestPair(ctx context.Context, requestID uuid.UUID) (follower, followee uuid.UUID, ok bool, err error)
	// LikeTarget is the owner of entryID when likerID's like on it is there
	// and the owner's bell shows it (NotificationsRepo.Likes): read when the
	// push runs, so a like undone before then is not ok. Not ok, too, when
	// either of them blocked the other, the liker no longer follows the
	// owner, either account is disabled, being deleted or an admin's, the
	// like is the owner's own, or an admin removed the post.
	LikeTarget(ctx context.Context, entryID, likerID uuid.UUID) (owner uuid.UUID, ok bool, err error)
}

// People reads names and notification settings (social.Service).
type People interface {
	People(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]social.Person, error)
	Settings(ctx context.Context, viewer uuid.UUID) (social.Settings, error)
}

// sendTimeout bounds one event's work in the background.
const sendTimeout = 20 * time.Second

// likeMemoFor is how long a like's push is remembered: the same person's like
// on the same post, undone and made again within it, is not pushed again.
// The like route allows 60 requests a minute, so without it a like and an
// undo in a row could buzz the owner's phone 30 times a minute.
const likeMemoFor = 24 * time.Hour

// likeMemoMax bounds the memo whatever happens: far more like pushes than
// the app sends in a day.
const likeMemoMax = 100_000

// likePrunePause is the least time between two sweeps of the memo for
// entries past likeMemoFor.
const likePrunePause = time.Minute

// likeKey is one person's like on one post.
type likeKey struct {
	entry, liker uuid.UUID
}

// Notifier turns events into pushes, in the background. A nil *Notifier is
// never handed out; the services hold an interface that may be nil.
type Notifier struct {
	repo   Repository
	people People
	sender Sender
	log    *slog.Logger
	wg     sync.WaitGroup

	// now is the wall clock (time.Now; a test moves it).
	now func() time.Time
	// mu guards liked and pruned.
	mu sync.Mutex
	// liked is when each like was pushed, kept likeMemoFor (claimLike). It
	// lives in memory only: a restart forgets it, so a like undone and made
	// again after a restart can be pushed once more.
	liked map[likeKey]time.Time
	// pruned is when liked was last swept for old entries.
	pruned time.Time
}

// NewNotifier builds a Notifier.
func NewNotifier(repo Repository, people People, sender Sender, log *slog.Logger) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	return &Notifier{repo: repo, people: people, sender: sender, log: log,
		now: time.Now, liked: map[likeKey]time.Time{}}
}

// Wait waits for every event started so far (tests and shutdown).
func (n *Notifier) Wait() { n.wg.Wait() }

// off: with the no-op sender there is nothing to look up.
func (n *Notifier) off() bool {
	_, noop := n.sender.(NoopSender)
	return noop || n.sender == nil
}

// run does fn in the background with its own deadline, never the request's.
func (n *Notifier) run(what string, fn func(ctx context.Context) error) {
	if n.off() {
		return
	}
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				n.log.Error("push: panic", "event", what, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			n.log.Warn("push: event not sent", "event", what, "error", err)
		}
	}()
}

// FirstName is the first word of the name, else the username (the app's
// firstName).
func FirstName(p social.Person) string {
	if f := strings.Fields(p.Name); len(f) > 0 {
		return f[0]
	}
	return p.Username
}

// CommentBody is the push text for a comment or reply, as the bell says it.
func CommentBody(kind, name, body string) string {
	text := notifications.CutText(strings.TrimSpace(body), notifications.TextRunes)
	if text == "" {
		return name + " sent a photo."
	}
	if kind == KindReply {
		return name + ` replied to your comment: "` + text + `"`
	}
	return name + ` commented: "` + text + `"`
}

// CommentAdded pushes a new comment or reply to the people it notifies.
func (n *Notifier) CommentAdded(commentID uuid.UUID) {
	n.run("comment", func(ctx context.Context) error {
		targets, err := n.repo.CommentTargets(ctx, commentID)
		if err != nil || len(targets) == 0 {
			return err
		}
		ids := make([]uuid.UUID, 0, len(targets))
		for _, t := range targets {
			ids = append(ids, t.ActorID)
		}
		people, err := n.people.People(ctx, ids)
		if err != nil {
			return err
		}
		var msgs []Message
		for _, t := range targets {
			actor, ok := people[t.ActorID]
			if !ok || t.UserID == t.ActorID {
				continue
			}
			settings, err := n.people.Settings(ctx, t.UserID)
			if err != nil {
				return err
			}
			if !settings.Comments {
				continue
			}
			own := "false"
			if t.Own {
				own = "true"
			}
			msgs = append(msgs, Message{
				UserID: t.UserID, Title: Title, Body: CommentBody(t.Kind, FirstName(actor), t.Body),
				Data: map[string]string{"kind": t.Kind, "entry_id": t.EntryID.String(), "own": own, "actor_id": t.ActorID.String()},
			})
		}
		if len(msgs) > 0 {
			n.sender.Send(ctx, msgs)
		}
		return nil
	})
}

// LikeBody is the push text for a like, the bell's like line: "Maya liked
// your post.".
func LikeBody(name string) string {
	return name + " liked your post."
}

// Liked pushes likerID's new like on entryID to the post's owner, by the
// bell's rules for likes (Repository.LikeTarget, read when it runs, so a like
// undone meanwhile sends nothing), and only while the owner's Likes switch
// (social.Settings.Reactions) is on. A tap opens the owner's own entry page,
// as the bell's like line does. Each person's like on a post is pushed at
// most once in likeMemoFor (claimLike): one undone and made again within it
// is not pushed again.
func (n *Notifier) Liked(entryID, likerID uuid.UUID) {
	n.run("like", func(ctx context.Context) error {
		owner, ok, err := n.repo.LikeTarget(ctx, entryID, likerID)
		if err != nil || !ok || owner == likerID {
			return err
		}
		settings, err := n.people.Settings(ctx, owner)
		if err != nil {
			return err
		}
		if !settings.Reactions {
			return nil
		}
		people, err := n.people.People(ctx, []uuid.UUID{likerID})
		if err != nil {
			return err
		}
		liker, ok := people[likerID]
		if !ok || !n.claimLike(entryID, likerID) {
			return nil
		}
		n.sender.Send(ctx, []Message{{
			UserID: owner, Title: Title, Body: LikeBody(FirstName(liker)),
			Data: map[string]string{"kind": KindLike, "entry_id": entryID.String(), "own": "true", "actor_id": likerID.String()},
		}})
		return nil
	})
}

// claimLike reports whether liker's like on entry may be pushed now and, when
// it may, remembers that it was: false while the pair's last push is less
// than likeMemoFor old. One lock covers the check and the write, so two
// pushes of the same like at once (a like, an undo and a like in a row) send
// once. Liked calls it only right before it sends: a like that sent nothing
// (undone before the push ran, or the switch off) leaves no mark, and the
// next like is pushed.
//
// Entries past likeMemoFor are swept out at most once a likePrunePause, so
// the memo holds about a day of pushes and never more than likeMemoMax:
// while it is full, a new like is not pushed (its line still shows in the
// bell) until older entries age out.
func (n *Notifier) claimLike(entry, liker uuid.UUID) bool {
	now := n.now()
	key := likeKey{entry: entry, liker: liker}
	n.mu.Lock()
	defer n.mu.Unlock()
	if at, ok := n.liked[key]; ok && now.Sub(at) < likeMemoFor {
		return false
	}
	if now.Sub(n.pruned) >= likePrunePause {
		for k, at := range n.liked {
			if now.Sub(at) >= likeMemoFor {
				delete(n.liked, k)
			}
		}
		n.pruned = now
	}
	if len(n.liked) >= likeMemoMax {
		return false
	}
	n.liked[key] = now
	return true
}

// FollowRequested pushes a new follow request from follower to followee.
func (n *Notifier) FollowRequested(follower, followee uuid.UUID) {
	n.run("follow_request", func(ctx context.Context) error {
		return n.personEvent(ctx, followee, follower, KindFollowRequest, " wants to follow you.")
	})
}

// FollowAccepted pushes to the person whose request requestID was accepted.
func (n *Notifier) FollowAccepted(requestID uuid.UUID) {
	n.run("follow_accepted", func(ctx context.Context) error {
		follower, followee, ok, err := n.repo.RequestPair(ctx, requestID)
		if err != nil || !ok {
			return err
		}
		return n.personEvent(ctx, follower, followee, KindFollowAccepted, " accepted your follow request.")
	})
}

// personEvent sends "{actor's first name}{rest}" to to.
func (n *Notifier) personEvent(ctx context.Context, to, actorID uuid.UUID, kind, rest string) error {
	if to == actorID {
		return nil
	}
	people, err := n.people.People(ctx, []uuid.UUID{actorID})
	if err != nil {
		return err
	}
	actor, ok := people[actorID]
	if !ok {
		return nil
	}
	n.sender.Send(ctx, []Message{{
		UserID: to, Title: Title, Body: FirstName(actor) + rest,
		Data: map[string]string{"kind": kind, "actor_id": actorID.String()},
	}})
	return nil
}
