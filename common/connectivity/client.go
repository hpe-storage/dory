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

package connectivity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hpe-storage/dory/common/util"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout = time.Duration(30) * time.Second
)

//Request encapsulates a request to the Do* family of functions
type Request struct {
	//Action to take, ie: GET, POST, PUT, PATCH, DELETE
	Action string
	//Path is the URI
	Path string
	//Payload to send (may be nil)
	Payload interface{}
	//Response to marshal into (may be nil)
	Response interface{}
	//ResponseError to marshal error into (may be nil)
	ResponseError interface{}
}

// Client is a simple wrapper for http.Client
type Client struct {
	*http.Client
	pathPrefix string
}

// NewHTTPClient returns a client that communicates over ip using a 30 second timeout
func NewHTTPClient(url string) *Client {
	return NewHTTPClientWithTimeout(url, defaultTimeout)
}

// NewHTTPClientWithTimeout returns a client that communicates over ip
func NewHTTPClientWithTimeout(url string, timeout time.Duration) *Client {
	if timeout < 1 {
		timeout = defaultTimeout
	}
	return &Client{&http.Client{Timeout: timeout}, url}
}

// NewHTTPSClientWithTimeout returns a client that communicates over ip with tls :
func NewHTTPSClientWithTimeout(url string, transport http.RoundTripper, timeout time.Duration) *Client {
	if timeout < 1 {
		timeout = defaultTimeout
	}
	return &Client{&http.Client{Timeout: timeout, Transport: transport}, url}
}

// NewHTTPSClient returns a new https client
func NewHTTPSClient(url string, transport http.RoundTripper) *Client {
	return NewHTTPSClientWithTimeout(url, transport, defaultTimeout)
}

// NewSocketClient returns a client that communicates over a unix socket using a 30 second connect timeout
func NewSocketClient(filename string) *Client {
	return NewSocketClientWithTimeout(filename, defaultTimeout)
}

// NewSocketClientWithTimeout returns a client that communicates over a unix file socket
func NewSocketClientWithTimeout(filename string, timeout time.Duration) *Client {
	if timeout < 1 {
		timeout = defaultTimeout
	}
	tr := &http.Transport{
		DisableCompression: true,
	}
	tr.Dial = func(_, _ string) (net.Conn, error) {
		return net.DialTimeout("unix", filename, timeout)
	}
	return &Client{&http.Client{Transport: tr, Timeout: timeout}, "http://unix"}
}

// DoJSON action on path.  payload and response are expected to be structs that decode/encode from/to json
// Example action=POST, path=/VolumeDriver.Create ...
// Tries 3 times to get data from the server
func (client *Client) DoJSON(r *Request) error {
	// make sure we have a root slash
	if !strings.HasPrefix(r.Path, "/") {
		r.Path = client.pathPrefix + "/" + r.Path
	} else {
		r.Path = client.pathPrefix + r.Path
	}

	var buf bytes.Buffer
	// encode the payload
	if r.Payload != nil {
		if err := json.NewEncoder(&buf).Encode(r.Payload); err != nil {
			return err
		}
	}

	// build request
	req, err := http.NewRequest(r.Action, r.Path, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/json")
	req.Close = true
	util.LogDebug.Printf("request: action=%s path=%s payload=%s", r.Action, r.Path, buf.String())

	// execute the do
	res, err := doWithRetry(client, req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// check the status code
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		//decode the body into the error response
		util.LogError.Printf("status code was %s for request: action=%s path=%s, attempting to decode error response.", res.Status, r.Action, r.Path)
		err = decode(res.Body, r.ResponseError, r)
		if err != nil {
			return err
		}
		return fmt.Errorf("status code was %s for request: action=%s path=%s, attempting to decode error response", res.Status, r.Action, r.Path)

	}

	err = decode(res.Body, r.Response, r)
	if err != nil {
		return err
	}

	return nil
}

func doWithRetry(client *Client, request *http.Request) (*http.Response, error) {
	try := 0
	maxTries := 3
	for {
		response, err := client.Do(request)
		if err != nil {
			if try < maxTries {
				try++
				time.Sleep(time.Duration(try) * time.Second)
				continue
			}
			return nil, err
		}
		util.LogDebug.Printf("response: %v, length=%v", response.Status, response.ContentLength)
		return response, nil
	}
}

func decode(rc io.ReadCloser, dest interface{}, r *Request) error {
	if rc != nil && dest != nil {
		if err := json.NewDecoder(rc).Decode(&dest); err != nil {
			util.LogError.Printf("unable to decode %v returned from action=%s to path=%s. error=%v", rc, r.Action, r.Path, err)
			return err
		}
	}
	return nil
}
