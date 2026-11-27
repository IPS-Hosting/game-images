package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ips-hosting/game-images/gamedef"
)

// Manifest media types accepted from registries.
var manifestTypes = []string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
}

func isIndex(mediaType string) bool {
	return strings.HasPrefix(mediaType, "application/vnd.oci.image.index.v1+json") ||
		strings.HasPrefix(mediaType, "application/vnd.docker.distribution.manifest.list.v2+json")
}

// Check outcomes.
const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

type imageResult struct {
	Ref    string
	Status string
	Detail string
}

// registryChecker checks image references against the registry v2 API without pulling.
type registryChecker struct {
	client  *http.Client
	timeout time.Duration
	// endpoint maps a registry host to its API base URL.
	endpoint func(host string) string
	// hub is the Docker Hub API base URL. Docker Hub counts every manifest GET as a pull
	// (anonymous: 100 per hour and IP), so the platforms of its images come from the tags
	// API when the tag still points at the digest.
	hub string
}

func newRegistryChecker() *registryChecker {
	return &registryChecker{
		client:  &http.Client{},
		timeout: 20 * time.Second,
		endpoint: func(host string) string {
			if host == "docker.io" {
				host = "registry-1.docker.io"
			}
			return "https://" + host
		},
		hub: "https://hub.docker.com",
	}
}

// checkAll checks the references with at most 8 checks in flight, in input order.
func (c *registryChecker) checkAll(ctx context.Context, refs []string) []imageResult {
	results := make([]imageResult, len(refs))
	limit := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			results[i] = c.check(ctx, ref)
		})
	}
	wg.Wait()
	return results
}

// imageRef is a reference split into registry host, repository, tag and the manifest
// reference to request (the digest if pinned, else the tag).
type imageRef struct {
	host, repository, tag, reference string
}

func splitRef(ref string) (imageRef, bool) {
	host, repository, _ := strings.Cut(gamedef.NormalizeImage(ref), "/")
	r := imageRef{host: host}
	if name, digest, ok := strings.Cut(repository, "@"); ok {
		repository, r.reference = name, digest
	}
	if i := strings.LastIndex(repository, ":"); i > strings.LastIndex(repository, "/") {
		repository, r.tag = repository[:i], repository[i+1:]
	}
	r.repository = repository
	if r.reference == "" {
		r.reference = r.tag
	}
	return r, r.reference != ""
}

// check verifies that a digest (or, for own images, a tag) exists and that an index
// contains linux/amd64. A missing tag of an own image is a warning only.
func (c *registryChecker) check(ctx context.Context, ref string) imageResult {
	result := imageResult{Ref: ref, Status: statusFail}
	r, ok := splitRef(ref)
	if !ok {
		result.Detail = "no tag or digest"
		return result
	}
	pinned := strings.HasPrefix(r.reference, "sha256:")
	own := false
	for _, prefix := range gamedef.OwnImagePrefixes {
		own = own || strings.HasPrefix(gamedef.NormalizeImage(ref), prefix)
	}

	target := c.endpoint(r.host) + "/v2/" + r.repository + "/manifests/" + r.reference
	response, body, err := c.get(ctx, http.MethodHead, target, r.repository)
	var platforms map[string]bool
	if err == nil && response.StatusCode == http.StatusOK && isIndex(response.Header.Get("Content-Type")) {
		// Only an index needs its body (the platform list); HEAD does not count as a pull.
		if r.host == "docker.io" && r.tag != "" {
			platforms = c.hubPlatforms(ctx, r, response.Header.Get("Docker-Content-Digest"))
		}
		if platforms == nil {
			response, body, err = c.get(ctx, http.MethodGet, target, r.repository)
		}
	}
	switch {
	case err != nil:
		result.Detail = err.Error()
		return result
	// Docker Hub answers 401 for a repository that does not exist.
	case own && !pinned && (response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnauthorized):
		result.Status, result.Detail = statusWarn, "own tag not published yet (warning only, not a failure)"
		return result
	case response.StatusCode == http.StatusNotFound:
		result.Detail = "manifest not found"
		return result
	case response.StatusCode != http.StatusOK:
		result.Detail = "registry answered " + response.Status
		return result
	}
	if pinned {
		got := response.Header.Get("Docker-Content-Digest")
		if body != nil {
			sum := sha256.Sum256(body)
			got = "sha256:" + hex.EncodeToString(sum[:])
		}
		if got != "" && got != r.reference {
			result.Detail = "registry returned digest " + got
			return result
		}
	}
	if body == nil && platforms == nil {
		result.Status, result.Detail = statusOK, "single-platform manifest"
		return result
	}
	if platforms == nil {
		if platforms, err = indexPlatforms(body); err != nil {
			result.Detail = err.Error()
			return result
		}
	}
	if !platforms["linux/amd64"] {
		result.Detail = "index has no linux/amd64"
		return result
	}
	result.Status, result.Detail = statusOK, "index, linux/arm64: no"
	if platforms["linux/arm64"] {
		result.Detail = "index, linux/arm64: yes"
	}
	return result
}

func indexPlatforms(body []byte) (map[string]bool, error) {
	var index struct {
		Manifests []struct {
			Platform struct {
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("invalid index: %w", err)
	}
	platforms := map[string]bool{}
	for _, manifest := range index.Manifests {
		platforms[manifest.Platform.OS+"/"+manifest.Platform.Architecture] = true
	}
	return platforms, nil
}

// hubPlatforms reads the platforms of a Docker Hub tag from the Hub API when the tag still
// points at digest; nil means unknown (the caller falls back to the registry).
func (c *registryChecker) hubPlatforms(ctx context.Context, r imageRef, digest string) map[string]bool {
	namespace, name, ok := strings.Cut(r.repository, "/")
	if !ok || digest == "" {
		return nil
	}
	response, body, err := c.do(ctx, http.MethodGet, c.hub+"/v2/namespaces/"+namespace+"/repositories/"+name+"/tags/"+url.PathEscape(r.tag), "")
	if err != nil || response.StatusCode != http.StatusOK {
		return nil
	}
	var tag struct {
		Digest string `json:"digest"`
		Images []struct {
			OS           string `json:"os"`
			Architecture string `json:"architecture"`
		} `json:"images"`
	}
	if json.Unmarshal(body, &tag) != nil || tag.Digest != digest {
		return nil
	}
	platforms := map[string]bool{}
	for _, image := range tag.Images {
		platforms[image.OS+"/"+image.Architecture] = true
	}
	return platforms
}

// get performs one manifest request, answering a bearer challenge with an anonymous token.
func (c *registryChecker) get(ctx context.Context, method, target, repository string) (*http.Response, []byte, error) {
	response, body, err := c.do(ctx, method, target, "")
	if err != nil || response.StatusCode != http.StatusUnauthorized {
		return response, body, err
	}
	token, err := c.token(ctx, response.Header.Get("WWW-Authenticate"), repository)
	if err != nil {
		return nil, nil, err
	}
	return c.do(ctx, method, target, token)
}

// do performs one request with the per-request timeout and reads the body of a successful
// GET (at most 4 MiB).
func (c *registryChecker) do(ctx context.Context, method, target, token string) (*http.Response, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", strings.Join(manifestTypes, ", "))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	var body []byte
	if method == http.MethodGet && response.StatusCode == http.StatusOK {
		if body, err = io.ReadAll(io.LimitReader(response.Body, 4<<20)); err != nil {
			return nil, nil, err
		}
	}
	return response, body, nil
}

// token fetches an anonymous pull token for a Bearer challenge
// (realm="...",service="...",scope="...").
func (c *registryChecker) token(ctx context.Context, challenge, repository string) (string, error) {
	scheme, params, _ := strings.Cut(challenge, " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return "", fmt.Errorf("unsupported registry challenge %q", challenge)
	}
	values := map[string]string{}
	for _, part := range strings.Split(params, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		values[strings.ToLower(key)] = strings.Trim(value, `"`)
	}
	if values["realm"] == "" {
		return "", fmt.Errorf("registry challenge without realm")
	}
	query := url.Values{}
	if values["service"] != "" {
		query.Set("service", values["service"])
	}
	scope := values["scope"]
	if scope == "" {
		scope = "repository:" + repository + ":pull"
	}
	query.Set("scope", scope)
	response, body, err := c.do(ctx, http.MethodGet, values["realm"]+"?"+query.Encode(), "")
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint answered %s", response.Status)
	}
	var token struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("invalid token response: %w", err)
	}
	if token.Token == "" {
		token.Token = token.AccessToken
	}
	return token.Token, nil
}
