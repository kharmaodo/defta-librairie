package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrBusy            = errors.New("experimental OCR is busy")
	ErrInvalidResponse = errors.New("invalid experimental OCR response")
)

// Result carries measured engine metadata. It never carries catalogue candidates.
type Result struct {
	Engine, EngineVersion, PolicyVersion, Language string
	TextRaw, TextNormalized                        string
	Confidence                                     *float64
	PSM                                            int
	Preprocessing                                  string
}

type DetailedRunner interface {
	Extract(context.Context, []byte, string, string) (Result, error)
}

type HTTP struct {
	endpoint string
	client   *http.Client
	timeout  time.Duration
}

// ValidateEndpoint accepts an origin only; credentials, paths and redirects are forbidden.
func ValidateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || u.ForceQuery {
		return ErrInvalidInput
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !privateIP(ip) {
		return ErrInvalidInput
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return ErrInvalidInput
		}
	}
	return nil
}

func privateIP(ip net.IP) bool { return ip.IsLoopback() || ip.IsPrivate() }

func NewHTTP(endpoint string, timeout time.Duration) (*HTTP, error) {
	if ValidateEndpoint(endpoint) != nil || timeout <= 0 || timeout > 180*time.Second {
		return nil, ErrInvalidInput
	}
	// No environment proxy and no public destination, even after DNS resolution.
	transport := &http.Transport{MaxIdleConns: 2, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, ErrUnavailable
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, ErrUnavailable
			}
			for _, ip := range ips {
				if !privateIP(ip.IP) {
					continue
				}
				connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return connection, nil
				}
			}
			return nil, ErrUnavailable
		}}
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &HTTP{endpoint: strings.TrimRight(endpoint, "/") + "/v1/ocr", client: client, timeout: timeout}, nil
}

func (h *HTTP) Run(ctx context.Context, image []byte, contentType string) (string, error) {
	result, err := h.Extract(ctx, image, contentType, "")
	return result.TextRaw, err
}

func (h *HTTP) Extract(ctx context.Context, image []byte, contentType, correlation string) (Result, error) {
	var empty Result
	if h == nil || ctx == nil || len(image) == 0 || len(image) > 10*1024*1024 || (contentType != "image/png" && contentType != "image/jpeg") {
		return empty, ErrInvalidInput
	}
	if (contentType == "image/png" && !bytes.HasPrefix(image, []byte("\x89PNG\r\n\x1a\n"))) || (contentType == "image/jpeg" && !bytes.HasPrefix(image, []byte{0xff, 0xd8, 0xff})) {
		return empty, ErrInvalidInput
	}
	deadline, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Disposition", `form-data; name="image"; filename="source"`)
	headers.Set("Content-Type", contentType)
	part, err := writer.CreatePart(headers)
	if err != nil {
		return empty, ErrInvalidInput
	}
	if _, err = part.Write(image); err != nil {
		return empty, ErrInvalidInput
	}
	if err = writer.Close(); err != nil {
		return empty, ErrInvalidInput
	}
	request, err := http.NewRequestWithContext(deadline, http.MethodPost, h.endpoint, &body)
	if err != nil {
		return empty, ErrInvalidInput
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	if correlation != "" && len(correlation) <= 128 && strings.IndexFunc(correlation, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r))
	}) < 0 {
		request.Header.Set("X-Request-Id", correlation)
	}
	reply, err := h.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		if deadline.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return empty, ErrTimeout
		}
		return empty, ErrUnavailable
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		if reply.StatusCode == 504 {
			return empty, ErrTimeout
		}
		if reply.StatusCode == 413 || reply.StatusCode == 422 {
			return empty, ErrInvalidInput
		}
		if reply.StatusCode == 503 {
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			_ = json.NewDecoder(io.LimitReader(reply.Body, 8192)).Decode(&envelope)
			if envelope.Error.Code == "OCR_BUSY" {
				return empty, ErrBusy
			}
		}
		return empty, ErrUnavailable
	}
	mediaType, _, err := mime.ParseMediaType(reply.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return empty, ErrInvalidResponse
	}
	data, err := io.ReadAll(io.LimitReader(reply.Body, 8*1024*1024+1))
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		if deadline.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return empty, ErrTimeout
		}
		return empty, ErrUnavailable
	}
	if len(data) > 8*1024*1024 || !utf8.Valid(data) {
		return empty, ErrInvalidResponse
	}
	return decodeResult(data)
}

func decodeResult(data []byte) (Result, error) {
	var wire struct {
		SchemaVersion  int             `json:"schemaVersion"`
		Engine         string          `json:"engine"`
		EngineVersion  string          `json:"engineVersion"`
		PolicyVersion  string          `json:"policyVersion"`
		Language       string          `json:"language"`
		TextRaw        *string         `json:"textRaw"`
		TextNormalized *string         `json:"textNormalized"`
		Confidence     json.RawMessage `json:"confidence"`
		PSM            int             `json:"psm"`
		Preprocessing  string          `json:"preprocessing"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if !uniqueResponseFields(data) || decoder.Decode(&wire) != nil {
		return Result{}, ErrInvalidResponse
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return Result{}, ErrInvalidResponse
	}
	if wire.SchemaVersion != 1 || wire.Engine != "tesseract-experimental" || wire.PolicyVersion != "ocr-local-v1" || wire.Language != "ara" || !strings.HasPrefix(wire.EngineVersion, "tesseract 5.") || len(wire.EngineVersion) <= len("tesseract 5.") || len(wire.EngineVersion) > 128 || strings.IndexFunc(wire.EngineVersion, unicode.IsControl) >= 0 || wire.TextRaw == nil || wire.TextNormalized == nil || len(*wire.TextRaw) > 1024*1024 || len(*wire.TextNormalized) > 1024*1024 || (wire.PSM != 6 && wire.PSM != 11) || (wire.Preprocessing != "original" && wire.Preprocessing != "grayscale-autocontrast") || len(wire.Confidence) == 0 {
		return Result{}, ErrInvalidResponse
	}
	var confidence *float64
	if string(wire.Confidence) != "null" {
		var value float64
		if json.Unmarshal(wire.Confidence, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return Result{}, ErrInvalidResponse
		}
		confidence = &value
	}
	return Result{Engine: wire.Engine, EngineVersion: wire.EngineVersion, PolicyVersion: wire.PolicyVersion, Language: wire.Language, TextRaw: *wire.TextRaw, TextNormalized: *wire.TextNormalized, Confidence: confidence, PSM: wire.PSM, Preprocessing: wire.Preprocessing}, nil
}

// Reject repeated top-level keys instead of accepting last-value-wins metadata.
func uniqueResponseFields(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}')
}
