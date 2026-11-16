//go:build unit

package monitorlogsstream_test

import (
	"context"
	"encoding/json"
	"testing"

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

func categoriesDetails(t *testing.T, categories types.Set) types.Object {
	t.Helper()
	details, diags := types.ObjectValueFrom(context.Background(), streamDetailsAttrTypes(), monitorlogsstream.StreamDetailsModel{
		Categories:                  categories,
		ClusterIDs:                  types.ListNull(types.Int64Type),
		IsAutoAddAllClustersEnabled: types.BoolNull(),
	})
	require.False(t, diags.HasError(), "%v", diags)
	return details
}

func TestAuditCategoriesOptions(t *testing.T) {
	ctx := context.Background()
	for _, category := range []string{"config", "login", "data_plane"} {
		t.Run(category, func(t *testing.T) {
			categories := types.SetValueMust(types.StringType, []attr.Value{types.StringValue(category)})
			m := monitorlogsstream.ResourceModel{
				Type:         types.StringValue("audit_logs"),
				Label:        types.StringValue("Audit logs"),
				Destinations: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
				Details:      categoriesDetails(t, categories),
			}
			var diags diag.Diagnostics
			create := m.GetCreateOptions(ctx, &diags)
			update := m.GetUpdateOptions(ctx, &diags)
			require.False(t, diags.HasError(), "%v", diags)
			expected := `{"categories":["` + category + `"]}`
			for _, details := range []*linodego.StreamDetails{create.Details, update.Details} {
				require.NotNil(t, details)
				payload, err := json.Marshal(details)
				require.NoError(t, err)
				assert.JSONEq(t, expected, string(payload))
			}
		})
	}
}

func TestAuditCategoriesOmittedOptions(t *testing.T) {
	ctx := context.Background()
	for _, details := range []types.Object{
		types.ObjectNull(streamDetailsAttrTypes()),
		types.ObjectUnknown(streamDetailsAttrTypes()),
		categoriesDetails(t, types.SetNull(types.StringType)),
		categoriesDetails(t, types.SetUnknown(types.StringType)),
		categoriesDetails(t, types.SetValueMust(types.StringType, []attr.Value{})),
	} {
		m := monitorlogsstream.ResourceModel{
			Type:         types.StringValue("audit_logs"),
			Destinations: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
			Details:      details,
		}
		var diags diag.Diagnostics
		create := m.GetCreateOptions(ctx, &diags)
		update := m.GetUpdateOptions(ctx, &diags)
		require.False(t, diags.HasError(), "%v", diags)
		assert.Nil(t, create.Details)
		assert.Nil(t, update.Details)
	}
}

func TestFlattenAuditCategories(t *testing.T) {
	ctx := context.Background()
	categories := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("login"), types.StringValue("config"),
	})
	for _, preserveKnown := range []bool{false, true} {
		m := monitorlogsstream.ResourceModel{Details: categoriesDetails(t, categories)}
		stream := &linodego.Stream{
			Type: linodego.StreamTypeAuditLogs,
			Details: &linodego.StreamDetails{
				Categories: []string{"config", "login"},
			},
		}
		var diags diag.Diagnostics
		m.FlattenStream(ctx, stream, preserveKnown, &diags)
		require.False(t, diags.HasError(), "%v", diags)
		assert.True(t, m.Details.Equal(categoriesDetails(t, categories)))

		var ds monitorlogsstream.DataSourceModel
		ds.FlattenStream(ctx, stream, &diags)
		require.False(t, diags.HasError(), "%v", diags)
		assert.True(t, ds.Details.Equal(categoriesDetails(t, categories)))
	}

	m := monitorlogsstream.ResourceModel{
		Details: types.ObjectUnknown(streamDetailsAttrTypes()),
	}
	var diags diag.Diagnostics
	m.FlattenStream(ctx, &linodego.Stream{
		Type:    linodego.StreamTypeAuditLogs,
		Details: &linodego.StreamDetails{Categories: []string{"config", "login"}},
	}, true, &diags)
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, m.Details.Equal(categoriesDetails(t, categories)))
}

func TestEmptyAuditCategoriesPreserveUnmanagedState(t *testing.T) {
	ctx := context.Background()
	empty := types.SetValueMust(types.StringType, []attr.Value{})
	stream := &linodego.Stream{
		Type:    linodego.StreamTypeAuditLogs,
		Details: &linodego.StreamDetails{Categories: []string{"data_plane"}},
	}

	for _, preserveKnown := range []bool{true, false} {
		m := monitorlogsstream.ResourceModel{Details: categoriesDetails(t, empty)}
		var diags diag.Diagnostics
		m.FlattenStream(ctx, stream, preserveKnown, &diags)
		require.False(t, diags.HasError(), "%v", diags)
		assert.True(t, m.Details.Equal(categoriesDetails(t, empty)))
	}

	ds := monitorlogsstream.DataSourceModel{Details: categoriesDetails(t, empty)}
	var diags diag.Diagnostics
	ds.FlattenStream(ctx, stream, &diags)
	require.False(t, diags.HasError(), "%v", diags)
	actual := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("data_plane")})
	assert.True(t, ds.Details.Equal(categoriesDetails(t, actual)))
}

func TestValidateAuditDetails(t *testing.T) {
	ctx := context.Background()
	categories := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("data_plane")})
	tests := []struct {
		name       string
		streamType string
		clusters   types.List
		autoAdd    types.Bool
		wantError  bool
	}{
		{"valid audit", "audit_logs", types.ListNull(types.Int64Type), types.BoolNull(), false},
		{"categories on LKE", "lke_audit_logs", types.ListNull(types.Int64Type), types.BoolNull(), true},
		{"cluster IDs on audit", "audit_logs", types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}), types.BoolNull(), true},
		{"explicit false on audit", "audit_logs", types.ListNull(types.Int64Type), types.BoolValue(false), true},
		{"explicit true on audit", "audit_logs", types.ListNull(types.Int64Type), types.BoolValue(true), true},
		{"unknown LKE fields deferred", "audit_logs", types.ListUnknown(types.Int64Type), types.BoolUnknown(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details, d := types.ObjectValueFrom(ctx, streamDetailsAttrTypes(), monitorlogsstream.StreamDetailsModel{
				Categories: categories, ClusterIDs: tt.clusters, IsAutoAddAllClustersEnabled: tt.autoAdd,
			})
			require.False(t, d.HasError(), "%v", d)
			m := monitorlogsstream.ResourceModel{
				Type:         types.StringValue(tt.streamType),
				Destinations: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
				Details:      details,
			}
			var diags diag.Diagnostics
			m.ValidateAuditDetails(ctx, &diags)
			assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
			diags = nil
			m.GetCreateOptions(ctx, &diags)
			assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
			diags = nil
			m.GetUpdateOptions(ctx, &diags)
			assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
		})
	}
}

func TestAuditCategoriesValidators(t *testing.T) {
	ctx := context.Background()
	details := monitorlogsstream.ResourceSchema.Attributes["details"].(schema.SingleNestedAttribute)
	categories := details.Attributes["categories"].(schema.SetAttribute)
	tests := []struct {
		value     types.Set
		wantError bool
	}{
		{types.SetNull(types.StringType), false},
		{types.SetUnknown(types.StringType), false},
		{types.SetValueMust(types.StringType, []attr.Value{}), false},
		{types.SetValueMust(types.StringType, []attr.Value{types.StringValue("data")}), true},
		{types.SetValueMust(types.StringType, []attr.Value{types.StringValue("data_plane")}), false},
		{types.SetValueMust(types.StringType, []attr.Value{
			types.StringValue("config"), types.StringValue("login"), types.StringValue("data_plane"),
		}), false},
	}
	for _, tt := range tests {
		var diags diag.Diagnostics
		for _, v := range categories.Validators {
			var resp validator.SetResponse
			v.ValidateSet(ctx, validator.SetRequest{ConfigValue: tt.value}, &resp)
			diags.Append(resp.Diagnostics...)
		}
		assert.Equal(t, tt.wantError, diags.HasError(), "%v", diags)
	}
}

func TestFlattenLKECategoriesAreNull(t *testing.T) {
	ctx := context.Background()
	var m monitorlogsstream.ResourceModel
	var diags diag.Diagnostics
	m.FlattenStream(ctx, &linodego.Stream{
		Type:    linodego.StreamTypeLKEAuditLogs,
		Details: &linodego.StreamDetails{ClusterIDs: []int{123}},
	}, false, &diags)
	require.False(t, diags.HasError(), "%v", diags)
	var details monitorlogsstream.StreamDetailsModel
	diags.Append(m.Details.As(ctx, &details, basetypes.ObjectAsOptions{})...)
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, details.Categories.IsNull())
	assert.False(t, details.IsAutoAddAllClustersEnabled.ValueBool())
}
