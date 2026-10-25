package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage/storagetest"
)

// The in-memory bucket stands in for the real client. The check is here
// because storagetest does not import this package.
var _ ObjectAPI = (*storagetest.Bucket)(nil)

const testBucket = "rndmroll-photos-test"

var (
	accountA = uuid.MustParse("3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e")
	// accountB differs from accountA only in its last digit, so a prefix
	// check that is off by one character shows.
	accountB = uuid.MustParse("3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5f")
)

// fileKey is a server shaped key in folder/{id}/: n as 16 lowercase hex
// digits, then .jpg.
func fileKey(folder string, id uuid.UUID, n int) string {
	return fmt.Sprintf("%s/%s/%016x.jpg", folder, id, n)
}

// fileKeys is fileKey for 1 to count, in lexical order.
func fileKeys(folder string, id uuid.UUID, count int) []string {
	keys := make([]string, 0, count)
	for n := 1; n <= count; n++ {
		keys = append(keys, fileKey(folder, id, n))
	}
	return keys
}

func sorted(keys ...string) []string {
	out := slices.Clone(keys)
	slices.Sort(out)
	return out
}

func TestAccountFolder_BuildsBothFolders(t *testing.T) {
	for folder, want := range map[string]string{
		"avatars": "avatars/3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e/",
		"entries": "entries/3f0c2a52-7d4e-4c4b-9c1d-6a1f2b3c4d5e/",
	} {
		got, err := accountFolder(folder, accountA)
		if err != nil || got != want {
			t.Errorf("accountFolder(%q) = %q, %v, want %q", folder, got, err, want)
		}
	}
}

func TestAccountFolder_RefusesTheZeroIDAndOtherFolders(t *testing.T) {
	for _, folder := range []string{"avatars", "entries"} {
		if got, err := accountFolder(folder, uuid.Nil); !errors.Is(err, ErrNotAccountFolder) || got != "" {
			t.Errorf("accountFolder(%q, uuid.Nil) = %q, %v, want ErrNotAccountFolder", folder, got, err)
		}
	}
	for _, folder := range []string{
		"", "board", "avatar", "entriesX", "Avatars", "ENTRIES", " entries", "entries ",
		"entries/", "/entries", "entries/..", "../entries", ".", "..",
		"avatars/" + accountB.String(),
	} {
		if got, err := accountFolder(folder, accountA); !errors.Is(err, ErrNotAccountFolder) || got != "" {
			t.Errorf("accountFolder(%q) = %q, %v, want ErrNotAccountFolder", folder, got, err)
		}
	}
}

// The pattern is the last guard, behind code that cannot produce these
// strings from a uuid.UUID, so it is tested on its own.
func TestAccountFolderPattern(t *testing.T) {
	id := accountA.String()
	for _, ok := range []string{"avatars/" + id + "/", "entries/" + id + "/"} {
		if !accountFolderPattern.MatchString(ok) {
			t.Errorf("the pattern refuses %q", ok)
		}
	}
	for _, bad := range []string{
		"",
		"avatars/",
		"entries/",
		"avatars//",
		"entries//",
		"entries/../",
		"entries/./",
		"entries/" + id,
		"entries/" + id + "//",
		"entries/" + id + "/x/",
		"entries/" + id + "/x",
		"entries/" + id + "/../",
		"entries/" + id + "/" + id + "/",
		"entries/" + strings.ToUpper(id) + "/",
		"entries/{" + id + "}/",
		"entries/" + strings.ReplaceAll(id, "-", "") + "/",
		"entries/" + id[:35] + "/",
		"entries/" + id + "0/",
		"entries/00000000-0000-0000-0000-00000000000g/",
		"/entries/" + id + "/",
		" entries/" + id + "/",
		"entries/" + id + "/ ",
		"entries/" + id + "/\n",
		"entriesX/" + id + "/",
		"board/" + id + "/",
		"Entries/" + id + "/",
		"avatars/entries/" + id + "/",
		"avatars/" + id + "/entries/" + id + "/",
	} {
		if accountFolderPattern.MatchString(bad) {
			t.Errorf("the pattern accepts %q", bad)
		}
	}
}

// Two accounts share the bucket, and only the one being purged loses files.
func TestDeleteAccountFolders_DeletesOnlyThatAccountsFiles(t *testing.T) {
	a := accountA.String()
	aAvatars := fileKeys("avatars", accountA, 3)
	aEntries := append(fileKeys("entries", accountA, 4),
		// A key the server would not make, but inside the account's folder,
		// goes too: the whole folder is the account's.
		"entries/"+a+"/old/0000000000000005.jpg")
	bKeys := append(fileKeys("avatars", accountB, 2), fileKeys("entries", accountB, 4)...)
	lookAlikes := []string{
		"entries/" + a,
		"avatars/" + a,
		"entries/" + a + "0/0000000000000001.jpg",
		"entriesX/" + a + "/a.jpg",
		"avatarsX/" + a + "/a.jpg",
		"board/" + a + "/x",
		"Entries/" + a + "/0000000000000001.jpg",
		"entries/" + strings.ToUpper(a) + "/0000000000000001.jpg",
		"x/entries/" + a + "/0000000000000001.jpg",
		"entries/",
		"avatars/",
		"entries",
	}
	var all []string
	for _, group := range [][]string{aAvatars, aEntries, bKeys, lookAlikes} {
		all = append(all, group...)
	}
	bucket := storagetest.NewBucket(testBucket, all...)
	files := NewAccountFilesOver(bucket, testBucket)

	n, err := files.DeleteAccountFolders(context.Background(), accountA)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(aAvatars) + len(aEntries); n != want {
		t.Errorf("deleted %d, want %d", n, want)
	}
	// One call per key, the avatars first, each folder in listing order.
	if got, want := bucket.Deleted(), append(sorted(aAvatars...), sorted(aEntries...)...); !slices.Equal(got, want) {
		t.Errorf("delete calls = %q\nwant %q", got, want)
	}
	if got, want := bucket.Keys(), sorted(append(slices.Clone(bKeys), lookAlikes...)...); !slices.Equal(got, want) {
		t.Errorf("left in the bucket = %q\nwant %q", got, want)
	}
	// Three avatars and five entries at two a page: two listings, then
	// three, and only ever the two folders of the account.
	avatarsA, entriesA := "avatars/"+a+"/", "entries/"+a+"/"
	if got, want := bucket.Listed(), []string{avatarsA, avatarsA, entriesA, entriesA, entriesA}; !slices.Equal(got, want) {
		t.Errorf("listings = %q, want %q", got, want)
	}

	// A second run, as after a failure, finds nothing left to delete.
	n, err = files.DeleteAccountFolders(context.Background(), accountA)
	if err != nil || n != 0 {
		t.Errorf("second run: deleted %d, error %v, want 0 and none", n, err)
	}
	if got := len(bucket.Deleted()); got != len(aAvatars)+len(aEntries) {
		t.Errorf("the second run made %d more delete calls", got-len(aAvatars)-len(aEntries))
	}
}

func TestDeleteAccountFolders_TheZeroIDSendsNothing(t *testing.T) {
	keys := []string{fileKey("avatars", uuid.Nil, 1), fileKey("entries", uuid.Nil, 1), fileKey("entries", accountA, 1)}
	bucket := storagetest.NewBucket(testBucket, keys...)

	n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrNotAccountFolder) || n != 0 {
		t.Fatalf("deleted %d, error %v, want 0 and ErrNotAccountFolder", n, err)
	}
	if got := bucket.Listed(); len(got) != 0 {
		t.Errorf("listed %q, want no call", got)
	}
	if got := bucket.Deleted(); len(got) != 0 {
		t.Errorf("deleted %q, want no call", got)
	}
	if got := bucket.Keys(); !slices.Equal(got, sorted(keys...)) {
		t.Errorf("left in the bucket = %q", got)
	}
}

// strayBucket misbehaves: from listing number from on (counting from 1),
// every page also names stray, in the middle of the page.
type strayBucket struct {
	*storagetest.Bucket
	stray types.Object
	from  int
	calls int
}

func (s *strayBucket) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	out, err := s.Bucket.ListObjectsV2(ctx, params, optFns...)
	s.calls++
	if err != nil || s.calls < s.from {
		return out, err
	}
	out.Contents = slices.Insert(out.Contents, min(1, len(out.Contents)), s.stray)
	out.KeyCount = aws.Int32(int32(len(out.Contents)))
	return out, nil
}

func TestDeleteAccountFolders_AListedKeyOutsideTheFolderDeletesNothing(t *testing.T) {
	a, b := accountA.String(), accountB.String()
	strays := map[string]types.Object{
		"another account's file":        {Key: aws.String(fileKey("avatars", accountB, 1))},
		"the account's other folder":    {Key: aws.String(fileKey("entries", accountA, 1))},
		"the folder without its slash":  {Key: aws.String("avatars/" + a)},
		"the whole avatars folder":      {Key: aws.String("avatars/")},
		"a dot dot into another folder": {Key: aws.String("avatars/" + a + "/../" + b + "/0000000000000001.jpg")},
		"a dot segment":                 {Key: aws.String("avatars/" + a + "/./0000000000000001.jpg")},
		"a trailing dot dot":            {Key: aws.String("avatars/" + a + "/..")},
		"an empty key":                  {Key: aws.String("")},
		"no key at all":                 {},
	}
	for name, stray := range strays {
		t.Run(name, func(t *testing.T) {
			keys := []string{
				fileKey("avatars", accountA, 1), fileKey("avatars", accountA, 2), fileKey("entries", accountA, 1),
				fileKey("avatars", accountB, 1), fileKey("entries", accountB, 1),
			}
			inner := storagetest.NewBucket(testBucket, keys...)
			bucket := &strayBucket{Bucket: inner, stray: stray, from: 1}

			n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), accountA)
			if !errors.Is(err, errBadListing) || n != 0 {
				t.Fatalf("deleted %d, error %v, want 0 and a refused listing", n, err)
			}
			if got := inner.Deleted(); len(got) != 0 {
				t.Errorf("delete calls = %q, want none", got)
			}
			if got := inner.Keys(); !slices.Equal(got, sorted(keys...)) {
				t.Errorf("left in the bucket = %q, want every key", got)
			}
			if got := inner.Listed(); !slices.Equal(got, []string{"avatars/" + a + "/"}) {
				t.Errorf("listings = %q, want only the first", got)
			}
		})
	}
}

func TestDeleteAccountFolders_AStrayOnALaterPageStopsThereAndKeepsThatPage(t *testing.T) {
	a := accountA.String()
	aAvatars := fileKeys("avatars", accountA, 3)
	others := []string{fileKey("entries", accountA, 1), fileKey("avatars", accountB, 1), fileKey("entries", accountB, 1)}
	inner := storagetest.NewBucket(testBucket, append(slices.Clone(aAvatars), others...)...)
	bucket := &strayBucket{Bucket: inner, stray: types.Object{Key: aws.String(fileKey("avatars", accountB, 1))}, from: 2}

	n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), accountA)
	if !errors.Is(err, errBadListing) || n != 2 {
		t.Fatalf("deleted %d, error %v, want 2 and a refused listing", n, err)
	}
	if got := inner.Deleted(); !slices.Equal(got, aAvatars[:2]) {
		t.Errorf("delete calls = %q, want the first page only", got)
	}
	// The refused page's own key, the entries folder (never reached) and
	// the other account's files are all still there.
	if got, want := inner.Keys(), sorted(append([]string{aAvatars[2]}, others...)...); !slices.Equal(got, want) {
		t.Errorf("left in the bucket = %q, want %q", got, want)
	}
	if got, want := inner.Listed(), []string{"avatars/" + a + "/", "avatars/" + a + "/"}; !slices.Equal(got, want) {
		t.Errorf("listings = %q, want %q", got, want)
	}
}

// pagingBucket misbehaves in its paging: next picks the token each
// truncated page carries, from the token the page was asked for with and
// the one the real bucket made.
type pagingBucket struct {
	*storagetest.Bucket
	next func(sent, made *string) *string
}

func (p *pagingBucket) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	out, err := p.Bucket.ListObjectsV2(ctx, params, optFns...)
	if err != nil {
		return out, err
	}
	if aws.ToBool(out.IsTruncated) {
		out.NextContinuationToken = p.next(params.ContinuationToken, out.NextContinuationToken)
	}
	return out, nil
}

func TestDeleteAccountFolders_BrokenPagingIsAnErrorNotALoop(t *testing.T) {
	cases := []struct {
		name    string
		next    func(sent, made *string) *string
		deleted int
		lists   int
	}{
		{"no token for the next page", func(_, _ *string) *string { return nil }, 0, 1},
		{"an empty token", func(_, _ *string) *string { return aws.String("") }, 0, 1},
		// On a later page, so the check for a token that came back
		// unchanged cannot stand in for this one.
		{"no token on a later page", func(sent, made *string) *string {
			if sent != nil {
				return nil
			}
			return made
		}, 2, 2},
		{"the token it was sent", func(sent, made *string) *string {
			if sent != nil {
				return sent
			}
			return made
		}, 2, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			keys := fileKeys("avatars", accountA, 5)
			inner := storagetest.NewBucket(testBucket, keys...)
			bucket := &pagingBucket{Bucket: inner, next: c.next}

			n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), accountA)
			if !errors.Is(err, errBadListing) || n != c.deleted {
				t.Fatalf("deleted %d, error %v, want %d and a refused listing", n, err, c.deleted)
			}
			if got := len(inner.Listed()); got != c.lists {
				t.Errorf("%d listings, want %d", got, c.lists)
			}
			if got := inner.Deleted(); !slices.Equal(got, keys[:c.deleted]) {
				t.Errorf("delete calls = %q, want %q", got, keys[:c.deleted])
			}
			if got := inner.Keys(); !slices.Equal(got, keys[c.deleted:]) {
				t.Errorf("left in the bucket = %q", got)
			}
		})
	}
}

func TestDeleteAccountFolders_ADeleteFailureStopsAndCountsWhatWentBeforeIt(t *testing.T) {
	avatars := fileKeys("avatars", accountA, 3)
	entries := fileKeys("entries", accountA, 5)
	bucket := storagetest.NewBucket(testBucket, append(slices.Clone(avatars), entries...)...)
	boom := errors.New("storage is down")
	bucket.FailDelete = map[string]error{entries[1]: boom}

	n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), accountA)
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the storage error", err)
	}
	if n != 4 {
		t.Errorf("deleted %d, want 4 (three avatars and the first entry)", n)
	}
	// Nothing after the failed key was tried.
	if got, want := bucket.Deleted(), append(slices.Clone(avatars), entries[0], entries[1]); !slices.Equal(got, want) {
		t.Errorf("delete calls = %q\nwant %q", got, want)
	}
	if !bucket.Has(entries[1]) {
		t.Error("the key whose delete failed is gone")
	}
	if got := bucket.Keys(); !slices.Equal(got, entries[1:]) {
		t.Errorf("left in the bucket = %q, want %q", got, entries[1:])
	}
}

func TestDeleteAccountFolders_AListFailureIsReturned(t *testing.T) {
	keys := fileKeys("avatars", accountA, 2)
	bucket := storagetest.NewBucket(testBucket, keys...)
	boom := errors.New("listing is down")
	bucket.FailList = boom

	n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(context.Background(), accountA)
	if !errors.Is(err, boom) || n != 0 {
		t.Fatalf("deleted %d, error %v, want 0 and the listing error", n, err)
	}
	if got := bucket.Deleted(); len(got) != 0 {
		t.Errorf("delete calls = %q, want none", got)
	}
	if got := bucket.Listed(); len(got) != 1 {
		t.Errorf("listings = %q, want one", got)
	}
}

func TestDeleteAccountFolders_ADoneContextSendsNothing(t *testing.T) {
	bucket := storagetest.NewBucket(testBucket, fileKeys("avatars", accountA, 2)...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n, err := NewAccountFilesOver(bucket, testBucket).DeleteAccountFolders(ctx, accountA)
	if !errors.Is(err, context.Canceled) || n != 0 {
		t.Fatalf("deleted %d, error %v, want 0 and context.Canceled", n, err)
	}
	if len(bucket.Listed()) != 0 || len(bucket.Deleted()) != 0 {
		t.Errorf("listed %q and deleted %q, want no call", bucket.Listed(), bucket.Deleted())
	}
}

// cancellingBucket ends the context after its first delete that succeeds.
type cancellingBucket struct {
	*storagetest.Bucket
	cancel context.CancelFunc
}

func (c *cancellingBucket) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	out, err := c.Bucket.DeleteObject(ctx, params, optFns...)
	if err == nil {
		c.cancel()
	}
	return out, err
}

func TestDeleteAccountFolders_StopsOnceTheContextIsDone(t *testing.T) {
	keys := fileKeys("avatars", accountA, 3)
	inner := storagetest.NewBucket(testBucket, keys...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := NewAccountFilesOver(&cancellingBucket{Bucket: inner, cancel: cancel}, testBucket).DeleteAccountFolders(ctx, accountA)
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatalf("deleted %d, error %v, want 1 and context.Canceled", n, err)
	}
	// No call at all once the context was done.
	if got := inner.Deleted(); !slices.Equal(got, keys[:1]) {
		t.Errorf("delete calls = %q, want %q", got, keys[:1])
	}
	if got := inner.Keys(); !slices.Equal(got, keys[1:]) {
		t.Errorf("left in the bucket = %q", got)
	}
}

func TestDeletePhoto_DeletesOneEntryPhoto(t *testing.T) {
	key := fileKey("entries", accountB, 0xabc)
	others := []string{fileKey("entries", accountB, 1), fileKey("avatars", accountB, 0xabc), fileKey("entries", accountA, 0xabc)}
	bucket := storagetest.NewBucket(testBucket, append([]string{key}, others...)...)

	if err := NewAccountFilesOver(bucket, testBucket).DeletePhoto(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if got := bucket.Deleted(); !slices.Equal(got, []string{key}) {
		t.Errorf("delete calls = %q, want one for the key", got)
	}
	if got := bucket.Keys(); !slices.Equal(got, sorted(others...)) {
		t.Errorf("left in the bucket = %q", got)
	}
}

func TestDeletePhoto_RefusesAnythingButAnEntryPhotoKeyWithNoCall(t *testing.T) {
	a, b := accountA.String(), accountB.String()
	const file = "0123456789abcdef"
	stored := "entries/" + a + "/" + file + ".jpg"
	for _, key := range []string{
		"",
		"entries/../x.jpg",
		"entries/" + a + "/../" + b + "/" + file + ".jpg",
		"entries/" + a + "/./" + file + ".jpg",
		"entries/" + a + "/x/" + file + ".jpg",
		"avatars/" + a + "/" + file + ".jpg",
		"entries/" + strings.ToUpper(a) + "/" + file + ".jpg",
		"entries/" + a + "/" + strings.ToUpper(file) + ".jpg",
		"entries/" + a + "/" + file + ".gif",
		"entries/" + a + "/" + file + ".JPG",
		"entries/" + a + "/" + file,
		"entries/" + a + "/",
		"entries/" + a,
		"entries/",
		"/" + stored,
		stored + "/",
		stored + " ",
	} {
		bucket := storagetest.NewBucket(testBucket, stored)
		err := NewAccountFilesOver(bucket, testBucket).DeletePhoto(context.Background(), key)
		if !errors.Is(err, errNotEntryPhotoKey) {
			t.Errorf("DeletePhoto(%q) = %v, want errNotEntryPhotoKey", key, err)
		}
		if got := bucket.Deleted(); len(got) != 0 {
			t.Errorf("DeletePhoto(%q) made delete calls %q", key, got)
		}
		if !bucket.Has(stored) {
			t.Errorf("DeletePhoto(%q) removed %q", key, stored)
		}
	}
}

func TestDeletePhoto_ReturnsTheStorageError(t *testing.T) {
	key := fileKey("entries", accountB, 1)
	bucket := storagetest.NewBucket(testBucket, key)
	boom := errors.New("storage is down")
	bucket.FailDelete = map[string]error{key: boom}

	if err := NewAccountFilesOver(bucket, testBucket).DeletePhoto(context.Background(), key); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the storage error", err)
	}
	if !bucket.Has(key) {
		t.Error("the key whose delete failed is gone")
	}
}

func TestNewAccountFiles_MissingConfigFieldReturnsError(t *testing.T) {
	full := Config{
		S3Endpoint:        "https://s3.example.test",
		S3Region:          "auto",
		S3Bucket:          "photos",
		S3AccessKeyID:     "key",
		S3SecretAccessKey: "secret",
		S3PublicBaseURL:   "https://cdn.example.test",
	}
	if _, err := NewAccountFiles(Config{}); err != ErrIncompleteConfig {
		t.Fatalf("empty config: error = %v, want ErrIncompleteConfig", err)
	}
	for name, mutate := range map[string]func(*Config){
		"endpoint":   func(c *Config) { c.S3Endpoint = "" },
		"bucket":     func(c *Config) { c.S3Bucket = "" },
		"access key": func(c *Config) { c.S3AccessKeyID = "" },
		"secret":     func(c *Config) { c.S3SecretAccessKey = "" },
		"public url": func(c *Config) { c.S3PublicBaseURL = "" },
	} {
		cfg := full
		mutate(&cfg)
		if _, err := NewAccountFiles(cfg); err != ErrIncompleteConfig {
			t.Errorf("missing %s: error = %v, want ErrIncompleteConfig", name, err)
		}
	}
	// A complete configuration builds with no network call, and an empty
	// region is accepted.
	for _, region := range []string{"auto", ""} {
		cfg := full
		cfg.S3Region = region
		files, err := NewAccountFiles(cfg)
		if err != nil || files == nil || files.api == nil || files.bucket != "photos" {
			t.Fatalf("region %q: files %+v, error %v", region, files, err)
		}
	}
}
