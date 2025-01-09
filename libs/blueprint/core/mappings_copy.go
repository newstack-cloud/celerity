package core

// CopyMappingNode produces a deep copy of the provided mapping node.
// This does not copy StringWithSubstitutions, this should only be used for
// mapping nodes for which all values have been resolved and substituted.
func CopyMappingNode(node *MappingNode) *MappingNode {
	if node == nil {
		return nil
	}

	if node.Scalar != nil {
		return &MappingNode{
			Scalar: copyScalar(node.Scalar),
		}
	}

	if node.Fields != nil {
		return &MappingNode{
			Fields: copyFields(node.Fields),
		}
	}

	if node.Items != nil {
		return &MappingNode{
			Items: copyItems(node.Items),
		}
	}

	return nil
}

func copyFields(fields map[string]*MappingNode) map[string]*MappingNode {
	if fields == nil {
		return nil
	}

	fieldsCopy := make(map[string]*MappingNode, len(fields))
	for k, v := range fields {
		fieldsCopy[k] = CopyMappingNode(v)
	}

	return fieldsCopy
}

func copyItems(items []*MappingNode) []*MappingNode {
	if items == nil {
		return nil
	}

	itemsCopy := make([]*MappingNode, len(items))
	for i, item := range items {
		itemsCopy[i] = CopyMappingNode(item)
	}

	return itemsCopy
}

func copyScalar(scalar *ScalarValue) *ScalarValue {
	if scalar == nil {
		return nil
	}

	if scalar.IntValue != nil {
		intCopy := *scalar.IntValue
		return &ScalarValue{
			IntValue: &intCopy,
		}
	}

	if scalar.FloatValue != nil {
		floatCopy := *scalar.FloatValue
		return &ScalarValue{
			FloatValue: &floatCopy,
		}
	}

	if scalar.StringValue != nil {
		stringCopy := *scalar.StringValue
		return &ScalarValue{
			StringValue: &stringCopy,
		}
	}

	if scalar.BoolValue != nil {
		boolCopy := *scalar.BoolValue
		return &ScalarValue{
			BoolValue: &boolCopy,
		}
	}

	return nil
}
