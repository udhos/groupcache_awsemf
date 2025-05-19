package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mailgun/groupcache/v2"
)

func startGroupcache() *groupcache.Group {

	ttl := 60 * time.Second

	log.Printf("groupcache ttl: %v", ttl)

	//
	// create groupcache pool
	//

	groupcachePort := ":5000"

	myURL := "http://127.0.0.1" + groupcachePort

	log.Printf("groupcache my URL: %s", myURL)

	pool := groupcache.NewHTTPPoolOpts(myURL,
		&groupcache.HTTPPoolOptions{})

	//
	// start groupcache server
	//

	serverGroupCache := &http.Server{Addr: groupcachePort, Handler: pool}

	go func() {
		log.Printf("groupcache server: listening on %s", groupcachePort)
		err := serverGroupCache.ListenAndServe()
		log.Printf("groupcache server: exited: %v", err)
	}()

	pool.Set(myURL)

	//
	// create cache
	//

	getter := groupcache.GetterFunc(
		func(_ /*ctx*/ context.Context, key string, dest groupcache.Sink) error {

			var data []byte

			if strings.HasPrefix(key, "fake-") {
				data = bytes.Repeat([]byte{'x'}, 3000)
			} else {
				var errFile error
				data, errFile = os.ReadFile(key)
				if errFile != nil {
					return errFile
				}
			}

			log.Printf("getter: loading: key:%s size:%d ttl:%v",
				key, len(data), ttl)

			time.Sleep(50 * time.Millisecond)

			expire := time.Now().Add(ttl)
			return dest.SetBytes(data, expire)
		})

	cache := groupcache.NewGroup("files", 8000, getter)

	return cache
}
