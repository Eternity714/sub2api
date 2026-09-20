package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrsaiIsNotOpenAICompatible(t *testing.T) {
	account := &Account{Platform: PlatformGrsai, Type: AccountTypeAPIKey}

	require.False(t, account.IsOpenAICompatible())
	require.False(t, account.IsCNProvider())
	require.False(t, account.IsOpenCodeGo())
}
