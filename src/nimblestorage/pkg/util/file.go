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
	"bufio"
	"encoding/gob"
	"errors"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// FileReadFirstLine read first line from a file
//TODO: make it OS independent
func FileReadFirstLine(path string) (line string, er error) {
	LogDebug.Print("In FileReadFirstLine")
	file, err := os.Open(path)
	a := ""
	if err != nil {
		LogError.Print(err)
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		a = scanner.Text()
		break
	}
	if err = scanner.Err(); err != nil {
		LogError.Print(err)
		return "", err
	}
	LogDebug.Print(a)
	return a, err
}

//FileExists does a stat on the path and returns true if it exists
//In addition, dir returns true if the path is a directory
func FileExists(path string) (exists bool, dir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, info.IsDir(), nil
}

// FileGetStringsWithPattern  : get the filecontents as array of string matching pattern pattern
func FileGetStringsWithPattern(path string, pattern string) (filelines []string, err error) {
	LogDebug.Print("FileGetStringsWithPattern caleld with path: ", path, " Pattern: ", pattern)
	file, err := os.Open(path)
	if err != nil {
		LogError.Print(err)
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		LogError.Print(err)
		return nil, err
	}

	var matchingLines []string
	if pattern == "" {
		return lines, nil
	}
	r := regexp.MustCompile(pattern)
	for _, l := range lines {
		if r.MatchString(l) {
			matchString := r.FindAllStringSubmatch(l, -1)
			LogDebug.Print("matchingline :", matchString[0][1])
			matchingLines = append(matchingLines, matchString[0][1])
		}
	}

	return matchingLines, err
}

// FileGetStrings : get the file contents as array of string
func FileGetStrings(path string) (line []string, err error) {

	return FileGetStringsWithPattern(path, "")
}

//FileWriteString : write line to the path
func FileWriteString(path, line string) (err error) {
	LogDebug.Printf("in FileWriteString called with path: %s and string: %s ", path, line)
	var file *os.File
	is, _, err := FileExists(path)
	if !is {
		LogDebug.Print("File doesn't exist, Creating : " + path)
		file, err = os.Create(path)
		defer file.Close()
		if err != nil {
			LogDebug.Print("err", err)
			return err
		}
	}
	err = os.Chmod(path, 644)
	if err != nil {
		return err
	}
	file, err = os.OpenFile(path, os.O_RDWR, 644)
	if err != nil {
		LogError.Print(err)
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	n, err := writer.WriteString(line)
	writer.Flush()
	if err != nil {
		LogDebug.Print("Unable to write to file " + path + " Err:" + err.Error())
		err = errors.New("Unable to write to file " + path + " Err:" + err.Error())
		return err
	}
	if n <= 0 {
		LogDebug.Printf("File write didnt go through as bytes written is %d: ", n)
	}
	LogDebug.Printf("%d bytes written", n)

	return err
}

// FileWriteStrings writes all lines to file specified by path. Newline is appended to each line
func FileWriteStrings(path string, lines []string) (err error) {
	LogDebug.Printf("in FileWriteString called with path: %s", path)
	var file *os.File
	is, _, err := FileExists(path)
	if !is {
		LogDebug.Print("File doesn't exist, Creating : " + path)
		file, err = os.Create(path)
		defer file.Close()
		if err != nil {
			LogDebug.Print("err", err)
			return err
		}
	}
	err = os.Chmod(path, 644)
	if err != nil {
		return err
	}
	file, err = os.OpenFile(path, os.O_RDWR, 644)
	if err != nil {
		LogError.Print(err)
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer func() {
		err = w.Flush()
	}()

	for _, line := range lines {
		// trim if newline is already appended to string, as we add later
		w.WriteString(strings.Trim(line, "\n"))
		// always add a new line so that caller doesn't need to append everytime
		w.WriteString("\n")
	}
	return err
}

//FileDelete : delete the file
func FileDelete(path string) error {
	LogDebug.Print("File delete called")
	is, _, _ := FileExists(path)
	if !is {
		return errors.New("File doesnt exist " + path)
	}
	err := os.RemoveAll(path)
	if err != nil {
		return errors.New("Unable to delete file " + path + " " + err.Error())
	}
	return nil
}

// FileSaveGob : save the Gob file
func FileSaveGob(path string, object interface{}) error {
	LogDebug.Print("FileSaveGob called with ", path)
	file, err := os.Create(path)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

// FileloadGob : Load and Decode Gob file
func FileloadGob(path string, object interface{}) error {
	LogDebug.Print("FileloadGob called with", path)
	file, err := os.Open(path)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}

// FileCheck : checks for error
func FileCheck(e error) {
	LogDebug.Print("FileCheck called")
	if e != nil {
		LogError.Print("err :", e.Error())
		_, file, line, _ := runtime.Caller(1)
		LogError.Print("Line", line, "File", file, "Error:", e)
	}
}
