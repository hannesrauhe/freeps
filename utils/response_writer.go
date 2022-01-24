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
	if o.StoredHeaderCode == 0 {
		o.StoredHeaderCode = 200
	}
	o.StoredBody = toWrite
	return len(o.StoredBody), nil
}

func (o *StoreWriter) WriteHeader(statusCode int) {
	o.StoredHeaderCode = statusCode
}

func (o *StoreWriter) Print() {
	fmt.Println("Status: ", o.StoredHeaderCode)
	fmt.Printf("%q\n", o.StoredBody)
}
