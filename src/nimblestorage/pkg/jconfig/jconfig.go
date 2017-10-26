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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// Config contains a map loaded from t a json file
type Config struct {
	config map[string]interface{}
}

//NewConfig loads the JSON in the file referred to in the path
func NewConfig(path string) (*Config, error) {
	c := &Config{}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if file != nil {
		defer file.Close()
		if err := json.NewDecoder(file).Decode(&c.config); err != nil {
			return nil, err
		}
	}
	return c, nil
}

//GetString returns the string value loaded from the JSON (backward compatibility)
func (c *Config) GetString(key string) (s string) {
	s, _ = c.GetStringWithError(key)
	return
}

//GetStringWithError returns the string value loaded from the JSON
func (c *Config) GetStringWithError(key string) (s string, err error) {
	if _, found := c.config[key]; found {
		switch value := c.config[key].(type) {
		case string:
			return value, nil
		default:
			return fmt.Sprintf("%v", c.config[key]), nil
		}
	}
	return s, fmt.Errorf("key:%v not found", key)
}

//GetStringSlice returns the string value loaded from the JSON (backward compatibility)
func (c *Config) GetStringSlice(key string) (strings []string) {
	strings, _ = c.GetStringSliceWithError(key)
	return
}

//GetStringSliceWithError returns the string value loaded from the JSON
func (c *Config) GetStringSliceWithError(key string) (strings []string, err error) {
	if _, found := c.config[key]; found {
		switch value := c.config[key].(type) {
		case []interface{}:
			for _, d := range value {
				strings = append(strings, fmt.Sprintf("%v", d))
			}
			return strings, nil
		default:
			return strings, fmt.Errorf("key:%v is not a slice.  value:%v kind:%s type:%s", key, c.config[key], reflect.TypeOf(c.config[key]).Kind(), reflect.TypeOf(c.config[key]))
		}
	}
	return strings, fmt.Errorf("key:%v not found", key)
}

//GetInt64 returns the value in the JSON cast to int64 (backward compatibility)
func (c *Config) GetInt64(key string) (i int64) {
	i, _ = c.GetInt64SliceWithError(key)
	return
}

//GetInt64SliceWithError returns the value in the JSON cast to int64
func (c *Config) GetInt64SliceWithError(key string) (i int64, err error) {
	if _, found := c.config[key]; found {
		switch value := c.config[key].(type) {
		//json marshall stores numbers as floats
		case float64:
			return int64(value), nil
		//we can always try to parse a string
		case string:
			return strconv.ParseInt(value, 10, 64)
		default:
			return 0, fmt.Errorf("key:%v is not a number.  value:%v kind:%s type:%s", key, c.config[key], reflect.TypeOf(c.config[key]).Kind(), reflect.TypeOf(c.config[key]))
		}
	}
	return 0, fmt.Errorf("key:%v not found", key)
}

//GetBool returns the value in the JSON cast to bool
func (c *Config) GetBool(key string) (b bool, err error) {
	if _, found := c.config[key]; found {
		switch value := c.config[key].(type) {
		case bool:
			return bool(value), nil
		//we can always try to parse a string
		case string:
			return strconv.ParseBool(value)
		default:
			return false, fmt.Errorf("key:%v is not a bool.  value:%v kind:%s type:%s", key, c.config[key], reflect.TypeOf(c.config[key]).Kind(), reflect.TypeOf(c.config[key]))
		}
	}
	return false, fmt.Errorf("key:%v not found", key)
}
