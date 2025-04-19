package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func (s *UtilsTestSuite) Test_map_to_slice() {
	input := map[string]int{
		"value1": 54,
		"value2": 78,
		"value3": 821,
	}
	intSlice := MapToSlice(input)

	// The conversion function can not guarantee ordering
	// so we'll sort to make it simpler to assert the expected
	// values.
	sort.Ints(intSlice)
	s.Assert().Equal(
		[]int{54, 78, 821},
		intSlice,
	)
}

func (s *UtilsTestSuite) Test_slice_to_map_keys() {
	input := []string{"key1", "key2", "key3", "key4", "key5"}
	mapping := SliceToMapKeys[int](input)
	s.Assert().Equal(
		map[string][]int{
			"key1": {},
			"key2": {},
			"key3": {},
			"key4": {},
			"key5": {},
		},
		mapping,
	)
}

type equalStr string

func (s equalStr) Equal(compare equalStr) bool {
	return s == compare
}

func (s *UtilsTestSuite) Test_slice_contains_value() {
	values := []equalStr{"value1", "value2", "value3"}
	s.Assert().True(SliceContains(values, "value3"))
}

func (s *UtilsTestSuite) Test_slice_does_not_contain_value() {
	values := []equalStr{"val130292", "val3029383", "val09281292"}
	s.Assert().False(SliceContains(values, "value4"))
}

func (s *UtilsTestSuite) Test_slice_contains_comparable_value() {
	values := []int{4, 5, 8, 12}
	s.Assert().True(
		SliceContainsComparable(values, 8),
	)
}

func (s *UtilsTestSuite) Test_slice_contains_comparable_does_not_contain_value() {
	values := []float64{0.6503, 2.39, 7.5402}
	s.Assert().False(
		SliceContainsComparable(values, 8.921),
	)
}

func (s *UtilsTestSuite) Test_map_same_output_type() {
	inputs := []string{
		"element1",
		"element2",
		"element3",
		"element4",
		"element5",
	}
	expected := []string{
		"element1_mapped",
		"element2_mapped",
		"element3_mapped",
		"element4_mapped",
		"element5_mapped",
	}
	s.Assert().Equal(
		expected,
		Map(
			inputs,
			func(input string, index int) string {
				return fmt.Sprintf("%s_mapped", input)
			},
		),
	)
}

func (s *UtilsTestSuite) Test_map_different_output_type() {
	inputs := []string{
		"element1",
		"element2",
		"element3",
		"element4",
		"element5",
	}
	expected := []int{
		1,
		2,
		3,
		4,
		5,
	}
	s.Assert().Equal(
		expected,
		Map(
			inputs,
			func(input string, index int) int {
				return index + 1
			},
		),
	)
}

func (s *UtilsTestSuite) Test_filter() {
	inputs := []string{
		"Breakthrough",
		"Exponential",
		"Circuit Breaker",
		"Backoff",
		"Something Breaks",
	}
	expected := []string{
		"Breakthrough",
		"Circuit Breaker",
		"Something Breaks",
	}
	s.Assert().Equal(
		expected,
		Filter(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Break")
			},
		),
	)
}

func (s *UtilsTestSuite) Test_find_when_item_exists() {
	inputs := []string{
		"Breakthrough",
		"Exponential",
		"Circuit Breaker",
		"Backoff",
		"Something Breaks",
	}
	expected := "Circuit Breaker"
	s.Assert().Equal(
		expected,
		Find(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Breaker")
			},
		),
	)
}

func (s *UtilsTestSuite) Test_find_when_item_does_not_exist() {
	inputs := []string{
		"Item 1",
		"Item 2",
		"Item 3",
		"Item 4",
		"Item 5",
		"Item 60938",
	}
	expected := ""
	s.Assert().Equal(
		expected,
		Find(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Item 968404359")
			},
		),
	)
}

func (s *UtilsTestSuite) Test_find_index_when_item_exists() {
	inputs := []string{
		"Excellence",
		"Achieving",
		"Failure",
		"Learning",
		"Fear",
	}
	expected := 3
	s.Assert().Equal(
		expected,
		FindIndex(
			inputs,
			func(item string, index int) bool {
				return item == "Learning"
			},
		),
	)
}

func (s *UtilsTestSuite) Test_find_index_when_item_does_not_exist() {
	inputs := []string{
		"Element 1",
		"Element 2",
		"Element 3",
		"Element 4",
		"Element 5",
		"Element 142938",
	}
	expected := -1
	s.Assert().Equal(
		expected,
		FindIndex(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Element 968404359")
			},
		),
	)
}

func (s *UtilsTestSuite) Test_remove_duplicates() {
	inputs := []string{
		"Breakthrough",
		"Exponential",
		"Circuit Breaker",
		"Backoff",
		"Something Breaks",
		"Circuit Breaker",
		"Backoff",
		"Exponential",
		"Breakthrough",
		"Breakthrough",
	}
	expected := []string{
		"Breakthrough",
		"Exponential",
		"Circuit Breaker",
		"Backoff",
		"Something Breaks",
	}
	s.Assert().Equal(
		expected,
		RemoveDuplicates(
			inputs,
		),
	)
}

func (s *UtilsTestSuite) Test_shallow_copy_map() {
	inputMap := map[string]string{
		"item1":  "Breakthrough",
		"item2":  "Exponential",
		"item3":  "Circuit Breaker",
		"item4":  "Backoff",
		"item5":  "Something Breaks",
		"item6":  "Circuit Breaker",
		"item7":  "Backoff",
		"item8":  "Exponential",
		"item9":  "Breakthrough",
		"item10": "Breakthrough",
	}
	mapCopy := ShallowCopyMap(inputMap)
	s.Assert().Equal(
		inputMap,
		mapCopy,
	)
}

func (s *UtilsTestSuite) Test_reverse() {
	inputSlice := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	reversed := Reverse(inputSlice)
	s.Assert().Equal(
		[]string{
			"Item5",
			"Item4",
			"Item3",
			"Item2",
			"Item1",
		},
		reversed,
	)
	// It's important that we test the behaviour in the Reverse function
	// does not modify the original as mentioned in the function's comment
	// as a part of its API.
	s.Assert().Equal(
		[]string{
			"Item1",
			"Item2",
			"Item3",
			"Item4",
			"Item5",
		},
		inputSlice,
	)
}

func (s *UtilsTestSuite) Test_slice_equals_true() {
	inputSlice1 := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	inputSlice2 := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	slicesEqual := SlicesEqual(inputSlice1, inputSlice2)
	s.Assert().True(slicesEqual)
}

func (s *UtilsTestSuite) Test_slice_equals_false_mismatch_length() {
	inputSlice1 := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	inputSlice2 := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item5",
	}
	slicesEqual := SlicesEqual(inputSlice1, inputSlice2)
	s.Assert().False(slicesEqual)
}

func (s *UtilsTestSuite) Test_slice_equals_false_mismatch_items() {
	inputSlice1 := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	inputSlice2 := []string{
		"Item1",
		"Item20",
		"Item30",
		"Item4",
		"Item5",
	}
	slicesEqual := SlicesEqual(inputSlice1, inputSlice2)
	s.Assert().False(slicesEqual)
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}
