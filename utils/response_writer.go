package utils

import (
	"fmt"
	"net/http"
)

type StoreWriter struct {
	StoredHeader     http.Header
	StoredBody       []byte
	StoredHeaderCode int
}

func (o *StoreWriter) Header() http.Header {
	return o.StoredHeader
}

func (o *StoreWriter) Write(toWrite []byte) (int, error) {
	o.StoredBody = toWrite
	return len(o.StoredBody), nil
}

func (o *StoreWriter) WriteHeader(statusCode int) {
	o.StoredHeaderCode = statusCode
}

func (o *StoreWriter) Print() {
	fmt.Printf("Status: %v\n%q\n", o.StoredHeaderCode, o.StoredBody)
}
