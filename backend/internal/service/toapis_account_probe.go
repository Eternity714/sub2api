package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// testToAPIsAccountConnection validates credentials through the read-only model
// catalog. It never creates a generation task, even when the UI sends a model
// or prompt intended for other account test protocols.
func (s *AccountTestService) testToAPIsAccountConnection(c *gin.Context, account *Account) error {
	if account.Type != AccountTypeAPIKey {
		return s.sendErrorAndEnd(c, "ToAPIs only supports API key accounts")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return s.sendErrorAndEnd(c, "No API key available")
	}
	configuredBase := strings.TrimSpace(account.GetCredential("base_url"))
	if configuredBase == "" {
		return s.sendErrorAndEnd(c, "ToAPIs base URL must be configured explicitly")
	}
	baseURL, err := s.validateUpstreamBaseURL(configuredBase)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid ToAPIs base URL")
	}
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid ToAPIs base URL")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/")
	if !strings.HasSuffix(endpoint.Path, "/v1") {
		endpoint.Path += "/v1"
	}
	endpoint.Path += "/models"
	endpoint.RawQuery = "type=all"
	endpoint.Fragment = ""
	ctx := c.Request.Context()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create ToAPIs connection test")
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	s.sendEvent(c, TestEvent{Type: "test_start", Model: "ToAPIs model catalog"})
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	response, err := s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return s.sendErrorAndEnd(c, "ToAPIs connection test failed; check endpoint and proxy settings")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		message := fmt.Sprintf("ToAPIs model catalog returned HTTP %d", response.StatusCode)
		if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && s.accountRepo != nil {
			_ = s.accountRepo.SetError(ctx, account.ID, message)
		}
		return s.sendErrorAndEnd(c, message)
	}
	const maximumResponseBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumResponseBytes+1))
	if err != nil || len(body) > maximumResponseBytes {
		return s.sendErrorAndEnd(c, "Invalid ToAPIs model catalog response")
	}
	var catalog struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(body, &catalog); err != nil || len(catalog.Data) == 0 || catalog.Data[0] != '[' {
		return s.sendErrorAndEnd(c, "Invalid ToAPIs model catalog response")
	}
	var models []json.RawMessage
	if err = json.Unmarshal(catalog.Data, &models); err != nil {
		return s.sendErrorAndEnd(c, "Invalid ToAPIs model catalog response")
	}
	s.sendEvent(c, TestEvent{Type: "content", Text: fmt.Sprintf("ToAPIs credentials verified. Model catalog contains %d entries. No generation request was sent.", len(models))})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
