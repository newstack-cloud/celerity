//go:build unit

package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type UtilsTestSuite struct{}

var _ = Suite(&UtilsTestSuite{})

func (s *UtilsTestSuite) Test_map_to_slice(c *C) {
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
	c.Assert(intSlice, DeepEquals, []int{54, 78, 821})
}

func (s *UtilsTestSuite) Test_slice_to_map_keys(c *C) {
	input := []string{"key1", "key2", "key3", "key4", "key5"}
	mapping := SliceToMapKeys[int](input)
	c.Assert(mapping, DeepEquals, map[string][]int{
		"key1": {},
		"key2": {},
		"key3": {},
		"key4": {},
		"key5": {},
	})
}

type equalStr string

func (s equalStr) Equal(compare equalStr) bool {
	return s == compare
}

func (s *UtilsTestSuite) Test_slice_contains_value(c *C) {
	values := []equalStr{"value1", "value2", "value3"}
	c.Assert(SliceContains(values, "value3"), Equals, true)
}

func (s *UtilsTestSuite) Test_slice_does_not_contain_value(c *C) {
	values := []equalStr{"val130292", "val3029383", "val09281292"}
	c.Assert(SliceContains(values, "value4"), Equals, false)
}

func (s *UtilsTestSuite) Test_slice_contains_comparable_value(c *C) {
	values := []int{4, 5, 8, 12}
	c.Assert(SliceContainsComparable(values, 8), Equals, true)
}

func (s *UtilsTestSuite) Test_slice_contains_comparable_does_not_contain_value(c *C) {
	values := []float64{0.6503, 2.39, 7.5402}
	c.Assert(SliceContainsComparable(values, 8.921), Equals, false)
}

func (s *UtilsTestSuite) Test_map_same_output_type(c *C) {
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
	c.Assert(
		Map(
			inputs,
			func(input string, index int) string {
				return fmt.Sprintf("%s_mapped", input)
			},
		),
		DeepEquals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_map_different_output_type(c *C) {
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
	c.Assert(
		Map(
			inputs,
			func(input string, index int) int {
				return index + 1
			},
		),
		DeepEquals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_filter(c *C) {
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
	c.Assert(
		Filter(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Break")
			},
		),
		DeepEquals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_find_when_item_exists(c *C) {
	inputs := []string{
		"Breakthrough",
		"Exponential",
		"Circuit Breaker",
		"Backoff",
		"Something Breaks",
	}
	expected := "Circuit Breaker"
	c.Assert(
		Find(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Breaker")
			},
		),
		Equals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_find_when_item_does_not_exist(c *C) {
	inputs := []string{
		"Item 1",
		"Item 2",
		"Item 3",
		"Item 4",
		"Item 5",
		"Item 60938",
	}
	expected := ""
	c.Assert(
		Find(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Item 968404359")
			},
		),
		Equals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_find_index_when_item_exists(c *C) {
	inputs := []string{
		"Excellence",
		"Achieving",
		"Failure",
		"Learning",
		"Fear",
	}
	expected := 3
	c.Assert(
		FindIndex(
			inputs,
			func(item string, index int) bool {
				return item == "Learning"
			},
		),
		Equals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_find_index_when_item_does_not_exist(c *C) {
	inputs := []string{
		"Element 1",
		"Element 2",
		"Element 3",
		"Element 4",
		"Element 5",
		"Element 142938",
	}
	expected := -1
	c.Assert(
		FindIndex(
			inputs,
			func(item string, index int) bool {
				return strings.Contains(item, "Element 968404359")
			},
		),
		Equals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_remove_duplicates(c *C) {
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
	c.Assert(
		RemoveDuplicates(
			inputs,
		),
		DeepEquals,
		expected,
	)
}

func (s *UtilsTestSuite) Test_shallow_copy_map(c *C) {
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
	c.Assert(
		mapCopy,
		DeepEquals,
		inputMap,
	)
}

func (s *UtilsTestSuite) Test_reverse(c *C) {
	inputSlice := []string{
		"Item1",
		"Item2",
		"Item3",
		"Item4",
		"Item5",
	}
	reversed := Reverse(inputSlice)
	c.Assert(
		reversed,
		DeepEquals,
		[]string{
			"Item5",
			"Item4",
			"Item3",
			"Item2",
			"Item1",
		},
	)
	// It's important that we test the behaviour in the Reverse function
	// does not modify the original as mentioned in the function's comment
	// as a part of its API.
	c.Assert(
		inputSlice,
		DeepEquals,
		[]string{
			"Item1",
			"Item2",
			"Item3",
			"Item4",
			"Item5",
		},
	)
}

func (s *UtilsTestSuite) Test_slice_equals_true(c *C) {
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
	c.Assert(
		slicesEqual,
		Equals,
		true,
	)
}

func (s *UtilsTestSuite) Test_slice_equals_false_mismatch_length(c *C) {
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
	c.Assert(
		slicesEqual,
		Equals,
		false,
	)
}

func (s *UtilsTestSuite) Test_slice_equals_false_mismatch_items(c *C) {
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
	c.Assert(
		slicesEqual,
		Equals,
		false,
	)
}
