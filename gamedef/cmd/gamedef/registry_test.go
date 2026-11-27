package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func digestOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(sum[:])
}

const (
	indexBoth  = `{"manifests":[{"platform":{"os":"linux","architecture":"amd64"}},{"platform":{"os":"linux","architecture":"arm64"}}]}`
	indexArm   = `{"manifests":[{"platform":{"os":"linux","architecture":"arm64"}}]}`
	singleBody = `{"config":{}}`
	ociIndex   = "application/vnd.oci.image.index.v1+json"
	ociImage   = "application/vnd.oci.image.manifest.v1+json"
)

// fakeRegistry serves manifests behind an anonymous bearer challenge, a token endpoint and
// the Docker Hub tags API. It counts manifest GETs (Docker Hub pulls).
func fakeRegistry(t *testing.T) (*registryChecker, *atomic.Int32) {
	t.Helper()
	manifests := map[string][2]string{ // path -> media type, body
		"/v2/eggs/yolks/manifests/" + digestOf(indexBoth):   {ociIndex, indexBoth},
		"/v2/eggs/arm/manifests/" + digestOf(indexArm):      {ociIndex, indexArm},
		"/v2/eggs/single/manifests/" + digestOf(singleBody): {ociImage, singleBody},
		"/v2/ipshosting/game-x/manifests/v1":                {ociIndex, indexBoth},
		"/v2/itzg/mc/manifests/" + digestOf(indexBoth):      {ociIndex, indexBoth},
	}
	var gets atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			if !strings.HasPrefix(r.URL.Query().Get("scope"), "repository:") {
				t.Errorf("token request without scope: %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"token":"t"}`))
			return
		case r.URL.Path == "/v2/namespaces/itzg/repositories/mc/tags/java21":
			_, _ = w.Write([]byte(`{"digest":"` + digestOf(indexBoth) + `","images":[{"os":"linux","architecture":"amd64"}]}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer t" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+server.URL+`/token",service="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		manifest, ok := manifests[r.URL.Path]
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/v2/ipshosting/") {
				w.WriteHeader(http.StatusUnauthorized) // Docker Hub style for a missing repository
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet {
			gets.Add(1)
		}
		w.Header().Set("Content-Type", manifest[0])
		w.Header().Set("Docker-Content-Digest", digestOf(manifest[1]))
		_, _ = w.Write([]byte(manifest[1]))
	}))
	t.Cleanup(server.Close)
	return &registryChecker{client: server.Client(), timeout: 5 * time.Second, endpoint: func(string) string { return server.URL }, hub: server.URL}, &gets
}

func TestRegistryCheck(t *testing.T) {
	checker, gets := fakeRegistry(t)
	missing := "sha256:" + strings.Repeat("0", 64)
	cases := []struct{ ref, status, detail string }{
		{"ghcr.io/eggs/yolks:debian@" + digestOf(indexBoth), statusOK, "linux/arm64: yes"},
		{"ghcr.io/eggs/single:x@" + digestOf(singleBody), statusOK, "single-platform"},
		{"ghcr.io/eggs/arm:x@" + digestOf(indexArm), statusFail, "no linux/amd64"},
		{"ghcr.io/eggs/yolks:debian@" + missing, statusFail, "not found"},
		{"ipshosting/game-x:v1", statusOK, "index"},
		{"ipshosting/installer:v1", statusWarn, "not a failure"},
		{"ghcr.io/eggs/yolks:latest", statusFail, "not found"},
	}
	for _, c := range cases {
		result := checker.check(context.Background(), c.ref)
		if result.Status != c.status || !strings.Contains(result.Detail, c.detail) {
			t.Errorf("%s: got %s (%s), want %s (%s)", c.ref, result.Status, result.Detail, c.status, c.detail)
		}
	}

	// Docker Hub images take their platforms from the tags API: no manifest GET.
	before := gets.Load()
	if result := checker.check(context.Background(), "itzg/mc:java21@"+digestOf(indexBoth)); result.Status != statusOK || result.Detail != "index, linux/arm64: no" {
		t.Errorf("hub: got %+v", result)
	}
	if gets.Load() != before {
		t.Errorf("hub image needed a manifest GET")
	}
}

func TestReportImages(t *testing.T) {
	checker, _ := fakeRegistry(t)
	refs := []string{"ghcr.io/eggs/yolks:debian@" + digestOf(indexBoth), "ipshosting/installer:v1"}
	var out bytes.Buffer
	if err := reportImages(&out, checker.checkAll(context.Background(), refs)); err != nil {
		t.Fatalf("a missing own tag must not fail: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "2 image(s): 1 ok, 1 warning(s), 0 failed") || !strings.Contains(out.String(), "::warning::ipshosting/installer:v1") {
		t.Fatalf("output:\n%s", out.String())
	}
	refs = append(refs, "ghcr.io/eggs/arm:x@"+digestOf(indexArm))
	if err := reportImages(&out, checker.checkAll(context.Background(), refs)); err == nil {
		t.Fatalf("a missing linux/amd64 must fail")
	}
}
