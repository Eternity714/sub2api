package service

import (
	"bytes"
	"encoding/json"
	"time"
)

type GrsaiTaskView struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	Progress      int             `json:"progress"`
	Model         string          `json:"model"`
	CreatedAt     time.Time       `json:"created_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	LinkExpiresAt *time.Time      `json:"link_expires_at"`
	LinkExpired   bool            `json:"link_expired"`
}

func NewGrsaiTaskView(record *GrsaiSettlement, now time.Time) GrsaiTaskView {
	if record == nil {
		return GrsaiTaskView{}
	}
	view := GrsaiTaskView{Status: record.PublicStatus, Progress: record.Progress,
		Model: record.Model, CreatedAt: record.CreatedAt, CompletedAt: record.ClosedAt,
		LinkExpiresAt: record.LinkExpiresAt}
	if record.LocalTaskID != nil {
		view.ID = *record.LocalTaskID
	}
	if view.Status == "" {
		view.Status = "queued"
	}
	if view.Status == "succeeded" && json.Valid(record.ResultJSON) {
		view.Result = bytes.Clone(record.ResultJSON)
	}
	if (view.Status == "failed" || view.Status == "manual_review") && record.LastErrorSummary != nil {
		view.Error = *record.LastErrorSummary
	}
	view.LinkExpired = view.LinkExpiresAt != nil && !now.Before(*view.LinkExpiresAt)
	return view
}
