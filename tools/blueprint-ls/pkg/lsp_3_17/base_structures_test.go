package lsp

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type PositionTestSuite struct {
	suite.Suite
}

func TestPositionTestSuite(t *testing.T) {
	suite.Run(t, new(PositionTestSuite))
}

func (s *PositionTestSuite) Test_position_for_utf16_offset_counting() {
	input := "This is the first line\n" + // 22 bytes
		"This is the second line\n" + // 23 bytes
		"This is the third line\n" + // 22 bytes
		"Fourth line𒀃𐐀here" // 15 utf-16 code points before target char, 19 bytes.
	position := Position{
		Line: 4,
		// "h" in "here"
		Character: 15,
	}
	s.Equal(89, position.IndexIn(input, PositionEncodingKindUTF16))
}

func (s *PositionTestSuite) Test_position_for_utf8_offset_counting() {
	input := "This is the first line\n" + // 22 bytes
		"This is the second line\n" + // 23 bytes
		"This is the third line\n" + // 22 bytes
		"Fourth line𒀃𐐀here" // 19 utf-8 code points before target char, 19 bytes.
	position := Position{
		Line: 4,
		// "h" in "here"
		Character: 19,
	}

	s.Equal(89, position.IndexIn(input, PositionEncodingKindUTF8))
}

func (s *PositionTestSuite) Test_position_for_utf32_offset_counting() {
	input := "This is the first line\n" + // 22 bytes
		"This is the second line\n" + // 23 bytes
		"This is the third line\n" + // 22 bytes
		"Fourth line𒀃𐐀here" // 13 utf-32 code points before target char, 19 bytes.
	position := Position{
		Line: 4,
		// "h" in "here"
		Character: 13,
	}

	s.Equal(89, position.IndexIn(input, PositionEncodingKindUTF32))
}
