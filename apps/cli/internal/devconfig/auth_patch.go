package devconfig

import (
	"strconv"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
)

const devAuthBasePort = 9099

// PatchJWTIssuer rewrites the issuer in every JWT auth guard of every
// celerity/api resource to point to the local dev auth sidecar. The
// merged blueprint (not the original source file) is modified in-place
// so the runtime performs OIDC discovery against the sidecar.
func PatchJWTIssuer(bp *schema.Blueprint, portOffset int) {
	if bp.Resources == nil {
		return
	}

	hostPort := strconv.Itoa(devAuthBasePort + portOffset)
	issuer := "http://host.docker.internal:" + hostPort

	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != compose.ResourceTypeAPI {
			continue
		}
		patchAPIResourceIssuer(resource, issuer)
	}
}

func patchAPIResourceIssuer(resource *schema.Resource, issuer string) {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return
	}
	authNode := resource.Spec.Fields["auth"]
	if authNode == nil || authNode.Fields == nil {
		return
	}
	guardsNode := authNode.Fields["guards"]
	if guardsNode == nil || guardsNode.Fields == nil {
		return
	}
	for _, guardNode := range guardsNode.Fields {
		if guardNode == nil || guardNode.Fields == nil {
			continue
		}
		if core.StringValue(guardNode.Fields["type"]) != "jwt" {
			continue
		}
		guardNode.Fields["issuer"] = &core.MappingNode{
			Scalar: core.ScalarFromString(issuer),
		}
	}
}
