package schema

import (
	"testing"

	"entgo.io/ent"
	"github.com/stretchr/testify/require"
)

func TestGrsaiSettlementProgressRejectsValuesOutsidePercentRange(t *testing.T) {
	validator := requireIntFieldValidator(t, GrsaiSettlement{}.Fields(), "progress")
	require.NoError(t, validator(0))
	require.NoError(t, validator(100))
	require.Error(t, validator(-1))
	require.Error(t, validator(101))
}

func requireIntFieldValidator(t *testing.T, fields []ent.Field, name string) func(int) error {
	t.Helper()

	for _, entField := range fields {
		descriptor := entField.Descriptor()
		if descriptor.Name != name {
			continue
		}
		require.NotEmpty(t, descriptor.Validators, "field %s should include a validator", name)
		validators := make([]func(int) error, 0, len(descriptor.Validators))
		for _, rawValidator := range descriptor.Validators {
			validator, ok := rawValidator.(func(int) error)
			require.True(t, ok, "field %s validator should be func(int) error", name)
			validators = append(validators, validator)
		}
		return func(value int) error {
			for _, validator := range validators {
				if err := validator(value); err != nil {
					return err
				}
			}
			return nil
		}
	}

	require.Failf(t, "missing field validator", "schema should include field %s", name)
	return nil
}
