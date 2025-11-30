package anilist

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"seanime/internal/util/filecache"
	"sync/atomic"
	"sync"
	"time"
)

// sharedCacher is injected by the app so AniList calls can persist cache to disk
var sharedCacher *filecache.Cacher
var sharedCacherOnce sync.Once

// Buckets to control TTL per query type
var (
	bucketShort  = filecache.NewBucket("anilist_short", 2*time.Minute)   // e.g., airing schedule
	bucketMedium = filecache.NewBucket("anilist_medium", 10*time.Minute) // e.g., collections, viewer
	bucketLong   = filecache.NewBucket("anilist_long", 24*time.Hour)     // e.g., media by ID/details
)

// SetSharedCacher wires the app's file cacher into the AniList package
func SetSharedCacher(c *filecache.Cacher) {
	sharedCacherOnce.Do(func() {
		sharedCacher = c
	})
}

// keyHash produces a compact hash for complex keys
func keyHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// formatKey creates a namespaced cache key and returns a stable hashed version
func formatKey(prefix string, parts ...interface{}) string {
	k := prefix
	for _, p := range parts {
		k += "|" + fmt.Sprintf("%v", p)
	}
	return keyHash(k)
}

// --------------------------------------------------------------------------------
// In-memory cache (fast path) + request coalescing
// --------------------------------------------------------------------------------

type memItem struct {
    value any
    exp   time.Time
}

var (
    memMu   sync.Mutex
    memData = make(map[string]memItem)
    // shorter in-memory TTLs to keep UI snappy yet fresh
    bucketMemTTL = map[string]time.Duration{
        "anilist_short":  30 * time.Second,
        "anilist_medium": 60 * time.Second,
        "anilist_long":   5 * time.Minute,
    }
)

func memGet(key string) (any, bool) {
    memMu.Lock()
    defer memMu.Unlock()
    it, ok := memData[key]
    if !ok || time.Now().After(it.exp) {
        if ok {
            delete(memData, key)
        }
        return nil, false
    }
    return it.value, true
}

func memSet(bucketName, key string, v any) {
    ttl := bucketMemTTL[bucketName]
    if ttl <= 0 {
        ttl = 30 * time.Second
    }
    memMu.Lock()
    memData[key] = memItem{value: v, exp: time.Now().Add(ttl)}
    memMu.Unlock()
}

// simple inflight dedupe (singleflight-lite)
type inflightCall struct {
    wait sync.WaitGroup
    res  any
    err  error
}

var (
    inflightMu   sync.Mutex
    inflight     = make(map[string]*inflightCall)
    inflightSize int64
)

func startInflight(key string) (*inflightCall, bool) {
    inflightMu.Lock()
    if c, ok := inflight[key]; ok {
        inflightMu.Unlock()
        return c, false
    }
    c := &inflightCall{}
    c.wait.Add(1)
    inflight[key] = c
    atomic.AddInt64(&inflightSize, 1)
    inflightMu.Unlock()
    return c, true
}

func doneInflight(key string, c *inflightCall, res any, err error) {
    inflightMu.Lock()
    if inflight[key] == c {
        delete(inflight, key)
        atomic.AddInt64(&inflightSize, -1)
    }
    inflightMu.Unlock()
    c.res, c.err = res, err
    c.wait.Done()
}

// getCached tries in-memory then file cache
func getCached[T any](bucket filecache.Bucket, key string, out *T) (bool, error) {
    if v, ok := memGet(key); ok {
        // type assert
        if cast, ok2 := v.(T); ok2 {
            *out = cast
            return true, nil
        }
    }
    if sharedCacher == nil {
        return false, nil
    }
    var tmp T
    ok, err := sharedCacher.Get(bucket, key, &tmp)
    if err != nil || !ok {
        return ok, err
    }
    // populate mem cache for fast subsequent hits
    memSet(bucket.Name(), key, tmp)
    *out = tmp
    return true, nil
}

func setCached[T any](bucket filecache.Bucket, key string, val T) {
    // set mem
    memSet(bucket.Name(), key, val)
    // set disk (best-effort)
    if sharedCacher != nil {
        _ = sharedCacher.Set(bucket, key, val)
    }
}

// CachedFetch provides a generic helper that:
// 1) Attempts in-memory cache
// 2) Falls back to file cache
// 3) Coalesces concurrent requests for the same key
// 4) On miss, executes fetch(), then caches the result in memory + file
func CachedFetch[T any](bucket filecache.Bucket, key string, fetch func() (T, error)) (T, error) {
    var zero T
    // Fast path: mem + file
    if ok, err := getCached(bucket, key, &zero); ok || err != nil {
        return zero, err
    }

    // Inflight coalescing
    call, leader := startInflight(key)
    if !leader {
        call.wait.Wait()
        if call.err != nil {
            return zero, call.err
        }
        // type assert
        if v, ok := call.res.(T); ok {
            return v, nil
        }
        return zero, fmt.Errorf("anilist cache: type assertion failed for key %s", key)
    }

    // Leader: perform fetch
    defer func() {
        // ensure inflight is cleaned in case of panic
        if r := recover(); r != nil {
            doneInflight(key, call, nil, fmt.Errorf("panic: %v", r))
        }
    }()

    val, err := fetch()
    if err == nil {
        setCached(bucket, key, val)
    }
    doneInflight(key, call, any(val), err)
    return val, err
}
