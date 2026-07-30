package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenCLIBridgeClient talks only to the minimal host-side OpenCLI Bridge.
// It intentionally does not expose browser-profile or provider-management APIs.
type OpenCLIBridgeClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewOpenCLIBridgeClient(baseURL, token string, timeout time.Duration) *OpenCLIBridgeClient {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &OpenCLIBridgeClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token),
		Client:  &http.Client{Timeout: timeout},
	}
}

func (c *OpenCLIBridgeClient) authorize(req *http.Request) {
	if c != nil && strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
}

func (c *OpenCLIBridgeClient) CollectWithTimeout(
	ctx context.Context,
	source, rawURL string,
	options map[string]any,
	timeout time.Duration,
) (*CollectOutcome, error) {
	if c == nil || c.Client == nil || strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("opencli bridge client unavailable")
	}
	body := map[string]any{
		"source": strings.TrimSpace(source),
		"url":    strings.TrimSpace(rawURL),
	}
	if len(options) > 0 {
		body["options"] = options
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/collect", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)

	httpClient := c.Client
	if timeout > 0 {
		httpClient = &http.Client{Timeout: timeout}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencli bridge request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("opencli bridge read body: %w", err)
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("opencli bridge invalid json (http %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK || env.Error != nil {
		rejected := &CollectorRejectedError{
			Code:    "OPENCLI_BRIDGE_ERROR",
			Message: fmt.Sprintf("opencli bridge http %d", resp.StatusCode),
		}
		if env.Error != nil {
			if strings.TrimSpace(env.Error.Code) != "" {
				rejected.Code = strings.TrimSpace(env.Error.Code)
			}
			if strings.TrimSpace(env.Error.Message) != "" {
				rejected.Message = strings.TrimSpace(env.Error.Message)
			}
		}
		if len(env.Data) > 0 {
			rejected.AccessReport = env.Data
		}
		return nil, rejected
	}
	var wrap collectDataProduct
	if err := json.Unmarshal(env.Data, &wrap); err != nil {
		return nil, fmt.Errorf("opencli bridge parse data: %w", err)
	}
	if len(wrap.Product) == 0 {
		return nil, fmt.Errorf("opencli bridge returned empty product")
	}
	return &CollectOutcome{ProductJSON: wrap.Product}, nil
}

// OpenCLIBridgeRuntimeStatus is the sanitized status returned by the bridge.
type OpenCLIBridgeRuntimeStatus struct {
	Ready              bool   `json:"ready"`
	BinaryAvailable    bool   `json:"binaryAvailable"`
	DaemonRunning      bool   `json:"daemonRunning"`
	ExtensionConnected bool   `json:"extensionConnected"`
	ProfileAvailable   bool   `json:"profileAvailable"`
	Message            string `json:"message"`
}

func (c *OpenCLIBridgeClient) ProbeStatus(parent context.Context) (*OpenCLIBridgeRuntimeStatus, error) {
	if c == nil || c.Client == nil || strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("opencli bridge client unavailable")
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/opencli/status", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencli bridge status: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("opencli bridge status invalid json: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		message := fmt.Sprintf("opencli bridge http %d", resp.StatusCode)
		if env.Error != nil && strings.TrimSpace(env.Error.Message) != "" {
			message = strings.TrimSpace(env.Error.Message)
		}
		return nil, fmt.Errorf("%s", message)
	}
	var status OpenCLIBridgeRuntimeStatus
	if err := json.Unmarshal(env.Data, &status); err != nil {
		return nil, fmt.Errorf("opencli bridge parse status: %w", err)
	}
	return &status, nil
}
