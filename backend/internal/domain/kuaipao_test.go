package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKuaipaoIsIndependentMediaPlatform(t *testing.T) {
	spec, ok := LookupPlatform("kuaipao")
	require.True(t, ok)
	require.Equal(t, "快跑 API", spec.DisplayName)
	require.Equal(t, PlatformGateway("kuaipao"), spec.Gateway)
	require.True(t, IsCompositeTargetPlatform("kuaipao"))
	require.True(t, IsGroupPlatform("kuaipao"))
	require.False(t, UsesOpenAIGateway("kuaipao"))
	require.False(t, IsQuotaPlatform("kuaipao"))
}
