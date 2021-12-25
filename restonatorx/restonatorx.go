package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

func ExecHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cmd := exec.Command("./scripts/"+vars["script"], vars["arg"])
	stdout, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nError: %v", vars["script"], vars["arg"], string(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nOutput: %v", vars["script"], vars["arg"], string(stdout))
	}
}

func DenonHandler(w http.ResponseWriter, r *http.Request) {
	denon_address := "192.168.170.26"
	c := http.Client{}
	vars := mux.Vars(r)
	var cmd string

	switch vars["function"] {
	case "on":
		cmd = "PutSystem_OnStandby/ON"
	case "off":
		cmd = "PutSystem_OnStandby/STANDBY"
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data_url := "http://" + denon_address + "/MainZone/index.put.asp"
	data := url.Values{}
	data.Set("cmd0", cmd)

	data_resp, err := c.PostForm(data_url, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "DenonHandler\nParameters: %v\nError: %v", vars, string(err.Error()))
		return
	}
	fmt.Fprintf(w, "Denon: %v, %v", vars, data_resp)
}

type FritzHandler struct {
	fconf freepslib.FBconfig
}

func (fh FritzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	f, err := freepslib.NewFreepsLib(&fh.fconf)
	if err != nil {
		fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError on freepslib-init: %v", vars, string(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fn := vars["function"]
	if fn == "getdevicelistinfos" {
		devl, err := f.GetDeviceList()
		if err != nil {
			fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError when getting device list: %v", vars, string(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jsonbytes, err := json.MarshalIndent(devl, "", "  ")
		if err != nil {
			fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError when creating JSON reponse: %v", vars, string(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(jsonbytes)
		return
	}

	dev := vars["device"]
	arg := make(map[string]string)
	for key, value := range r.URL.Query() {
		arg[key] = value[0]
	}
	if fn == "wakeup" {
		log.Printf("Waking Up %v", dev)
		err = f.WakeUpDevice(dev)
	} else {
		err = f.HomeAutoSwitch(fn, dev, arg)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError: %v", vars, string(err.Error()))
		return
	}
	fmt.Fprintf(w, "Fritz: %v, %v, %v", fn, dev, arg)
}

func main() {
	var configpath string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	// verb := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	conf := freepslib.DefaultConfig
	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.ReadSectionWithDefaults("freepslib", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	fh := FritzHandler{conf}

	r := mux.NewRouter()
	r.HandleFunc("/exec/{script:[a-z0-9_]+}/{arg:[a-z0-9_]+}", ExecHandler)
	r.HandleFunc("/script/{script:[a-z0-9_]+}/{arg:[a-z0-9_]+}", ExecHandler)
	r.HandleFunc("/denon/{function}", DenonHandler)

	r.PathPrefix("/fritz/{function}").Handler(fh)
	r.PathPrefix("/fritz/{function}/{device}").Handler(fh)

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Println("Starting Server")
	log.Fatal(srv.ListenAndServe())
}
