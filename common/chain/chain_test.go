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
package chain

import (
	"fmt"
	"github.com/hpe-storage/dory/common/util"
	"testing"
)

type testAdder struct {
	data  int
	err   bool
	chain *Chain
}

func (tr *testAdder) Run() (interface{}, error) {
	foo := 0
	otherData := tr.chain.GetRunnerOutput(fmt.Sprintf("testTask%d", tr.data-1))
	if otherData != nil {
		foo = otherData.(int)
	}

	if tr.err {
		return tr.data, fmt.Errorf("bad news")
	}
	return tr.data + foo, nil
}

func (tr *testAdder) Rollback() error {
	if tr.err {
		return fmt.Errorf("rollback bad news")
	}
	return nil
}

func (tr *testAdder) Name() string {
	return fmt.Sprintf("testTask%d", tr.data)
}

var basicTests = []struct {
	name        string
	testData    []int
	testFails   []bool
	testResults []int
}{
	{"4 commands - no error", []int{1, 2, 3, 4}, []bool{false, false, false, false}, []int{1, 3, 6, 10}},
	{"4 commands - error[1]", []int{1, 2, 3, 4}, []bool{false, true, false, false}, []int{1, -1, -1, -1}},
	{"5 commands - no error", []int{1, 2, 3, 4, 5}, []bool{false, false, false, false, false}, []int{1, 3, 6, 10, 15}},
	{"5 commands - error[3]", []int{1, 2, 3, 4, 5}, []bool{false, false, false, true, false}, []int{1, 3, 6, -1, -1}},
	{"6 commands - no error", []int{1, 2, 3, 4, 5, 6}, []bool{false, false, false, false, false, false}, []int{1, 3, 6, 10, 15, 21}},
	{"6 commands - error[3]", []int{1, 2, 3, 4, 5, 6}, []bool{false, false, false, true, false, false}, []int{1, 3, 6, -1, -1, -1}},
}

func TestBasic(t *testing.T) {
	util.OpenLog(true)

	for _, tc := range basicTests {
		t.Run(tc.name, func(t *testing.T) {
			testChain := NewChain(2, 0)
			for i := range tc.testData {
				testChain.AppendRunner(&testAdder{tc.testData[i], tc.testFails[i], testChain})
			}
			errorCheck(tc.name, tc.testFails, testChain, t)

			err := testChain.Execute()
			if err == nil {
				t.Fatalf("%s: should not be able to execute the same chain twice", tc.name)
			}
			err = testChain.AppendRunner(nil)
			if err == nil {
				t.Fatalf("%s: should not be able to append runners to an executed chain", tc.name)
			}

			for i, result := range tc.testResults {
				if tc.testResults[i] == -1 {
					if testChain.GetRunnerOutput(fmt.Sprintf("testTask%d", i+1)) != nil {
						t.Fatalf("%s: result for index %d should be <nil>; got %v", tc.name, i, testChain.GetRunnerOutput(fmt.Sprintf("testTask%d", i+1)))
					}
				} else if testChain.GetRunnerOutput(fmt.Sprintf("testTask%d", i+1)) != result {
					t.Fatalf("%s: result for index %d should be %v; got %v", tc.name, i, result, testChain.GetRunnerOutput(fmt.Sprintf("testTask%d", i+1)))
				}
			}
		})
	}
}

func TestMistakes(t *testing.T) {
	util.OpenLog(true)

	testChain := NewChain(0, 0)
	testChain.AppendRunner(&testAdder{1, false, testChain})
	testChain.AppendRunner(&testAdder{1, false, testChain})
	err := testChain.Execute()
	if err == nil {
		t.Fatalf("%s: should not be able to execute the chain with two runners named the same thing", "TestMistakes - samename")
	}

	testChain = NewChain(0, 0)
	testChain.AppendRunner(nil)
	testChain.AppendRunner(&testAdder{1, true, testChain})
	err = testChain.Execute()
	if err == nil {
		t.Fatalf("%s: should get an error for a chain with a nil runner with a failed command", "TestMistakes - nil task")
	}

	testChain = NewChain(0, 0)
	testChain.AppendRunner(nil)
	testChain.AppendRunner(&testAdder{1, false, testChain})
	err = testChain.Execute()
	if err != nil {
		t.Fatalf("%s: should not get a error for a chain with a nil runner", "TestMistakes - nil task")
	}

}

func errorCheck(name string, b []bool, chain *Chain, t *testing.T) {
	err := chain.Execute()
	shouldFail := false
	for _, fail := range b {
		if fail {
			shouldFail = true
			break
		}
	}

	if err != nil {
		if !shouldFail {
			t.Fatalf("%s: result for test chain should be no error; got %v", name, err)
		}
		if chain.Error() != err {
			t.Fatalf("%s: chain error (%v) should match returned error (%v)", name, chain.Error(), err)
		}
		if chain.ErrorRollback() == nil {
			t.Fatalf("%s: rollback error (%v) should not be nil", name, chain.ErrorRollback())
		}
	} else {
		if shouldFail {
			t.Fatalf("%s: result for test chain should be error; got %v", name, err)
		}
	}
}
