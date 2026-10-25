// Package storagetest holds Bucket, an in-memory stand-in for one S3 bucket,
// for the tests of code that deletes files through storage.ObjectAPI. It
// does not import package storage, so storage's own tests can use it without
// an import cycle; the check that *Bucket satisfies storage.ObjectAPI is in
// those tests.
package storagetest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// defaultPageSize is the page size when PageSize is zero. It is small so a
// test with only a handful of keys still goes through several pages.
const defaultPageSize = 2

// tokenPrefix starts every continuation token the bucket hands out. The rest
// of the token is the last key of the page it came with.
const tokenPrefix = "after:"

// Bucket is an in-memory stand-in for one S3 bucket. It answers
// ListObjectsV2 and DeleteObject the way S3 does for the fields the purge
// sends (Bucket, Prefix, ContinuationToken and MaxKeys), records every call,
// and can be told to fail. It is safe for concurrent use. Set PageSize,
// FailList and FailDelete before the bucket is in use.
type Bucket struct {
	// PageSize is the most keys one listing returns. Zero means 2. A
	// smaller MaxKeys in the request wins.
	PageSize int
	// FailList, when not nil, is the error every ListObjectsV2 call
	// returns.
	FailList error
	// FailDelete maps a key to the error DeleteObject returns for it. The
	// key stays in the bucket.
	FailDelete map[string]error

	name    string
	mu      sync.Mutex
	keys    map[string]struct{}
	deleted []string
	listed  []string
}

// NewBucket returns a bucket called name that holds keys.
func NewBucket(name string, keys ...string) *Bucket {
	b := &Bucket{name: name}
	b.Put(keys...)
	return b
}

// Put adds keys. A key that is already there stays as it is.
func (b *Bucket) Put(keys ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keys == nil {
		b.keys = make(map[string]struct{})
	}
	for _, key := range keys {
		b.keys[key] = struct{}{}
	}
}

// Keys returns the keys in the bucket, sorted.
func (b *Bucket) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Sorted(maps.Keys(b.keys))
}

// Has reports whether key is in the bucket.
func (b *Bucket) Has(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.keys[key]
	return ok
}

// Deleted returns every key DeleteObject was called with, in call order:
// keys that were not there and calls that failed are included, since a test
// usually wants to know what was asked for, not only what went.
func (b *Bucket) Deleted() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.deleted)
}

// Listed returns the prefix of every ListObjectsV2 call, one entry per call,
// in call order, failed calls included.
func (b *Bucket) Listed() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.listed)
}

// ListObjectsV2 lists, in lexical order, the keys that start with
// params.Prefix and come after the key in params.ContinuationToken, at most
// one page of them. When more remain, the answer is truncated and carries the
// token for the next page. The call is recorded in Listed first. A done ctx,
// another bucket name, FailList or a token this bucket did not make is an
// error.
func (b *Bucket) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if params == nil {
		params = &s3.ListObjectsV2Input{}
	}
	prefix := aws.ToString(params.Prefix)
	b.listed = append(b.listed, prefix)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if got := aws.ToString(params.Bucket); got != b.name {
		return nil, fmt.Errorf("storagetest: there is no bucket %q, this one is %q", got, b.name)
	}
	if b.FailList != nil {
		return nil, b.FailList
	}
	after := ""
	if params.ContinuationToken != nil {
		var ok bool
		after, ok = strings.CutPrefix(*params.ContinuationToken, tokenPrefix)
		if !ok {
			return nil, fmt.Errorf("storagetest: continuation token %q was not made by this bucket", *params.ContinuationToken)
		}
	}
	size := b.PageSize
	if size <= 0 {
		size = defaultPageSize
	}
	if maxKeys := aws.ToInt32(params.MaxKeys); maxKeys > 0 && int(maxKeys) < size {
		size = int(maxKeys)
	}

	var match []string
	for _, key := range slices.Sorted(maps.Keys(b.keys)) {
		if strings.HasPrefix(key, prefix) && key > after {
			match = append(match, key)
		}
	}
	page := match[:min(size, len(match))]
	more := len(page) < len(match)

	out := &s3.ListObjectsV2Output{
		Name:              aws.String(b.name),
		Prefix:            params.Prefix,
		ContinuationToken: params.ContinuationToken,
		MaxKeys:           aws.Int32(int32(size)),
		KeyCount:          aws.Int32(int32(len(page))),
		IsTruncated:       aws.Bool(more),
	}
	for _, key := range page {
		out.Contents = append(out.Contents, types.Object{Key: aws.String(key), Size: aws.Int64(0)})
	}
	if more {
		out.NextContinuationToken = aws.String(tokenPrefix + page[len(page)-1])
	}
	return out, nil
}

// DeleteObject deletes params.Key and, as S3 does, succeeds for a key that
// is not there. The call is recorded in Deleted first. A done ctx, another
// bucket name, an empty key or an error in FailDelete for the key is an
// error, and the key stays.
func (b *Bucket) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if params == nil {
		params = &s3.DeleteObjectInput{}
	}
	key := aws.ToString(params.Key)
	b.deleted = append(b.deleted, key)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if got := aws.ToString(params.Bucket); got != b.name {
		return nil, fmt.Errorf("storagetest: there is no bucket %q, this one is %q", got, b.name)
	}
	if key == "" {
		return nil, errors.New("storagetest: a delete needs a key")
	}
	if err := b.FailDelete[key]; err != nil {
		return nil, err
	}
	delete(b.keys, key)
	return &s3.DeleteObjectOutput{}, nil
}
