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

package util

import (
	"os"
	"path"
	"testing"
)

type testPath struct {
	path      string
	dirResult string
}

func getPaths() []testPath {
	tmpDir := os.TempDir()
	return []testPath{
		{
			path.Join(".", "testfile.test"),
			path.Join("."),
		},
		{
			path.Join(tmpDir, "testfile.test"),
			path.Join(tmpDir, "."),
		},
		{
			path.Join(tmpDir, "testdir.test", "testfile.test"),
			path.Join(tmpDir, "testdir.test"),
		},
		{
			path.Join(tmpDir, "testdir1.test", "testdir2.test", "testfile.test"),
			path.Join(tmpDir, "testdir1.test", "testdir2.test"),
		},
	}
}

func cleanUp() {
	tmpDir := os.TempDir()
	for _, pathInfo := range getPaths() {
		os.Remove(pathInfo.path)
	}
	os.Remove(path.Join(tmpDir, "testdir.test"))
	os.Remove(path.Join(tmpDir, "testdir1.test", "testdir2.test"))
}

func TestNotOpen(t *testing.T) {
	// log something at each level
	logAtAllLevels()
}

func TestOpenLogClose(t *testing.T) {
	defer cleanUp()

	for _, pathInfo := range getPaths() {
		//Open a logfile
		openLog(t, pathInfo.path, true)

		// make sure we created any needed directories
		is, dir, err := FileExists(pathInfo.dirResult)
		if err != nil {
			t.Error(
				"For", pathInfo.path,
				"expected", "no error",
				"got error:", err,
			)
		}
		if !is || !dir {
			t.Error(
				"For", pathInfo.path,
				"expected", pathInfo.dirResult,
				"got exists", is,
				"got dir", dir,
			)
		}

		// log something at each level
		logAtAllLevels()

		// make sure we can't call open again
		err = OpenLogFile(pathInfo.path, 1, 2, 3, true)
		if err == nil {
			t.Error(
				"For", pathInfo.path,
				"expected", "error (a log file is already open.)",
				"got", "no error",
			)
		}

		// close the log file
		CloseLogFile()

		// open with debug off
		openLog(t, pathInfo.path, false)
		// log something at each level
		logAtAllLevels()
		// close the log file
		CloseLogFile()
	}
}

func openLog(t *testing.T, path string, debug bool) {
	if err := OpenLogFile(path, 1, 2, 3, debug); err != nil {
		t.Error(
			"For", path,
			"expected", "no error",
			"got error:", err,
		)
	}
}

func logAtAllLevels() {
	LogDebug.Printf("%s: testing", "Paul Bunyan")
	LogInfo.Printf("%s: testing", "Blue")
	LogError.Printf("%s: testing", "Axe")
}
