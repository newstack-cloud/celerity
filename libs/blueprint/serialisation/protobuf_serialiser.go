package serialisation

// ProtobufExpandedBlueprintSerialiser is a service that serialises and deserialises
// expanded blueprints using the Protocol Buffer format.
// This is especially useful for serialising expanded blueprints
// for storage or transmission to distributed caches/data stores.
type ProtobufExpandedBlueprintSerialiser struct{}

// NewProtobufSerialiser creates an instance of an
// ExpandedBlueprintSerialiser that uses the Protocol Buffer format.
func NewProtobufSerialiser() ExpandedBlueprintSerialiser {
	return &ProtobufExpandedBlueprintSerialiser{}
}
