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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	socket      = "socket.socket"
	socket2     = "socket2.socket"
	requestJSON = "{\"ping\":\"junk\"}\n"
	pathString  = "/woohoo"
)

type answer struct {
	Pong string
}

type badnews struct {
	Info string
}

type question struct {
	Ping string `json:"ping,omitempty"`
}

type testHandler struct {
	t *testing.T
}

func (th *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	switch r.URL.String() {
	case "/error":
		http.Error(w, "{\"info\":\"sending an error\"}", http.StatusInternalServerError)
	default:
		if pathString != r.URL.String() {
			th.t.Error(
				"For", "URL",
				"expected", pathString,
				"got", r.URL.String(),
			)
		}
		buf, _ := ioutil.ReadAll(r.Body)
		if requestJSON != fmt.Sprintf("%s", buf) {
			th.t.Error(
				"For", "requestedJSON",
				"expected", requestJSON,
				"got", fmt.Sprintf("%s", buf),
			)
		}
		fmt.Fprint(w, "{\"pong\":\"test\"}")
	}

}

type testTimeoutHandler struct {
	t *testing.T
}

func (th *testTimeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second)
	fmt.Fprint(w, "{\"pong\":\"test\"}")
}

func TestSocket(t *testing.T) {
	// server
	os.Remove(socket)
	defer os.Remove(socket)
	server := http.Server{}
	server.Handler = &testHandler{t: t}
	unixListener, err := net.Listen("unix", socket)
	if err != nil {
		t.Error(
			"trying to listen.  expected to start server!",
			"got error:", err,
		)
	}
	go server.Serve(unixListener)

	//client
	client := NewSocketClient(socket)

	var foo answer
	err = client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	verifyFoo(err, foo, t)

	var bad badnews
	err = client.DoJSON(
		&Request{
			Action:        "POST",
			Path:          "/error",
			Payload:       &question{Ping: "junk"},
			Response:      &foo,
			ResponseError: &bad,
		})
	verifyBadNews(err, bad, t)
}

func TestSocketTimeout(t *testing.T) {
	// server
	os.Remove(socket2)
	defer os.Remove(socket2)
	server := http.Server{}
	server.Handler = &testTimeoutHandler{t: t}
	unixListener, err := net.Listen("unix", socket2)
	if err != nil {
		t.Error(
			"trying to listen.  expected to start server!",
			"got error:", err,
		)
	}
	go server.Serve(unixListener)

	//client with timout
	client := NewSocketClientWithTimeout(socket2, time.Millisecond)
	var foo answer
	err = client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	if err == nil {
		t.Error(
			"client post expected to timeout",
			"error: was nil",
		)
	}

	//client with no timout
	client = NewSocketClient(socket2)
	err = client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	verifyFoo(err, foo, t)
}

func TestHTTP(t *testing.T) {
	// server
	go http.ListenAndServe(":8080", &testHandler{t: t})

	client := NewHTTPClient("http://127.0.0.1:8080")

	//client
	var foo answer
	err := client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	verifyFoo(err, foo, t)

	var bad badnews
	err = client.DoJSON(
		&Request{
			Action:        "POST",
			Path:          "/error",
			Payload:       &question{Ping: "junk"},
			Response:      &foo,
			ResponseError: &bad,
		})
	verifyBadNews(err, bad, t)
}

func TestHTTPTimeout(t *testing.T) {
	// server
	go http.ListenAndServe(":8082", &testTimeoutHandler{t: t})

	//client with timeout
	client := NewHTTPClientWithTimeout("http://127.0.0.1:8082", time.Millisecond)
	var foo answer
	err := client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	if err == nil {
		t.Error(
			"client post expected to timeout",
			"error: was nil",
		)
	}

	client = NewHTTPClient("http://127.0.0.1:8080")
	err = client.DoJSON(&Request{"POST", pathString, &question{Ping: "junk"}, &foo, nil})
	verifyFoo(err, foo, t)
}

func verifyFoo(err error, foo answer, t *testing.T) {
	if err != nil {
		t.Error(
			"client post expected to not have error",
			"got error:", err,
		)
	}
	if foo.Pong != "test" {
		t.Error(
			"For", "foo.Pong",
			"expected", "test",
			"got", foo.Pong)
	}
}

func verifyBadNews(err error, bad badnews, t *testing.T) {
	if err == nil {
		t.Error(
			"expected to get an error from POST to /error",
		)
	}
	if bad.Info != "sending an error" {
		t.Error(
			"Bad", "bad.Info",
			"expected", "sending an error",
			"got", bad.Info)
	}
}
