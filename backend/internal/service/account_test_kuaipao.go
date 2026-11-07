package service

import (
	"fmt"
	"maps"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/gin-gonic/gin"
)

func (s *AccountTestService) testKuaipaoAccountConnection(c *gin.Context, account *Account, modelID, prompt string) error {
	if account.Type != AccountTypeAPIKey || strings.TrimSpace(account.GetCredential("api_key")) == "" {
		return s.sendErrorAndEnd(c, "Kuaipao requires an API key account")
	}
	if strings.TrimSpace(modelID) == "" {
		modelID = KuaipaoDefaultImageModel
	}
	if !account.IsModelSupported(modelID) {
		return s.sendErrorAndEnd(c, "Model is not supported by this Kuaipao account")
	}
	model := account.GetMappedModel(modelID)
	if !isOpenAIImageModel(model) {
		return s.sendErrorAndEnd(c, "Kuaipao account tests require an image model")
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultOpenAIImageTestPrompt
	}
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if base == "" {
		base = KuaipaoDefaultBaseURL
	}
	base, err := s.validateUpstreamBaseURL(base)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid Kuaipao base URL")
	}
	copy := *account
	copy.Credentials = maps.Clone(account.Credentials)
	copy.Credentials["base_url"] = base
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})
	upstream := NewUnifiedMediaUpstream(nil).(*unifiedMediaHTTPUpstream)
	upstream.transport = s.httpUpstream
	normalized := &mediacapability.NormalizedRequest{Values: map[string]any{"model": modelID, "prompt": prompt, "size": "1:1", "metadata": map[string]any{"resolution": "1K", "quality": "auto"}}, ImageSpec: &mediacapability.ImageSpec{Width: 1024, Height: 1024}, RequestedCount: 1}
	result, err := upstream.Submit(c.Request.Context(), &copy, "image", model, normalized, 1)
	if err != nil || result == nil || result.Disposition != "accepted" || result.Status != "completed" || len(result.URLs) == 0 {
		if result != nil && result.HTTPStatus > 0 {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Kuaipao image test failed (HTTP %d)", result.HTTPStatus))
		}
		return s.sendErrorAndEnd(c, "Kuaipao image test failed; submission outcome may be unknown")
	}
	for _, url := range result.URLs {
		s.sendEvent(c, TestEvent{Type: "image", ImageURL: url})
	}
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
