package transformerv1

import (
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/transformerserverv1"
)

func toTransformNameErrorResponse(err error) *transformerserverv1.TransformNameResponse {
	return &transformerserverv1.TransformNameResponse{
		Response: &transformerserverv1.TransformNameResponse_ErrorResponse{
			ErrorResponse: errorsv1.CreateResponseFromError(err),
		},
	}
}

func toPBTransformNameResponse(transformName string) *transformerserverv1.TransformNameResponse {
	return &transformerserverv1.TransformNameResponse{
		Response: &transformerserverv1.TransformNameResponse_NameInfo{
			NameInfo: &transformerserverv1.TransformNameInfo{
				TransformName: transformName,
			},
		},
	}
}
