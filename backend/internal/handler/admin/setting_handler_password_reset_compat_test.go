package admin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsPasswordResetLegacyCompatPreservesOmittedAndDisablesExplicitly(t *testing.T) {
	const key = "password_reset_token_legacy_compat"
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		"email_verify_enabled":   "true",
		"password_reset_enabled": "true",
	})

	enabled := doUpdateSettings(t, h, map[string]any{key: true}, nil)
	require.Equal(t, http.StatusOK, enabled.Code)
	require.Equal(t, "true", repo.values[key])
	require.Contains(t, enabled.Body.String(), `"password_reset_token_legacy_compat":true`)
	require.Equal(t, "true", repo.values["email_verify_enabled"])
	require.Equal(t, "true", repo.values["password_reset_enabled"])

	omitted := doUpdateSettings(t, h, map[string]any{"site_name": "changed"}, nil)
	require.Equal(t, http.StatusOK, omitted.Code)
	require.Equal(t, "true", repo.values[key])
	require.Equal(t, "true", repo.values["email_verify_enabled"])
	require.Equal(t, "true", repo.values["password_reset_enabled"])

	disabled := doUpdateSettings(t, h, map[string]any{key: false}, nil)
	require.Equal(t, http.StatusOK, disabled.Code)
	require.Equal(t, "false", repo.values[key])
	require.Contains(t, disabled.Body.String(), `"password_reset_token_legacy_compat":false`)
}
