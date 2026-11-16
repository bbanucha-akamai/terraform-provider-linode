package monitorlogsstream

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/linode/linodego/v2"
)

// ValidateAuditDetails checks that categories are only used with audit logs.
func (m *ResourceModel) ValidateAuditDetails(ctx context.Context, diags *diag.Diagnostics) {
	if m.Type.IsNull() || m.Type.IsUnknown() || m.Details.IsNull() || m.Details.IsUnknown() {
		return
	}

	var details StreamDetailsModel
	diags.Append(m.Details.As(ctx, &details, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}

	if m.Type.ValueString() != string(linodego.StreamTypeAuditLogs) {
		if !details.Categories.IsNull() && !details.Categories.IsUnknown() {
			diags.AddAttributeError(path.Root("details").AtName("categories"),
				"Unsupported audit log categories", "categories can only be used with audit_logs streams.")
		}
		return
	}

	if (!details.ClusterIDs.IsNull() && !details.ClusterIDs.IsUnknown()) ||
		(!details.IsAutoAddAllClustersEnabled.IsNull() && !details.IsAutoAddAllClustersEnabled.IsUnknown()) {
		diags.AddAttributeError(path.Root("details"), "Incompatible stream details",
			"audit_logs streams cannot include cluster_ids or is_auto_add_all_clusters_enabled.")
	}
}
