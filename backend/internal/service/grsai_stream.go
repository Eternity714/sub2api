package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const grsaiSSEMaxLineSize = 4 << 20

var (
	ErrGrsaiSSEProtocol                  = errors.New("invalid grsai SSE protocol")
	ErrGrsaiSSERead                      = errors.New("grsai SSE stream read failed")
	ErrGrsaiStreamPersistenceUnavailable = errors.New("grsai stream persistence is unavailable")
)

// GrsaiStreamEvent is one validated provider SSE data frame. RawData is a
// private snapshot and can safely be forwarded after persistence.
type GrsaiStreamEvent struct {
	RawData    []byte
	TaskID     string
	Status     string
	Progress   int
	ResultURLs []string
	Terminal   bool
}

// ParseGrsaiSSE validates the provider stream and returns its terminal result.
// The callback is invoked only after a complete frame has passed all protocol
// checks. Persistence ordering is owned by the callback caller.
func ParseGrsaiSSE(r io.Reader, callback func(GrsaiStreamEvent) error) (*GrsaiUpstreamResult, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrGrsaiSSEProtocol)
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), grsaiSSEMaxLineSize)
	var data bytes.Buffer
	var taskID string
	progress := 0
	seen := false
	terminal := false
	var final *GrsaiUpstreamResult
	consume := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := bytes.TrimSpace(data.Bytes())
		data.Reset()
		if len(raw) == 0 {
			return nil
		}
		event, err := decodeGrsaiStreamEvent(raw, taskID, progress, seen)
		if err != nil {
			return err
		}
		if terminal {
			return fmt.Errorf("%w: event received after terminal status", ErrGrsaiSSEProtocol)
		}
		if !seen {
			taskID = event.TaskID
			seen = true
		}
		progress = event.Progress
		terminal = event.Terminal
		event.RawData = append([]byte(nil), raw...)
		if callback != nil {
			if err := callback(event); err != nil {
				return err
			}
		}
		final = &GrsaiUpstreamResult{HTTPStatus: 200, RawBody: append([]byte(nil), raw...), TaskID: event.TaskID, Status: event.Status, Progress: event.Progress, ResultURLs: append([]string(nil), event.ResultURLs...)}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := consume(); err != nil {
				return final, err
			}
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return final, fmt.Errorf("%w: %v", ErrGrsaiSSERead, err)
	}
	if err := consume(); err != nil {
		return final, err
	}
	if !seen {
		return nil, fmt.Errorf("%w: stream contained no data event", ErrGrsaiSSEProtocol)
	}
	if !terminal || final == nil {
		return final, fmt.Errorf("%w: stream ended before terminal status", ErrGrsaiSSEProtocol)
	}
	return final, nil
}

func decodeGrsaiStreamEvent(raw []byte, previousID string, previousProgress int, seen bool) (GrsaiStreamEvent, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: data is not a JSON object", ErrGrsaiSSEProtocol)
	}
	dataObject := rawJSONObject(root["data"])
	id := strings.TrimSpace(firstRawScalar(root, dataObject, "id", "taskId", "task_id"))
	if !seen && id == "" {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: first event must contain a task ID", ErrGrsaiSSEProtocol)
	}
	if seen && id == "" {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: every event must contain the stable task ID", ErrGrsaiSSEProtocol)
	}
	if seen && id != previousID {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: task ID changed", ErrGrsaiSSEProtocol)
	}
	if id == "" {
		id = previousID
	}
	status := normalizeGrsaiStatus(firstRawScalar(root, dataObject, "status", "state"))
	if status == "" {
		status = GrsaiUpstreamStatusRunning
	}
	switch status {
	case GrsaiUpstreamStatusRunning, GrsaiUpstreamStatusSucceeded, GrsaiUpstreamStatusFailed, GrsaiUpstreamStatusViolation:
	default:
		return GrsaiStreamEvent{}, fmt.Errorf("%w: unsupported status %q", ErrGrsaiSSEProtocol, status)
	}
	progress := previousProgress
	if rawProgress := firstRawMessage(root, dataObject, "progress", "percent", "percentage"); len(rawProgress) > 0 {
		parsed, err := parseGrsaiProgress(rawProgress)
		if err != nil {
			return GrsaiStreamEvent{}, err
		}
		progress = parsed
	}
	if progress < 0 || progress > 100 || (seen && progress < previousProgress) {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: progress must be monotonic within 0..100", ErrGrsaiSSEProtocol)
	}
	urls := extractGrsaiResultURLs(root, dataObject)
	terminal := status == GrsaiUpstreamStatusSucceeded || status == GrsaiUpstreamStatusFailed || status == GrsaiUpstreamStatusViolation
	if status == GrsaiUpstreamStatusSucceeded && len(urls) == 0 {
		return GrsaiStreamEvent{}, fmt.Errorf("%w: succeeded event must contain a result URL", ErrGrsaiSSEProtocol)
	}
	return GrsaiStreamEvent{TaskID: id, Status: status, Progress: progress, ResultURLs: urls, Terminal: terminal}, nil
}

func firstRawMessage(primary, secondary map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, object := range []map[string]json.RawMessage{primary, secondary} {
		for _, key := range keys {
			if value := object[key]; len(value) > 0 {
				return value
			}
		}
	}
	return nil
}

func parseGrsaiProgress(raw json.RawMessage) (int, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, fmt.Errorf("%w: empty progress", ErrGrsaiSSEProtocol)
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed, nil
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil || number != float64(int(number)) {
		return 0, fmt.Errorf("%w: progress must be an integer", ErrGrsaiSSEProtocol)
	}
	return int(number), nil
}

func extractGrsaiResultURLs(primary, secondary map[string]json.RawMessage) []string {
	for _, key := range []string{"results", "result", "result_urls", "resultURLs", "images", "urls"} {
		for _, object := range []map[string]json.RawMessage{primary, secondary} {
			if raw, ok := object[key]; ok {
				if urls := parseGrsaiResultURLs(raw); len(urls) > 0 {
					return urls
				}
			}
		}
	}
	return nil
}

func parseGrsaiResultURLs(raw json.RawMessage) []string {
	var stringsOnly []string
	if json.Unmarshal(raw, &stringsOnly) == nil {
		return cleanGrsaiResultURLs(stringsOnly)
	}
	var entries []map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return nil
	}
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		url := strings.TrimSpace(firstRawScalar(entry, nil, "url", "uri", "href"))
		if url != "" {
			urls = append(urls, url)
		}
	}
	return cleanGrsaiResultURLs(urls)
}

func cleanGrsaiResultURLs(urls []string) []string {
	clean := make([]string, 0, len(urls))
	for _, url := range urls {
		if value := strings.TrimSpace(url); value != "" {
			clean = append(clean, value)
		}
	}
	return clean
}

// ConsumeGrsaiSSE persists each validated event before notifying the caller.
// An interruption before binding releases the hold via manual review; after
// binding it records an unknown result and leaves the hold for polling.
func (s *GrsaiSettlementService) ConsumeGrsaiSSE(ctx context.Context, claim *GrsaiSettlement, reader io.Reader, onPersistedEvent func(GrsaiStreamEvent) error) (*GrsaiUpstreamResult, error) {
	if s == nil || claim == nil || reader == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	final, err := ParseGrsaiSSE(reader, func(event GrsaiStreamEvent) error {
		if persistErr := s.RecordStreamEvent(ctx, claim, event); persistErr != nil {
			return persistErr
		}
		if onPersistedEvent != nil {
			if callbackErr := onPersistedEvent(event); callbackErr != nil {
				return callbackErr
			}
		}
		return nil
	})
	if err == nil {
		return final, nil
	}
	// Only an I/O interruption leaves the provider outcome uncertain. Protocol,
	// persistence and downstream callback failures are local failures and must
	// not mutate the durable settlement state behind the caller's back.
	if !errors.Is(err, ErrGrsaiSSERead) {
		if claim.UpstreamTaskID == nil || strings.TrimSpace(*claim.UpstreamTaskID) == "" {
			return final, errors.Join(err, s.markManualReviewWithRelease(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, "upstream stream protocol, persistence, or callback failure before task ID binding"))
		}
		retryAt := time.Now().Add(grsaiSettlementRetryDelay)
		_, updateErr := s.Repo.UpdateResult(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, "unknown", "upstream stream protocol, persistence, or callback failure after task ID binding", retryAt)
		pendingErr := s.Repo.MarkPendingUpstream(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, retryAt)
		return final, errors.Join(err, updateErr, pendingErr)
	}
	retryAt := time.Now().Add(grsaiSettlementRetryDelay)
	firstEvent := claim.UpstreamTaskID == nil || strings.TrimSpace(*claim.UpstreamTaskID) == ""
	if firstEvent {
		return final, errors.Join(err, s.markManualReviewWithRelease(ctx, claim.ID, claim.ClaimVersion, "upstream stream interrupted before task ID binding"))
	}
	summary := "upstream stream interrupted after task ID binding; result polling required"
	_, updateErr := s.Repo.UpdateResult(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, "unknown", summary, retryAt)
	pendingErr := s.Repo.MarkPendingUpstream(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, retryAt)
	return final, errors.Join(err, updateErr, pendingErr)
}

// GrsaiStreamEventRepository is an optional extension to the settlement
// repository. Keeping it separate preserves existing recovery mocks.
type GrsaiStreamEventRepository interface {
	RecordStreamEvent(context.Context, int64, int64, GrsaiStreamEvent) (bool, error)
}

// GrsaiStreamBindRepository atomically binds the first provider ID and stores
// its event. Implementations must fence on the settlement claim so a failed
// event write cannot leave an apparently-bound task behind.
type GrsaiStreamBindRepository interface {
	BindAndRecordStreamEvent(context.Context, int64, int64, GrsaiStreamEvent) (bool, error)
}

func (s *GrsaiSettlementService) RecordStreamEvent(ctx context.Context, claim *GrsaiSettlement, event GrsaiStreamEvent) error {
	if s == nil || s.Repo == nil || claim == nil || event.TaskID == "" {
		return ErrGrsaiSettlementInvalidInput
	}
	firstEvent := claim.UpstreamTaskID == nil || strings.TrimSpace(*claim.UpstreamTaskID) == ""
	if firstEvent {
		binder, ok := s.Repo.(GrsaiStreamBindRepository)
		if !ok {
			return ErrGrsaiStreamPersistenceUnavailable
		}
		bound, err := binder.BindAndRecordStreamEvent(ctx, claim.ID, claim.ClaimVersion, event)
		if err != nil {
			return err
		}
		if !bound {
			return ErrGrsaiSettlementClaimLost
		}
		id := event.TaskID
		claim.UpstreamTaskID = &id
	} else if strings.TrimSpace(*claim.UpstreamTaskID) != event.TaskID {
		return fmt.Errorf("%w: task ID changed", ErrGrsaiSSEProtocol)
	}
	if firstEvent {
		claim.UpstreamStatus = event.Status
		claim.Progress = event.Progress
		claim.ResultURLs = append([]string(nil), event.ResultURLs...)
		return nil
	}
	recorder, ok := s.Repo.(GrsaiStreamEventRepository)
	if !ok {
		return ErrGrsaiStreamPersistenceUnavailable
	}
	updated, err := recorder.RecordStreamEvent(ctx, claim.ID, claim.ClaimVersion, event)
	if err != nil {
		return err
	}
	if !updated {
		return ErrGrsaiSettlementClaimLost
	}
	claim.UpstreamStatus = event.Status
	claim.Progress = event.Progress
	claim.ResultURLs = append([]string(nil), event.ResultURLs...)
	return nil
}
