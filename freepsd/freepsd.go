package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/google/gousb"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/freepslisten"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool

func usb() {
	// Initialize a new Context.
	ctx := gousb.NewContext()
	defer ctx.Close()

	// Open any device with a given VID/PID using a convenience function.
	dev, err := ctx.OpenDeviceWithVIDPID(0x1d34, 0x000d)
	if err != nil {
		log.Fatalf("Could not open a device: %v", err)
	}
	defer dev.Close()

	fmt.Println("device: %v", dev)

	dev.Reset()
	// Claim the default interface using a convenience function.
	// The default interface is always #0 alt #0 in the currently active
	// config.
	intf, done, err := dev.DefaultInterface()
	if err != nil {
		log.Fatalf("%s.DefaultInterface(): %v", dev, err)
	}
	defer done()
	fmt.Println("intf: %v", intf)

	// In this interface open endpoint #6 for reading.
	epIn, err := intf.InEndpoint(0x81)
	if err != nil {
		log.Fatalf("%s.InEndpoint(6): %v", intf, err)
	}
	fmt.Println("endp: %v", epIn)

	// And in the same interface open endpoint #5 for writing.
	// epOut, err := intf.OutEndpoint(5)
	// if err != nil {
	// 	log.Fatalf("%s.OutEndpoint(5): %v", intf, err)
	// }

	// Buffer large enough for 10 USB packets from endpoint 6.
	buf := make([]byte, 10*epIn.Desc.MaxPacketSize)
	total := 0
	// Repeat the read/write cycle 10 times.
	for i := 0; i < 10; i++ {
		some := []byte("\x00\x00\x00\x00\x00\x00\x00\x02")
		res, err := dev.Control(0x21, 0x09, 0x0200, 0, some)
		if err != nil {
			fmt.Println("Control returned an error:", err)
		}
		// result = handle.controlMsg(requestType=0x21,
		// 	request= 0x09,
		// 	value= 0x0200,
		// 	buffer="\x00\x00\x00\x00\x00\x00\x00\x02")

		fmt.Printf("Waiting for bytes: %v %v\n", res, some)
		// readBytes might be smaller than the buffer size. readBytes might be greater than zero even if err is not nil.
		readBytes, err := epIn.Read(buf)
		if err != nil {
			fmt.Println("Read returned an error:", err)
		}
		if readBytes == 0 {
			log.Fatalf("IN endpoint 6 returned 0 bytes of data.")
		}
		fmt.Printf("%q\n", readBytes)
		// writeBytes might be smaller than the buffer size if an error occurred. writeBytes might be greater than zero even if err is not nil.
		// writeBytes, err := epOut.Write(buf[:readBytes])
		// if err != nil {
		// 	fmt.Println("Write returned an error:", err)
		// }
		// if writeBytes != readBytes {
		// 	log.Fatalf("IN endpoint 5 received only %d bytes of data out of %d sent", writeBytes, readBytes)
		// }
		// total += writeBytes
	}
	fmt.Printf("Total number of bytes copied: %d\n", total)
}

func main() {
	var configpath, fn, mod, argstring string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&mod, "m", "", "Specify mod to execute directly without starting rest server")
	flag.StringVar(&fn, "f", "", "Specify function to execute in mod")
	flag.StringVar(&argstring, "a", "", "Specify arguments to function as urlencoded string")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Parse()

	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}

	usb()
	doer := freepsdo.NewTemplateMod(cr)

	if mod != "" {
		w := utils.StoreWriter{StoredHeader: make(http.Header)}
		args, _ := url.ParseQuery(argstring)
		doer.ExecuteModWithArgs(mod, fn, args, &w)
		w.Print()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rest := freepslisten.NewRestEndpoint(cr, doer, cancel)
	mqtt := freepslisten.NewMqttSubscriber(cr)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		rest.Shutdown(ctx)
		mqtt.Shutdown()
	}
	log.Printf("Server stopped")
}
