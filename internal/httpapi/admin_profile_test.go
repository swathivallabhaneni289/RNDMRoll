package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/admin"
)

// The person page reads as a profile (developer, 2026-11-18): a back arrow to
// the tab it was opened from, their profile, and their posts while a report
// about the account is open or it is disabled (admin.ShowsPosts).

// postsOf is each post on a person page, in the page's order, up to its end.
func postsOf(t *testing.T, page string) []string {
	t.Helper()
	start := strings.Index(page, `<div class="posts">`)
	if start < 0 {
		return nil
	}
	posts := strings.Split(page[start:], `<article class="post">`)[1:]
	for i, p := range posts {
		posts[i] = p[:strings.Index(p, "</article>")]
	}
	return posts
}

// While a report about the account is open, the page shows their posts as
// the store gives them (the newest first), after the open reports: each with
// every photo in order, the first large and each opening full size, its
// category and day, its title and text, escaped, and one tag when it is
// removed, hidden or reported. The posts are read only then.
func TestAdminPerson_ShowsTheirPostsWhileTheAccountIsReported(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person := uuid.New()
	r.repo.reports = []admin.Report{{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "tess_r", Reason: "impersonation", CreatedAt: cardAt(9, 0)}}
	r.repo.posts = []admin.Post{
		{ID: uuid.New(), Title: "Ramen", Category: "Meal", LocalDate: "2026-11-15", Note: "Slurp", PhotoKeys: []string{"k1", "k2", "k3"}, Reported: true},
		{ID: uuid.New(), Title: "Dune", Category: "Movie", LocalDate: "2026-11-14", PhotoKeys: []string{"k4"}, Hidden: true, Reported: true},
		{ID: uuid.New(), Title: "<b>Loud</b>", LocalDate: "2026-11-13", Removed: true},
		{ID: uuid.New(), Title: "Walk", Category: "Place", LocalDate: "2026-11-12"},
	}
	page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
	posts := postsOf(t, page)
	if r.repo.postReads != 1 || len(posts) != 4 {
		t.Fatalf("want the 4 posts, read once (%d reads): %s", r.repo.postReads, page)
	}
	if strings.Index(page, "<h2>Open reports</h2>") > strings.Index(page, "<h2>Posts</h2>") ||
		strings.Index(page, `<article class="card`) > strings.Index(page, "<h2>Posts</h2>") {
		t.Fatalf("want the posts after the open reports: %s", page)
	}
	link := func(key string) string {
		return `<a href="` + fakePhotoLink(key) + `" target="_blank" rel="noopener"><img src="` + fakePhotoLink(key) +
			`" alt="Post photo" loading="lazy" decoding="async"></a>`
	}
	for i, want := range []string{
		`<div class="cover">` + link("k1") + link("k2") + link("k3") + `</div>` + "\n" +
			`<p class="day">Meal · Nov 15, 2026</p><h3>Ramen</h3><p class="text">Slurp</p>` + "\n" +
			`<p class="flag"><span class="tag">Reported</span></p>`,
		`<div class="cover">` + link("k4") + `</div>` + "\n" +
			`<p class="day">Movie · Nov 14, 2026</p><h3>Dune</h3>` + "\n" + `<p class="flag"><span class="tag">Hidden</span></p>`,
		`<p class="day">Nov 13, 2026</p><h3>&lt;b&gt;Loud&lt;/b&gt;</h3>` + "\n" + `<p class="flag"><span class="tag">Removed</span></p>`,
		`<p class="day">Place · Nov 12, 2026</p><h3>Walk</h3>`,
	} {
		if !strings.Contains(posts[i], want) || strings.Count(posts[i], `class="tag"`) > 1 {
			t.Errorf("post %d lacks %q: %s", i, want, posts[i])
		}
	}
	if strings.Contains(posts[3], `class="tag"`) || strings.Contains(posts[2], `class="cover"`) ||
		strings.Contains(page, "Showing the newest") || strings.Contains(page, "Their posts show here") {
		t.Fatalf("a post with no tag or no photo, or a line that does not belong: %s", page)
	}
	socialNoScript(t, "the person page with posts", page)
}

// Past admin.PostsLimit posts the page shows the newest that many and says
// so; with none, it says that.
func TestAdminPerson_ShowsTheNewestPostsAndSaysSo(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person := uuid.New()
	r.repo.reports = []admin.Report{{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(9, 0)}}
	for i := 0; i <= admin.PostsLimit; i++ {
		r.repo.posts = append(r.repo.posts, admin.Post{ID: uuid.New(), Title: "Post", LocalDate: "2026-11-15"})
	}
	page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
	if n := len(postsOf(t, page)); n != admin.PostsLimit || !strings.Contains(page, `<p class="lede">Showing the newest 60.</p>`) {
		t.Fatalf("%d posts, want %d and the line: %s", n, admin.PostsLimit, page)
	}
	r.repo.posts = nil
	page = adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
	if !strings.Contains(page, "<h2>Posts</h2>\n"+`<p class="empty">No posts.</p>`) || strings.Contains(page, "Showing the newest") {
		t.Fatalf("no posts: %s", page)
	}
}

// A report about one post or comment shows that one on its card and none of
// their other posts, and neither does an account report that is closed: the
// page says when they show, and reads none. A disabled account shows them,
// as Disable closes the account reports and Enable is decided on them.
func TestAdminPerson_ShowsNoPostsWithoutAnOpenAccountReportUnlessDisabled(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person := uuid.New()
	closedAt := cardAt(12, 0)
	r.repo.posts = []admin.Post{{ID: uuid.New(), Title: "Ramen", LocalDate: "2026-11-15"}}
	for name, reports := range map[string][]admin.Report{
		"an open post report": {{Kind: admin.KindPost, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(9, 0),
			EntryID: uuidPtr(uuid.New()), Title: "Ramen", TitleNow: "Ramen", LocalDate: "2026-11-15"}},
		"a closed account report": {{Kind: admin.KindPerson, ID: uuid.New(), Reporter: "tess_r", Reason: "spam", CreatedAt: cardAt(9, 0),
			ResolvedAt: &closedAt, Resolution: admin.Dismissed}},
	} {
		r.repo.reports = reports
		page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
		if r.repo.postReads != 0 || postsOf(t, page) != nil ||
			!strings.Contains(page, "<h2>Posts</h2>\n"+`<p class="lede">Their posts show here while a report about the account is open, or while it's disabled.</p>`) {
			t.Fatalf("%s: want no posts read (%d reads) and the line: %s", name, r.repo.postReads, page)
		}
	}
	r.repo.disabledAt = &closedAt
	page := adminDo(t, r.w, http.MethodGet, "/admin/people/"+person.String(), cookie, nil, "").Body.String()
	if r.repo.postReads != 1 || len(postsOf(t, page)) != 1 {
		t.Fatalf("a disabled account, want its posts: %s", page)
	}
}

// The back arrow on top of the person page goes to the tab whose Open
// profile link opened it, and to Reports from anywhere else: only the tabs'
// own paths, never what the link says. The Disabled and Removed tabs' links
// say which tab they are on.
func TestAdminPerson_TheBackArrowGoesToTheTabItCameFrom(t *testing.T) {
	r := newAdminRig(t, true)
	cookie := r.session(time.Now())
	person := uuid.New()
	reports := `<p class="back"><a class="button" href="/admin">← Reports</a></p>`
	for from, want := range map[string]string{
		"":                      reports,
		"list":                  reports,
		"removed":               `<p class="back"><a class="button" href="/admin/removed">← Removed</a></p>`,
		"disabled":              `<p class="back"><a class="button" href="/admin/disabled">← Disabled</a></p>`,
		"https://evil.example/": reports,
		"/admin/removed":        reports,
	} {
		path := "/admin/people/" + person.String()
		if from != "" {
			path += "?from=" + url.QueryEscape(from)
		}
		page := adminDo(t, r.w, http.MethodGet, path, cookie, nil, "").Body.String()
		if !strings.Contains(page, want) || strings.Index(page, want) > strings.Index(page, `<section class="card profile">`) ||
			strings.Contains(page, "evil") || strings.Count(page, `<p class="back">`) != 1 {
			t.Fatalf("from %q: want %s on top: %s", from, want, page)
		}
	}
	r.repo.disabled = []admin.Person{{ID: person, Username: "loud_larry"}}
	if tab := adminDo(t, r.w, http.MethodGet, "/admin/disabled", cookie, nil, "").Body.String(); !strings.Contains(tab,
		`<a class="button" href="/admin/people/`+person.String()+`?from=disabled">Open profile</a>`) {
		t.Fatalf("the Disabled tab's link: %s", tab)
	}
}
