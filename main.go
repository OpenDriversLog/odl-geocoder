// Package main starts a Geocoder - used to chain Geocoding requests between as many services as you want.
package main

import (
	"flag"
	"fmt"
	"github.com/Compufreak345/dbg"
	"github.com/Compufreak345/manners"
	"github.com/julienschmidt/httprouter"
	"github.com/OpenDriversLog/odl-geocoder/json"
	"github.com/OpenDriversLog/odl-geocoder/utils"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"
	"net/url"
	js "encoding/json"
)

var router = httprouter.New()

const TAG = "GC"

func main() {
	var err error
	port := flag.Int("port", 6091, "Port for the server to listen")
	debug := flag.Bool("debug", false, "Debug mode enabled")

	flag.Parse()
	utils.Debug = *debug
	dbg.I(TAG, "Initialised with port : %d", *port)
	fnr := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		defer func() {
			if err := recover(); err != nil {
				dbg.E(TAG, "panic in reverse: %v for request : %v", err, dbg.GetRequest(r))
				http.Error(w, http.StatusText(500), 500)
			}
		}()
		w.Write(GetReverseResult(r, ps))
	}
	router.GET("/reverse/:userId/:key/:reqId/:lat/:lng", fnr)
	router.POST("/reverse/:userId/:key/:reqId/:lat/:lng", fnr)
	fnf := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		defer func() {
			if err := recover(); err != nil {
				dbg.E(TAG, "panic in reverse: %v for request : %v", err, dbg.GetRequest(r))
				http.Error(w, http.StatusText(500), 500)
			}
		}()
		w.Write(GetForwardResult(r, ps))
	}
	router.GET("/forward/:userId/:key/:reqId/:addr", fnf)
	router.POST("/forward/:userId/:key/:reqId/:addr", fnf)
	router.GET("/reparseChain", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		b, err := ioutil.ReadFile("Providers.json")
		if err != nil {
			dbg.E(TAG, "Error reading Providers.json : ", err)
			http.Error(w, "Error reading Providers.json", 500)
			return
		}
		err = utils.ParseProviders(b)
		if err != nil {
			dbg.E(TAG, "Error parsing Providers.json : ", err)
			http.Error(w, "Error parsing Providers.json", 500)
			return
		}
		w.Write([]byte("Success!"))
	})
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(404), 404)
	})
	b, err := ioutil.ReadFile("Providers.json")
	if err != nil {
		dbg.E(TAG, "Error reading Providers.json : ", err)
	}
	err = utils.ParseProviders(b)
	if err != nil {
		dbg.E(TAG, "Error parsing Providers.json : ", err)
	}
	uri := fmt.Sprintf(":%d", *port)
	dbg.I(TAG, "Starting server with uri : %s", uri)
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, os.Kill)
		<-sigchan
		dbg.I(TAG, "Shutting down...")
		utils.SaveProviders(true)
		manners.Close()
	}()

	go func() {
		for {
			time.Sleep(15*time.Second)
			utils.SaveProviders(false)
		}
	}()
	err = manners.ListenAndServe(uri, router)
	if err != nil {
		dbg.E(TAG, "Error starting server : ", err)
	}
}

func GetReverseResult(r *http.Request, ps httprouter.Params) (res []byte) {

	var err error
	res, err = json.GetJsonReverseGeoCode(ps.ByName("lat"), ps.ByName("lng"), ps.ByName("reqId"), r.FormValue("dontChain") != "", ps.ByName("userId"))
	if err != nil {
		dbg.E(TAG, "Error calling json.GetJsonReverseGeoCode : ", err)
	}
	return
}

func GetForwardResult(r *http.Request, ps httprouter.Params) (res []byte) {

	var err error
	var a string
	a, err = url.QueryUnescape(ps.ByName("addr"))
	if err != nil {
		dbg.E(TAG,"Unable to unescape addr : ", err)
		res, _ = js.Marshal(json.GetErrorGeoCodeResponse("Could not parse address",ps.ByName("reqId")))
		return
	}
	res, err = json.GetJsonGeoCode(a,  ps.ByName("reqId"), r.FormValue("dontChain") != "", ps.ByName("userId"))
	if err != nil {
		dbg.E(TAG, "Error calling json.GetJsonGeoCode : ", err)
	}
	return
}