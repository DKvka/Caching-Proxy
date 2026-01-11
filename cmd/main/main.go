package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
)

type Response struct {
	count     uint64
	data      []byte
	cacheFile *os.File

	sync.RWMutex
}

type Cache struct {
	cache map[string]*Response
	sync.Mutex
}

var L1Cache = make(map[string]*Response)
var L2Cache = make(map[string]*Response)

func main() {
	port := flag.String("port", ":9921", "port to run proxy on")
	origin := flag.String("origin", "example.com", "origin to proxy requests to")
	flag.Parse()

	originURL, err := url.Parse(*origin)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", createHandlerToDest(originURL))

	log.Fatal(http.ListenAndServe(*port, nil))
}

func createHandlerToDest(URL *url.URL) func(http.ResponseWriter, *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(URL)
	return func(w http.ResponseWriter, r *http.Request) {
		if resp, ok := L1Cache[r.URL.String()]; ok {
			resp.RLock()
			defer resp.RUnlock()

			resp.count++

			log.Println("L1 Cache hit - responding")
			w.Write(resp.data)
			return
		}

		if resp, ok := L2Cache[r.URL.String()]; ok {
			resp.Lock()
			defer resp.Unlock()

			resp.count++
			if resp.count > 10 {
				L1Cache
			}

			log.Println("L2 Cache hit - responding")
			resp, err := io.ReadAll(resp.cacheFile)
			if err != nil {
				log.Println(err)
			}

			w.Write(resp)
			return
		}

		log.Println("Cache miss - Roundtripping request to origin")
		proxy.ServeHTTP(w, r)
	}
}
