/*
(c) Copyright 2017 Hewlett Packard Enterprise Development LP

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jconfig

import (
	"testing"
)

var (
	mapsSlice []map[string]interface{}
)

var basicTests = []struct {
	name              string
	testKey           string
	resultString      string
	resultInt         int64
	resultStringSlice []string
	resultBool        bool
	resultBoolError   bool
	resultMapSlice    []map[string]interface{}
}{
	{"get someString", "someString", "some string", 0, nil, false, true, nil},
	{"get stringNumber", "stringNumber", "23", 23, nil, false, true, nil},
	{"get actualNumber", "actualNumber", "1.073741824e+10", 10737418240, nil, false, true, nil},
	{"get badIntNumber", "badIntNumber", "2.3", 2, nil, false, true, nil},
	{"get notFound", "no key named this", "", 0, nil, false, true, nil},
	{"get smallNumber", "smallNumber", "40", 40, nil, false, true, nil},
	{"get someStrings", "someStrings", "[first 2nd c]", 0, []string{"first", "2nd", "c"}, false, true, nil},
	{"get boolean", "boolean", "true", 0, nil, true, false, nil},
	{"get stringBool", "stringBool", "True", 0, nil, true, false, nil},
	{"get someMaps", "someMaps", "[map[first:1] map[second:2]]", 0, nil, false, false, append(mapsSlice, map[string]interface{}{"first": 1}, map[string]interface{}{"second": 2})},
}

func TestBasic(t *testing.T) {
	c, err := NewConfig("./test.json")
	if err != nil {
		t.Error(
			"For file load of ./test.json",
			"expected", "no error",
			"got error:", err,
		)
	}

	for _, tc := range basicTests {
		t.Run(tc.name, func(t *testing.T) {
			s := c.GetString(tc.testKey)
			if s != tc.resultString {
				t.Fatalf("%s: GetString(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultString, s)
			}
			i := c.GetInt64(tc.testKey)
			if i != tc.resultInt {
				t.Fatalf("%s: GetInt64(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultInt, i)
			}
			ss := c.GetStringSlice(tc.testKey)
			if ss != nil && tc.resultStringSlice != nil {
				for x := range tc.resultStringSlice {
					if ss[x] != tc.resultStringSlice[x] {
						t.Fatalf("%s: GetStringSlice(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultStringSlice, ss)
					}
				}
			}
			b, _ := c.GetBool(tc.testKey)
			if b != tc.resultBool {
				t.Fatalf("%s: GetBool(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultBool, b)
			}
			ms, _ := c.GetMapSlice(tc.testKey)
			testMapSlice(ms, tc.resultMapSlice, t)
		})
	}
}

func testMapSlice(ms []map[string]interface{}, resultSlice []map[string]interface{}, t *testing.T) {
	if ms != nil && resultSlice != nil {
		for x := range resultSlice {
			if ms[x] == nil {
				t.Errorf("GetMapSlice should return %v; got %v", resultSlice, ms)
			}
		}
	}
}

func TestBroken(t *testing.T) {
	_, err := NewConfig("./broken.json")
	if err == nil {
		t.Errorf("%s: FileLoadConfig(./broken.json) should get error.", "TestBroken")
	}
}

func TestFNF(t *testing.T) {
	_, err := NewConfig("./missing.json")
	if err == nil {
		t.Errorf("%s: FileLoadConfig(./missing.json) should get error.", "TestFNF")
	}
}
