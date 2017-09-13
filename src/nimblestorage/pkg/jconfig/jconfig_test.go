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

var basicTests = []struct {
	name              string
	testKey           string
	resultString      string
	resultInt         int64
	resultStringSlice []string
	resultBool        bool
	resultBoolError   bool
}{
	{"get someString", "someString", "some string", 0, nil, false, true},
	{"get stringNumber", "stringNumber", "23", 23, nil, false, true},
	{"get actualNumber", "actualNumber", "1.073741824e+10", 10737418240, nil, false, true},
	{"get badIntNumber", "badIntNumber", "2.3", 2, nil, false, true},
	{"get notFound", "no key named this", "", 0, nil, false, true},
	{"get smallNumber", "smallNumber", "40", 40, nil, false, true},
	{"get someStrings", "someStrings", "[first 2nd c]", 0, []string{"first", "2nd", "c"}, false, true},
	{"get boolean", "boolean", "true", 0, nil, true, false},
	{"get stringBool", "stringBool", "True", 0, nil, true, false},
}

func TestBasic(t *testing.T) {
	err := FileLoadConfig("./test.json")
	if err != nil {
		t.Error(
			"For file load of ./test.json",
			"expected", "no error",
			"got error:", err,
		)
	}

	for _, tc := range basicTests {
		t.Run(tc.name, func(t *testing.T) {
			s := GetString(tc.testKey)
			if s != tc.resultString {
				t.Fatalf("%s: GetString(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultString, s)
			}
			i := GetInt64(tc.testKey)
			if i != tc.resultInt {
				t.Fatalf("%s: GetInt64(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultInt, i)
			}
			ss := GetStringSlice(tc.testKey)
			if ss != nil && tc.resultStringSlice != nil {
				for x := range tc.resultStringSlice {
					if ss[x] != tc.resultStringSlice[x] {
						t.Fatalf("%s: GetStringSlice(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultStringSlice, ss)
					}
				}
			}
			b, _ := GetBool(tc.testKey)
			if b != tc.resultBool {
				t.Fatalf("%s: GetBool(%v) should return %v; got %v", tc.name, tc.testKey, tc.resultBool, b)
			}
		})
	}
}

func TestBroken(t *testing.T) {
	err := FileLoadConfig("./broken.json")
	if err == nil {
		t.Errorf("%s: FileLoadConfig(./broken.json) should get error.", "TestBroken")
	}
}

func TestFNF(t *testing.T) {
	err := FileLoadConfig("./missing.json")
	if err == nil {
		t.Errorf("%s: FileLoadConfig(./missing.json) should get error.", "TestFNF")
	}
}
