package dlna

import (
	"net"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
)

type OpDLNA struct {
}

func (o *OpDLNA) DiscoverServers(ctx *base.Context) *base.OperatorIO {
	conn, err := net.Dial("udp", "239.255.255.250:1900")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error connecting to UPnP multicast address: %v", err)
	}
	defer conn.Close()

	searchRequest := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: ssdp:all\r\n" +
		"\r\n"

	_, err = conn.Write([]byte(searchRequest))
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error sending search request: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error reading search response: %v", err)
	}

	response := string(buf[:n])
	serverList := make([]string, 0)
	for _, line := range strings.Split(response, "\r\n") {
		if strings.HasPrefix(line, "LOCATION: ") {
			serverList = append(serverList, line[10:])
		}
	}
	return base.MakeObjectOutput(serverList)
}
