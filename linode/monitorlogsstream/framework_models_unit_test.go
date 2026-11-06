//go:build unit

package monitorlogsstream_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/linode/linodego/v2"
	"github.com/linode/terraform-provider-linode/v4/linode/monitorlogsstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTime(s string) *time.Time {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		panic(err)
	}
	return &t
}

// streamDetailsAttrTypes derives the details nested object attribute types from the exported schema.
func streamDetailsAttrTypes() map[string]attr.Type {
	detailsAttr := monitorlogsstream.ResourceSchema.Attributes["details"].(schema.SingleNestedAttribute)
	return detailsAttr.GetType().(types.ObjectType).AttrTypes
}

func TestFlattenStream_AuditLogs(t *testing.T) {
	ctx := context.Background()

	destID := 12345
	stream := &linodego.Stream{
		ID:      456,
		Label:   "AuditLog-config",
		Type:    linodego.StreamTypeAuditLogs,
		Status:  linodego.StreamStatusActive,
		Version: 1,
		Destinations: []linodego.StreamDestination{
			{ID: destID, Label: "OBJ_logs_destination", Type: linodego.StreamDestinationTypeAkamaiObjectStorage},
		},
		Details:   nil,
		Created:   makeTime("2025-03-20T01:41:09"),
		Updated:   makeTime("2025-03-20T01:41:09"),
		CreatedBy: "John Q. Linode",
		UpdatedBy: "Jane Q. Linode",
	}

	var m monitorlogsstream.ResourceModel
	var diags diag.Diagnostics

	m.FlattenStream(ctx, stream, false, &diags)

	require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)

	assert.Equal(t, "456", m.ID.ValueString())
	assert.Equal(t, "AuditLog-config", m.Label.ValueString())
	assert.Equal(t, "audit_logs", m.Type.ValueString())
	assert.Equal(t, "active", m.Status.ValueString())
	assert.Equal(t, int64(1), m.Version.ValueInt64())
	assert.Equal(t, "John Q. Linode", m.CreatedBy.ValueString())
	assert.Equal(t, "Jane Q. Linode", m.UpdatedBy.ValueString())
	assert.False(t, m.Created.IsNull())
	assert.False(t, m.Updated.IsNull())

	// details should be null for audit_logs
	assert.True(t, m.Details.IsNull(), "details should be null for audit_logs stream")
	_ = streamDetailsAttrTypes() // ensure schema is accessible

	// destinations should contain the destination ID
	var destIDs []int64
	require.False(t, m.Destinations.IsNull())
	diags = m.Destinations.ElementsAs(ctx, &destIDs, false)
	require.False(t, diags.HasError())
	assert.Equal(t, []int64{int64(destID)}, destIDs)
}

func TestFlattenStream_LKEAuditLogs(t *testing.T) {
	ctx := context.Background()

	stream := &linodego.Stream{
		ID:      789,
		Label:   "LKEAuditLog-config",
		Type:    linodego.StreamTypeLKEAuditLogs,
		Status:  linodego.StreamStatusActive,
		Version: 1,
		Destinations: []linodego.StreamDestination{
			{ID: 12345, Label: "OBJ_logs_destination", Type: linodego.StreamDestinationTypeAkamaiObjectStorage},
		},
		Details: &linodego.StreamDetails{
			ClusterIDs:                  []int{1234, 5678},
			IsAutoAddAllClustersEnabled: false,
		},
		Created:   makeTime("2025-03-20T01:41:09"),
		Updated:   makeTime("2025-03-20T01:41:09"),
		CreatedBy: "John Q. Linode",
		UpdatedBy: "Jane Q. Linode",
	}

	var m monitorlogsstream.ResourceModel
	var diags diag.Diagnostics

	m.FlattenStream(ctx, stream, false, &diags)

	require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)

	assert.Equal(t, "789", m.ID.ValueString())
	assert.Equal(t, "lke_audit_logs", m.Type.ValueString())
	assert.False(t, m.Details.IsNull(), "details should be set for lke_audit_logs stream")

	var detailsModel monitorlogsstream.StreamDetailsModel
	diags = m.Details.As(ctx, &detailsModel, basetypes.ObjectAsOptions{})
	require.False(t, diags.HasError())

	assert.False(t, detailsModel.IsAutoAddAllClustersEnabled.ValueBool())

	var clusterIDs []int64
	diags = detailsModel.ClusterIDs.ElementsAs(ctx, &clusterIDs, false)
	require.False(t, diags.HasError())
	assert.Equal(t, []int64{1234, 5678}, clusterIDs)
}

func TestFlattenStream_PreserveKnown_UnknownDetails(t *testing.T) {
	ctx := context.Background()

	stream := &linodego.Stream{
		ID:    456,
		Label: "AuditLog-config",
		Type:  linodego.StreamTypeAuditLogs,
		Destinations: []linodego.StreamDestination{
			{ID: 12345},
		},
		Details:   nil,
		Created:   makeTime("2025-03-20T01:41:09"),
		Updated:   makeTime("2025-03-20T01:41:09"),
		CreatedBy: "user",
		UpdatedBy: "user",
	}

	m := monitorlogsstream.ResourceModel{
		Details: types.ObjectUnknown(streamDetailsAttrTypes()),
	}
	var diags diag.Diagnostics

	m.FlattenStream(ctx, stream, true, &diags)
	require.False(t, diags.HasError())
	assert.True(t, m.Details.IsNull(), "nil API details must become null after create, not unknown")
}

func TestFlattenStream_PreserveKnown(t *testing.T) {
	ctx := context.Background()

	stream := &linodego.Stream{
		ID:    456,
		Label: "AuditLog-updated",
		Type:  linodego.StreamTypeAuditLogs,
		Destinations: []linodego.StreamDestination{
			{ID: 99999},
		},
		Created:   makeTime("2025-03-20T01:41:09"),
		Updated:   makeTime("2025-03-20T01:41:09"),
		CreatedBy: "user",
		UpdatedBy: "user",
	}

	// Pre-populate with known values that should be preserved
	m := monitorlogsstream.ResourceModel{
		ID:      types.StringValue("456"),
		Label:   types.StringValue("AuditLog-original"),
		Details: types.ObjectNull(streamDetailsAttrTypes()),
	}
	var diags diag.Diagnostics

	m.FlattenStream(ctx, stream, true, &diags)
	require.False(t, diags.HasError())

	// Known values should be preserved when preserveKnown = true
	assert.Equal(t, "AuditLog-original", m.Label.ValueString(), "known label should be preserved")
	assert.Equal(t, "456", m.ID.ValueString(), "known ID should be preserved")
}

func TestFlattenStream_NotPreserveKnown(t *testing.T) {
	ctx := context.Background()

	stream := &linodego.Stream{
		ID:    456,
		Label: "AuditLog-updated",
		Type:  linodego.StreamTypeAuditLogs,
		Destinations: []linodego.StreamDestination{
			{ID: 99999},
		},
		Created:   makeTime("2025-03-20T01:41:09"),
		Updated:   makeTime("2025-03-20T01:41:09"),
		CreatedBy: "user",
		UpdatedBy: "user",
	}

	m := monitorlogsstream.ResourceModel{
		ID:      types.StringValue("456"),
		Label:   types.StringValue("AuditLog-original"),
		Details: types.ObjectNull(streamDetailsAttrTypes()),
	}
	var diags diag.Diagnostics

	m.FlattenStream(ctx, stream, false, &diags)
	require.False(t, diags.HasError())

	// Values should be updated when preserveKnown = false
	assert.Equal(t, "AuditLog-updated", m.Label.ValueString(), "label should be updated")
}

func TestGetCreateOptions_AuditLogs(t *testing.T) {
	ctx := context.Background()

	destList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{12345})

	m := monitorlogsstream.ResourceModel{
		Label:        types.StringValue("AuditLog-config"),
		Type:         types.StringValue("audit_logs"),
		Destinations: destList,
		Details:      types.ObjectNull(streamDetailsAttrTypes()),
	}

	var diags diag.Diagnostics
	opts := m.GetCreateOptions(ctx, &diags)
	require.False(t, diags.HasError())

	assert.Equal(t, "AuditLog-config", opts.Label)
	assert.Equal(t, linodego.StreamTypeAuditLogs, opts.Type)
	assert.Equal(t, []int{12345}, opts.Destinations)
	assert.Nil(t, opts.Details)
	assert.Nil(t, opts.Status)
}

func TestGetCreateOptions_LKEAuditLogs(t *testing.T) {
	ctx := context.Background()

	destList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{12345})
	clusterList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{1111, 2222})

	detailsObj, _ := types.ObjectValueFrom(ctx, streamDetailsAttrTypes(), monitorlogsstream.StreamDetailsModel{
		ClusterIDs:                  clusterList,
		IsAutoAddAllClustersEnabled: types.BoolValue(false),
		VPNGatewayIDs:               types.SetNull(types.Int64Type),
	})

	m := monitorlogsstream.ResourceModel{
		Label:        types.StringValue("LKEAuditLog-config"),
		Type:         types.StringValue("lke_audit_logs"),
		Destinations: destList,
		Details:      detailsObj,
	}

	var diags diag.Diagnostics
	opts := m.GetCreateOptions(ctx, &diags)
	require.False(t, diags.HasError())

	assert.Equal(t, "LKEAuditLog-config", opts.Label)
	assert.Equal(t, linodego.StreamTypeLKEAuditLogs, opts.Type)
	require.NotNil(t, opts.Details)
	assert.Equal(t, []int{1111, 2222}, opts.Details.ClusterIDs)
	assert.False(t, opts.Details.IsAutoAddAllClustersEnabled)
}

func TestGetCreateOptions_WithStatus(t *testing.T) {
	ctx := context.Background()

	destList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{12345})

	m := monitorlogsstream.ResourceModel{
		Label:        types.StringValue("AuditLog-config"),
		Type:         types.StringValue("audit_logs"),
		Status:       types.StringValue("inactive"),
		Destinations: destList,
		Details:      types.ObjectNull(streamDetailsAttrTypes()),
	}

	var diags diag.Diagnostics
	opts := m.GetCreateOptions(ctx, &diags)
	require.False(t, diags.HasError())

	require.NotNil(t, opts.Status)
	assert.Equal(t, linodego.StreamStatusInactive, *opts.Status)
}

func TestGetUpdateOptions(t *testing.T) {
	ctx := context.Background()

	destList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{12345})

	m := monitorlogsstream.ResourceModel{
		Label:        types.StringValue("AuditLog-renamed"),
		Type:         types.StringValue("audit_logs"),
		Status:       types.StringValue("active"),
		Destinations: destList,
		Details:      types.ObjectNull(streamDetailsAttrTypes()),
	}

	var diags diag.Diagnostics
	opts := m.GetUpdateOptions(ctx, &diags)
	require.False(t, diags.HasError())

	require.NotNil(t, opts.Label)
	assert.Equal(t, "AuditLog-renamed", *opts.Label)
	assert.Equal(t, []int{12345}, opts.Destinations)
	require.NotNil(t, opts.Status)
	assert.Equal(t, linodego.StreamStatusActive, *opts.Status)
}

func TestVPNGatewayStreamOptionsAndState(t *testing.T) {
	ctx := context.Background()
	for _, streamType := range []linodego.StreamType{
		linodego.StreamTypeVPNGWIPSecLogs, linodego.StreamTypeVPNGWBGPLogs,
	} {
		t.Run(string(streamType), func(t *testing.T) {
			gateways := types.SetValueMust(types.Int64Type, []attr.Value{
				types.Int64Value(50), types.Int64Value(60),
			})
			details, d := types.ObjectValueFrom(ctx, streamDetailsAttrTypes(), monitorlogsstream.StreamDetailsModel{
				ClusterIDs:                  types.ListNull(types.Int64Type),
				IsAutoAddAllClustersEnabled: types.BoolNull(),
				VPNGatewayIDs:               gateways,
			})
			require.False(t, d.HasError(), "%v", d)
			m := monitorlogsstream.ResourceModel{
				Label:        types.StringValue("VPN logs"),
				Type:         types.StringValue(string(streamType)),
				Destinations: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
				Details:      details,
			}
			var diags diag.Diagnostics
			create := m.GetCreateOptions(ctx, &diags)
			update := m.GetUpdateOptions(ctx, &diags)
			require.False(t, diags.HasError(), "%v", diags)
			for _, opts := range []any{create, update} {
				body, err := json.Marshal(opts)
				require.NoError(t, err)
				var payload map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(body, &payload))
				assert.JSONEq(t, `{"vpn_gateway_ids":[50,60]}`, string(payload["details"]))
			}

			stream := &linodego.Stream{
				ID: 5, Type: streamType, Label: "VPN logs",
				Details: &linodego.StreamDetails{VPNGatewayIDs: []int{60, 50}},
			}
			for _, preserve := range []bool{false, true} {
				m.FlattenStream(ctx, stream, preserve, &diags)
				require.False(t, diags.HasError(), "%v", diags)
				assert.True(t, details.Equal(m.Details), "gateway ordering must not change state")
			}
			var data monitorlogsstream.DataSourceModel
			data.FlattenStream(ctx, stream, &diags)
			require.False(t, diags.HasError(), "%v", diags)
			assert.True(t, details.Equal(data.Details))
		})
	}
}

func TestValidateVPNDetails(t *testing.T) {
	ctx := context.Background()
	gateways := types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(50)})
	tests := []struct {
		name       string
		streamType string
		details    monitorlogsstream.StreamDetailsModel
		wantError  bool
	}{
		{name: "valid VPN", streamType: "vpngw_ipsec_logs", details: monitorlogsstream.StreamDetailsModel{VPNGatewayIDs: gateways}},
		{name: "missing IDs", streamType: "vpngw_bgp_logs", wantError: true},
		{name: "empty IDs", streamType: "vpngw_bgp_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: types.SetValueMust(types.Int64Type, []attr.Value{}),
		}, wantError: true},
		{name: "LKE IDs on VPN", streamType: "vpngw_ipsec_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: gateways, ClusterIDs: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(50)}),
		}, wantError: true},
		{name: "explicit false LKE setting on VPN", streamType: "vpngw_ipsec_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: gateways, IsAutoAddAllClustersEnabled: types.BoolValue(false),
		}, wantError: true},
		{name: "VPN IDs on LKE", streamType: "lke_audit_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: gateways,
		}, wantError: true},
		{name: "VPN IDs on audit", streamType: "audit_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: gateways,
		}, wantError: true},
		{name: "unknown IDs deferred", streamType: "vpngw_bgp_logs", details: monitorlogsstream.StreamDetailsModel{
			VPNGatewayIDs: types.SetUnknown(types.Int64Type),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.details.ClusterIDs.IsNull() {
				tt.details.ClusterIDs = types.ListNull(types.Int64Type)
			}
			if tt.details.VPNGatewayIDs.IsNull() {
				tt.details.VPNGatewayIDs = types.SetNull(types.Int64Type)
			}
			details, d := types.ObjectValueFrom(ctx, streamDetailsAttrTypes(), tt.details)
			require.False(t, d.HasError(), "%v", d)
			m := monitorlogsstream.ResourceModel{Type: types.StringValue(tt.streamType), Details: details}
			var diags diag.Diagnostics
			m.ValidateVPNDetails(ctx, &diags)
			assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
		})
	}
	t.Run("missing details", func(t *testing.T) {
		m := monitorlogsstream.ResourceModel{
			Type:    types.StringValue("vpngw_ipsec_logs"),
			Details: types.ObjectNull(streamDetailsAttrTypes()),
		}
		var diags diag.Diagnostics
		m.ValidateVPNDetails(ctx, &diags)
		assert.True(t, diags.HasError())
	})
}

func TestVPNGatewayIDsValidators(t *testing.T) {
	ctx := context.Background()
	details := monitorlogsstream.ResourceSchema.Attributes["details"].(schema.SingleNestedAttribute)
	gateways := details.Attributes["vpn_gateway_ids"].(schema.SetAttribute)
	for _, tt := range []struct {
		name      string
		value     types.Set
		wantError bool
	}{
		{name: "positive", value: types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(50)})},
		{name: "empty", value: types.SetValueMust(types.Int64Type, []attr.Value{}), wantError: true},
		{name: "zero", value: types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(0)}), wantError: true},
		{name: "negative", value: types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(-1)}), wantError: true},
		{name: "unknown", value: types.SetUnknown(types.Int64Type)},
		{name: "null", value: types.SetNull(types.Int64Type)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			for _, v := range gateways.Validators {
				var resp validator.SetResponse
				v.ValidateSet(ctx, validator.SetRequest{ConfigValue: tt.value}, &resp)
				diags.Append(resp.Diagnostics...)
			}
			assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
		})
	}
}
