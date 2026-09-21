package migrate

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/stretchr/testify/require"
)

func TestGrsaiTaskPayloadForeignKeyCascadesWithSettlement(t *testing.T) {
	fk := findForeignKeyByColumn(t, GrsaiTaskPayloadsTable, "settlement_id")
	require.Len(t, fk.Columns, 1)
	require.False(t, fk.Columns[0].Nullable)
	require.Len(t, fk.RefColumns, 1)
	require.Equal(t, "id", fk.RefColumns[0].Name)
	require.Same(t, GrsaiSettlementsTable, fk.RefTable)
	require.Equal(t, entschema.Cascade, fk.OnDelete)
	require.Contains(t, GrsaiTaskPayloadsTable.PrimaryKey, fk.Columns[0])
}
