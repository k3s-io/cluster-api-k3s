/*
Copyright 2026 The Kubernetes Authors.

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

package runtimeclient

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestErrorFromHTTPResponseBoundsAndRedactsResponse(t *testing.T) {
	const (
		bodySentinel   = "body-sentinel-secret"
		statusSentinel = "remote-sentinel-secret"
		headerSentinel = "header-sentinel-secret"
	)
	countingBody := &countingReadCloser{
		Reader: bytes.NewReader(append(
			bytes.Repeat([]byte("x"), int(maxErrorResponseBodyBytes)+1024),
			[]byte(bodySentinel)...,
		)),
	}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 " + statusSentinel,
		Header:     http.Header{"X-Remote-Secret": {headerSentinel}},
		Body:       countingBody,
	}

	err := errorFromHTTPResponse(resp)
	if err == nil {
		t.Fatal("errorFromHTTPResponse() returned nil")
	}
	if countingBody.bytesRead > maxErrorResponseBodyBytes {
		t.Fatalf("errorFromHTTPResponse() read %d bytes, want at most %d", countingBody.bytesRead, maxErrorResponseBodyBytes)
	}
	for _, want := range []string{"500", "Internal Server Error"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("errorFromHTTPResponse() error %q does not contain %q", err, want)
		}
	}
	for _, sentinel := range []string{bodySentinel, statusSentinel, headerSentinel} {
		if strings.Contains(err.Error(), sentinel) {
			t.Errorf("errorFromHTTPResponse() error %q contains remote sentinel %q", err, sentinel)
		}
	}
}

func TestErrorFromHTTPResponseRedactsBodyReadError(t *testing.T) {
	const readErrorSentinel = "body-read-error-sentinel-secret"
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       errorReadCloser{err: errors.New(readErrorSentinel)},
	}

	err := errorFromHTTPResponse(resp)
	const want = "http call failed: got response with status code 503 (Service Unavailable); failed to discard bounded response body"
	if err == nil {
		t.Fatal("errorFromHTTPResponse() returned nil")
	}
	if err.Error() != want {
		t.Fatalf("errorFromHTTPResponse() error = %q, want %q", err, want)
	}
	if strings.Contains(err.Error(), readErrorSentinel) {
		t.Errorf("errorFromHTTPResponse() error %q contains read error sentinel", err)
	}
}

func TestErrorFromHTTPResponseOmitsEmptyStatusText(t *testing.T) {
	resp := &http.Response{
		StatusCode: 599,
		Status:     "599 remote-reason-sentinel-secret",
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	err := errorFromHTTPResponse(resp)
	const want = "http call failed: got response with status code 599"
	if err == nil {
		t.Fatal("errorFromHTTPResponse() returned nil")
	}
	if err.Error() != want {
		t.Fatalf("errorFromHTTPResponse() error = %q, want %q", err, want)
	}
}

type countingReadCloser struct {
	io.Reader
	bytesRead int64
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytesRead += int64(n)
	return n, err
}

func (*countingReadCloser) Close() error {
	return nil
}

type errorReadCloser struct {
	err error
}

func (r errorReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (errorReadCloser) Close() error {
	return nil
}
