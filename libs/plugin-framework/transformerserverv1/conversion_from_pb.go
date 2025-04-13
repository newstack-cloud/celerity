package transformerserverv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	sharedtypesv1 "github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

func fromPBTypeDescription(
	typeDescripion *sharedtypesv1.TypeDescription,
) *transform.AbstractResourceGetTypeDescriptionOutput {
	return &transform.AbstractResourceGetTypeDescriptionOutput{
		MarkdownDescription:  typeDescripion.MarkdownDescription,
		PlainTextDescription: typeDescripion.PlainTextDescription,
		MarkdownSummary:      typeDescripion.MarkdownSummary,
		PlainTextSummary:     typeDescripion.PlainTextSummary,
	}
}
