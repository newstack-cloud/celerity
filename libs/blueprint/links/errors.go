package links

import (
	"fmt"
)

type LinkError struct {
	ReasonCode   LinkErrorReasonCode
	Err          error
	FromResource *ResourceWithNameAndSelectors
	ToResource   *ResourceWithNameAndSelectors
	FromLink     *ChainLinkNode
	ToLink       *ChainLinkNode
	ChildErrors  []error
}

func (e *LinkError) Error() string {
	childErrCount := len(e.ChildErrors)
	if childErrCount == 0 {
		return fmt.Sprintf("blueprint linking error: %s", e.Err.Error())
	}
	return fmt.Sprintf("blueprint linking error (%d child errors): %s", childErrCount, e.Err.Error())
}

type LinkErrorReasonCode string

const (
	// LinkErrorReasonCodeMissingLinkImpl is provided
	// when the reason for a blueprint chain link building error
	// is due to a missing link implementation
	// when a resource type is reported to be able to link to
	// another resource type.
	LinkErrorReasonCodeMissingLinkImpl LinkErrorReasonCode = "missing_link_implementation"
	// LinkErrorReasonCodeCircularLinks is provided
	// when one or more circular links are found in a blueprint
	// in the process of building our chains to be used the blueprint container
	// for deployment orchestration.
	LinkErrorReasonCodeCircularLinks LinkErrorReasonCode = "circular_links"
	// LinkErrorReasonCodeCircularLink is provided
	// when a circular link is found in a blueprint
	// in the process of building our chains to be used the blueprint container
	// for deployment orchestration.
	LinkErrorReasonCodeCircularLink LinkErrorReasonCode = "circular_link"
)

func errMissingLinkImplementation(linkFromResource *ResourceWithNameAndSelectors, linkToResource *ResourceWithNameAndSelectors) error {
	return &LinkError{
		ReasonCode: LinkErrorReasonCodeMissingLinkImpl,
		Err: fmt.Errorf(
			"missing link implementation from \"%s(%s)\" to \"%s(%s)\"",
			linkFromResource.Name,
			linkFromResource.Resource.Type.Value,
			linkToResource.Name,
			linkToResource.Resource.Type.Value,
		),
		FromResource: linkFromResource,
		ToResource:   linkToResource,
	}
}

func errCircularLinks(circularLinkErrors []error) error {
	return &LinkError{
		ReasonCode: LinkErrorReasonCodeCircularLinks,
		Err: fmt.Errorf(
			"%d circular links found when attempting to build chains",
			len(circularLinkErrors),
		),
		ChildErrors: circularLinkErrors,
	}
}

func errCircularLink(
	linkFrom *ChainLinkNode,
	linkTo *ChainLinkNode,
	indirect bool,
) error {
	linkType := "direct"
	if indirect {
		linkType = "indirect"
	}

	return &LinkError{
		ReasonCode: LinkErrorReasonCodeCircularLink,
		Err: fmt.Errorf(
			"%s hard circular link found between \"%s(%s)\" and \"%s(%s)\"",
			linkType,
			linkFrom.ResourceName,
			linkFrom.Resource.Type.Value,
			linkTo.ResourceName,
			linkTo.Resource.Type.Value,
		),
		FromLink: linkFrom,
		ToLink:   linkTo,
	}
}
