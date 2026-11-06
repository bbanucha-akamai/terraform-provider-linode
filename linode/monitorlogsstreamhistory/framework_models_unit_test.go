//go:build unit

package monitorlogsstreamhistory

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamHistoryVPNGateway(t *testing.T) {
	ctx := context.Background()
	for _, streamType := range []linodego.StreamType{
		linodego.StreamTypeVPNGWIPSecLogs, linodego.StreamTypeVPNGWBGPLogs,
	} {
		t.Run(string(streamType), func(t *testing.T) {
			m := DataSourceModel{StreamID: types.Int64Value(5)}
			var diags diag.Diagnostics
			m.ParseStreamHistory(ctx, []linodego.Stream{
				{ID: 5, Type: streamType, Version: 1, Details: &linodego.StreamDetails{VPNGatewayIDs: []int{50, 60}}},
				{ID: 5, Type: streamType, Version: 2, Details: &linodego.StreamDetails{VPNGatewayIDs: []int{70}}},
				{ID: 6, Type: linodego.StreamTypeAuditLogs},
				{ID: 7, Type: linodego.StreamTypeLKEAuditLogs, Details: &linodego.StreamDetails{ClusterIDs: []int{10}}},
			}, &diags)
			require.False(t, diags.HasError(), "%v", diags)
			require.Len(t, m.Streams, 4)
			assert.Equal(t, "5", m.ID.ValueString())
			for i, expected := range [][]int64{{50, 60}, {70}} {
				attrs := m.Streams[i].Details.Attributes()
				var ids []int64
				d := attrs["vpn_gateway_ids"].(types.Set).ElementsAs(ctx, &ids, false)
				require.False(t, d.HasError(), "%v", d)
				assert.ElementsMatch(t, expected, ids)
				assert.True(t, attrs["cluster_ids"].IsNull())
				assert.True(t, attrs["is_auto_add_all_clusters_enabled"].IsNull())
			}
			assert.True(t, m.Streams[2].Details.IsNull())
			assert.True(t, m.Streams[3].Details.Attributes()["vpn_gateway_ids"].IsNull())
		})
	}
}
