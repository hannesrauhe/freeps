package utils

import (
	"fmt"
	"net/http"
)

type StoreWriter struct {
	StoredHeader     http.Header
	storedBody       []byte
	storedHeaderCode int
}

func (o *StoreWriter) Header() http.Header {
	return o.StoredHeader
}

func (o *StoreWriter) Write(toWrite []byte) (int, error) {
	if o.storedHeaderCode == 0 {
		o.storedHeaderCode = 200
	}
	o.storedBody = toWrite
	return len(o.storedBody), nil
}

func (o *StoreWriter) WriteHeader(statusCode int) {
	o.storedHeaderCode = statusCode
}

func (o *StoreWriter) Print() {
	str := fmt.Sprintf("%q", o.storedBody)
	fmt.Printf("Status %v\n %v\n", o.storedHeaderCode, str)
}
