package monitorlogsstream

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/linode/linodego/v2"
)

// ValidateVPNDetails checks the relationship between the stream type and its details.
func (m *ResourceModel) ValidateVPNDetails(ctx context.Context, diags *diag.Diagnostics) {
	if m.Type.IsNull() || m.Type.IsUnknown() || m.Details.IsUnknown() {
		return
	}

	isVPN := m.Type.ValueString() == string(linodego.StreamTypeVPNGWIPSecLogs) ||
		m.Type.ValueString() == string(linodego.StreamTypeVPNGWBGPLogs)

	if m.Details.IsNull() {
		if isVPN {
			diags.AddAttributeError(path.Root("details"), "Missing VPN Gateway details",
				"VPN Gateway streams require details.vpn_gateway_ids.")
		}
		return
	}

	var details StreamDetailsModel
	diags.Append(m.Details.As(ctx, &details, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}

	if !isVPN {
		if !details.VPNGatewayIDs.IsNull() && !details.VPNGatewayIDs.IsUnknown() {
			diags.AddAttributeError(path.Root("details").AtName("vpn_gateway_ids"),
				"Unsupported VPN Gateway details", "vpn_gateway_ids can only be used with VPN Gateway stream types.")
		}
		return
	}

	if details.VPNGatewayIDs.IsNull() ||
		(!details.VPNGatewayIDs.IsUnknown() && len(details.VPNGatewayIDs.Elements()) == 0) {
		diags.AddAttributeError(path.Root("details").AtName("vpn_gateway_ids"),
			"Missing VPN Gateway IDs", "VPN Gateway streams require at least one gateway ID.")
	}
	if (!details.ClusterIDs.IsNull() && !details.ClusterIDs.IsUnknown()) ||
		(!details.IsAutoAddAllClustersEnabled.IsNull() && !details.IsAutoAddAllClustersEnabled.IsUnknown()) {
		diags.AddAttributeError(path.Root("details"), "Incompatible stream details",
			"VPN Gateway streams cannot include cluster_ids or is_auto_add_all_clusters_enabled.")
	}
}
