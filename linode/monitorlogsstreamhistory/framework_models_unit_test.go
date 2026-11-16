//go:build unit

package monitorlogsstreamhistory

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/linode/linodego/v2"
	"github.com/linode/terraform-provider-linode/v4/linode/monitorlogsstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamHistoryCategories(t *testing.T) {
	ctx := context.Background()
	m := DataSourceModel{StreamID: types.Int64Value(456)}
	var diags diag.Diagnostics
	m.ParseStreamHistory(ctx, []linodego.Stream{
		{
			ID: 456, Type: linodego.StreamTypeAuditLogs, Version: 1,
			Details: &linodego.StreamDetails{Categories: []string{"config", "login"}},
		},
		{
			ID: 456, Type: linodego.StreamTypeAuditLogs, Version: 2,
			Details: &linodego.StreamDetails{Categories: []string{"data_plane"}},
		},
		{
			ID: 789, Type: linodego.StreamTypeLKEAuditLogs,
			Details: &linodego.StreamDetails{ClusterIDs: []int{123}},
		},
		{ID: 456, Type: linodego.StreamTypeAuditLogs},
	}, &diags)
	require.False(t, diags.HasError(), "%v", diags)
	require.Len(t, m.Streams, 4)
	assert.Equal(t, "456", m.ID.ValueString())
	for i, expected := range [][]string{{"config", "login"}, {"data_plane"}} {
		var details monitorlogsstream.StreamDetailsModel
		diags.Append(m.Streams[i].Details.As(ctx, &details, basetypes.ObjectAsOptions{})...)
		require.False(t, diags.HasError(), "%v", diags)
		values := make([]attr.Value, len(expected))
		for j, category := range expected {
			values[j] = types.StringValue(category)
		}
		assert.True(t, details.Categories.Equal(types.SetValueMust(types.StringType, values)))
		assert.True(t, details.ClusterIDs.IsNull())
		assert.True(t, details.IsAutoAddAllClustersEnabled.IsNull())
	}
	var lke monitorlogsstream.StreamDetailsModel
	diags.Append(m.Streams[2].Details.As(ctx, &lke, basetypes.ObjectAsOptions{})...)
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, lke.Categories.IsNull())
	assert.True(t, lke.ClusterIDs.Equal(types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(123)})))
	assert.True(t, m.Streams[3].Details.IsNull())
}
