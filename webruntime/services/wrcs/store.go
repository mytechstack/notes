package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Versioned manifest storage in S3 (MinIO locally; any S3 API works).
//
//   manifests/{tenant}/v{n}.json     immutable, environment-agnostic: built once,
//                                    promoted dev → … → prod unchanged
//   pointers/{tenant}/{env}.json     one per environment, the only mutable objects:
//                                    {"stable":n,"candidate":n,"canary":{"version":n,"percent":p}}
//   releases/{tenant}/v{n}.json      release record: where the version has been stable
//
// The bucket is anonymously readable so the Web Runtime Service reads it
// like a CDN; only WCR holds write credentials.

type Canary struct {
	Version int `json:"version"`
	Percent int `json:"percent"`
}

type Pointers struct {
	Stable    int    `json:"stable"`
	Candidate int    `json:"candidate"`
	Canary    Canary `json:"canary"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type versionEntry struct {
	N        int
	Modified time.Time
}

var (
	s3c    *minio.Client
	bucket string
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func initStore() error {
	bucket = envOr("S3_BUCKET", "web-manifests")
	c, err := minio.New(envOr("S3_ENDPOINT", "minio:9000"), &minio.Options{
		Creds:  credentials.NewStaticV4(envOr("S3_ACCESS_KEY", ""), envOr("S3_SECRET_KEY", ""), ""),
		Secure: false,
	})
	if err != nil {
		return err
	}
	s3c = c

	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = ensureBucket(); lastErr == nil {
			return nil
		}
		log.Printf("waiting for S3: %v", lastErr)
		time.Sleep(time.Second)
	}
	return lastErr
}

func ensureBucket() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := s3c.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s3c.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	// Readers (Web Runtime Service, CDN) need anonymous GetObject. Stores
	// that don't implement bucket policies grant it through their own
	// access config instead, so a failure here is logged, not fatal.
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},`+
		`"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
	if err := s3c.SetBucketPolicy(ctx, bucket, policy); err != nil {
		log.Printf("bucket policy not applied (%v); relying on the store's own anonymous-read config", err)
	}
	return nil
}

func versionKey(tenant string, n int) string {
	return fmt.Sprintf("manifests/%s/v%d.json", tenant, n)
}

func pointerKey(tenant, env string) string {
	return fmt.Sprintf("pointers/%s/%s.json", tenant, env)
}

func putJSON(key string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s3c.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/json"})
	return err
}

// getObject returns (nil, nil) when the key does not exist.
func getObject(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	obj, err := s3c.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

var versionFile = regexp.MustCompile(`/v(\d+)\.json$`)

func listVersionEntries(tenant string) ([]versionEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out []versionEntry
	prefix := fmt.Sprintf("manifests/%s/", tenant)
	for o := range s3c.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if o.Err != nil {
			return nil, o.Err
		}
		if m := versionFile.FindStringSubmatch(o.Key); m != nil {
			n, _ := strconv.Atoi(m[1])
			out = append(out, versionEntry{N: n, Modified: o.LastModified})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out, nil
}

func listVersions(tenant string) ([]int, error) {
	entries, err := listVersionEntries(tenant)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(entries))
	for i, e := range entries {
		out[i] = e.N
	}
	return out, nil
}

func nextVersion(tenant string) (int, error) {
	versions, err := listVersions(tenant)
	if err != nil {
		return 0, err
	}
	if len(versions) == 0 {
		return 1, nil
	}
	return versions[len(versions)-1] + 1, nil
}

func writeVersion(tenant string, version int, data []byte) error {
	var pretty map[string]interface{}
	if err := json.Unmarshal(data, &pretty); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}
	return putJSON(versionKey(tenant, version), out)
}

func readVersion(tenant string, version int) ([]byte, error) {
	data, err := getObject(versionKey(tenant, version))
	if err == nil && data == nil {
		return nil, fmt.Errorf("version %d not found", version)
	}
	return data, err
}

func readPointers(tenant, env string) (Pointers, error) {
	var p Pointers
	data, err := getObject(pointerKey(tenant, env))
	if err != nil || data == nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	return p, err
}

func writePointers(tenant, env string, p Pointers) error {
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return putJSON(pointerKey(tenant, env), data)
}

// listTenants derives the tenant list from the manifest objects.
func listTenants() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seen := map[string]bool{}
	out := []string{}
	for o := range s3c.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: "pointers/", Recursive: true}) {
		if o.Err != nil {
			return nil, o.Err
		}
		t, _, ok := strings.Cut(strings.TrimPrefix(o.Key, "pointers/"), "/")
		if ok && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out, nil
}
