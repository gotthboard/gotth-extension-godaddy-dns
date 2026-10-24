package godaddydns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	pageSize     = 100
	maxPages     = 10
	maxRecords   = 1000
	maxWalkBytes = 4 << 20
)

type providerResult int

const (
	providerAccepted providerResult = iota
	providerRejected
	providerUnavailable
)

type providerClient struct {
	base    string
	pat     []byte
	http    *http.Client
	timeout time.Duration
}

type pageResponse struct {
	Items      []apiRecord `json:"items"`
	TotalItems *int        `json:"totalItems,omitempty"`
	TotalPages *int        `json:"totalPages,omitempty"`
	Links      []pageLink  `json:"links"`
}
type pageLink struct {
	Rel                 string          `json:"rel"`
	Href                string          `json:"href"`
	Title               string          `json:"title,omitempty"`
	TargetMediaType     string          `json:"targetMediaType,omitempty"`
	TargetSchema        json.RawMessage `json:"targetSchema,omitempty"`
	Method              string          `json:"method,omitempty"`
	SubmissionMediaType string          `json:"submissionMediaType,omitempty"`
	SubmissionSchema    json.RawMessage `json:"submissionSchema,omitempty"`
}

// Complexity: time O(p), Omega(p), tight Theta(p); auxiliary space O(p),
// Omega(p), tight Theta(p); variable: p is the PAT byte length copied once.
func newProviderClient(config Config, pat []byte) *providerClient {
	base := providerProductionBase
	if config.Environment == "ote" {
		base = providerOTEBase
	}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableCompression: true, MaxIdleConns: 2, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 2, IdleConnTimeout: 30 * time.Second, MaxResponseHeaderBytes: 64 << 10}
	return &providerClient{base: base, pat: append([]byte(nil), pat...), timeout: config.Timeout(), http: &http.Client{Transport: transport, Timeout: config.Timeout(), CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect disabled") }}}
}

func (c *providerClient) close() {
	clear(c.pat)
	if t, ok := c.http.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

func (c *providerClient) observe(ctx context.Context, zone, correlationID string) (snapshot, error) {
	observationCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	left, err := c.walk(observationCtx, zone, correlationID)
	if err != nil {
		return snapshot{}, err
	}
	right, err := c.walk(observationCtx, zone, correlationID)
	if err != nil {
		return snapshot{}, err
	}
	if !snapshotsEqual(left, right) {
		return snapshot{}, errors.New("zone changed during observation")
	}
	return right, nil
}

func (c *providerClient) walk(ctx context.Context, zone, correlationID string) (snapshot, error) {
	var all []apiRecord
	remaining := int64(maxWalkBytes)
	expectedItems, expectedPages := -1, -1
	for page := 1; page <= maxPages; page++ {
		u := c.base + "/zones/" + url.PathEscape(zone) + "/dns-records?page=" + strconv.Itoa(page) + "&pageSize=" + strconv.Itoa(pageSize) + "&totalRequired=true"
		req, err := c.request(ctx, http.MethodGet, u, correlationID, nil)
		if err != nil {
			return snapshot{}, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			return snapshot{}, errors.New("provider observation unavailable")
		}
		if resp.StatusCode != http.StatusOK || !jsonContentType(resp.Header.Get("Content-Type")) {
			discard(resp.Body)
			resp.Body.Close()
			return snapshot{}, errors.New("provider observation rejected")
		}
		body, err := readBounded(resp.Body, remaining)
		resp.Body.Close()
		if err != nil {
			return snapshot{}, err
		}
		remaining -= int64(len(body))
		var out pageResponse
		if err := strictDecode(body, &out); err != nil {
			return snapshot{}, errors.New("invalid provider observation")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			return snapshot{}, errors.New("invalid provider observation")
		}
		totalItemsJSON, hasTotalItems := fields["totalItems"]
		totalPagesJSON, hasTotalPages := fields["totalPages"]
		if (hasTotalItems && bytes.Equal(totalItemsJSON, []byte("null"))) || (hasTotalPages && bytes.Equal(totalPagesJSON, []byte("null"))) {
			return snapshot{}, errors.New("null provider totals forbidden")
		}
		if out.Items == nil || out.Links == nil {
			return snapshot{}, errors.New("provider arrays required")
		}
		if len(out.Links) > 16 || len(out.Items) > pageSize {
			return snapshot{}, errors.New("invalid provider links")
		}
		for _, link := range out.Links {
			if len(link.Rel) < 1 || len(link.Rel) > 32 || len(link.Href) < 1 || len(link.Href) > 2048 || !validText(link.Rel) || !validText(link.Href) {
				return snapshot{}, errors.New("invalid provider links")
			}
		}
		if !hasTotalItems && !hasTotalPages && len(out.Items) == 0 {
			if page != 1 || len(out.Links) != 0 {
				return snapshot{}, errors.New("invalid empty provider page")
			}
			return buildSnapshot(zone, nil)
		}
		if len(out.Links) < 1 || !hasTotalItems || !hasTotalPages || out.TotalItems == nil || out.TotalPages == nil || *out.TotalItems < 1 || *out.TotalItems > maxRecords || *out.TotalPages < 1 || *out.TotalPages > maxPages {
			return snapshot{}, errors.New("invalid provider pagination")
		}
		if page == 1 {
			expectedItems, expectedPages = *out.TotalItems, *out.TotalPages
			calculatedPages := (expectedItems + pageSize - 1) / pageSize
			if calculatedPages < 1 {
				calculatedPages = 1
			}
			if expectedPages != calculatedPages {
				return snapshot{}, errors.New("provider page count mismatch")
			}
		} else if *out.TotalItems != expectedItems || *out.TotalPages != expectedPages {
			return snapshot{}, errors.New("provider pagination changed")
		}
		all = append(all, out.Items...)
		if len(all) > maxRecords {
			return snapshot{}, errors.New("provider record limit exceeded")
		}
		expectedOnPage := pageSize
		if page == expectedPages {
			expectedOnPage = expectedItems - pageSize*(page-1)
		}
		if len(out.Items) != expectedOnPage {
			return snapshot{}, errors.New("provider page size mismatch")
		}
		if page == expectedPages {
			if len(all) != expectedItems {
				return snapshot{}, errors.New("provider total mismatch")
			}
			return buildSnapshot(zone, all)
		}
	}
	return snapshot{}, errors.New("provider page limit exceeded")
}

func (c *providerClient) mutate(ctx context.Context, method, zone, recordID, correlationID string, record *apiRecord) providerResult {
	u := c.base + "/zones/" + url.PathEscape(zone) + "/dns-records"
	if recordID != "" {
		u += "/" + url.PathEscape(recordID)
	}
	var body []byte
	if record != nil {
		body, _ = json.Marshal(record)
	}
	req, err := c.request(ctx, method, u, correlationID, body)
	if err != nil {
		return providerUnavailable
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return providerUnavailable
	}
	defer resp.Body.Close()
	expected := method == http.MethodPost && resp.StatusCode == http.StatusCreated || method == http.MethodPut && resp.StatusCode == http.StatusOK || method == http.MethodDelete && resp.StatusCode == http.StatusNoContent
	if expected {
		if method != http.MethodDelete && !jsonContentType(resp.Header.Get("Content-Type")) {
			discard(resp.Body)
			return providerUnavailable
		}
		data, err := readBounded(resp.Body, 64<<10)
		if err != nil {
			return providerUnavailable
		}
		if method != http.MethodDelete {
			var returned apiRecord
			if strictDecode(data, &returned) != nil {
				return providerUnavailable
			}
			if _, err := canonicalRecord(returned, true); err != nil {
				return providerUnavailable
			}
		}
		return providerAccepted
	}
	discard(resp.Body)
	switch resp.StatusCode {
	case 400, 401, 403, 404, 409, 422:
		return providerRejected
	default:
		return providerUnavailable
	}
}

func (c *providerClient) request(ctx context.Context, method, target, correlationID string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+string(c.pat))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gotth-extension-godaddy-dns/1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Request-Id", correlationID)
	return req, nil
}
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, errors.New("response limit exceeded")
	}
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("bounded provider response required")
	}
	return b, nil
}
func discard(r io.Reader) { _, _ = io.Copy(io.Discard, io.LimitReader(r, 4096)) }

func jsonContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func strictDecode(data []byte, target any) error {
	if len(data) == 0 {
		return errors.New("JSON response required")
	}
	if err := rejectDuplicateJSONNames(data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func rejectDuplicateJSONNames(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func() error
	walk = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return errors.New("duplicate JSON name")
				}
				seen[s] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case '[':
			for d.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		default:
			return fmt.Errorf("invalid JSON delimiter %q", delim)
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}
