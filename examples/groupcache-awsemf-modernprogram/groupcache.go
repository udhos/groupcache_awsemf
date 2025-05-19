package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modernprogram/groupcache/v2"
)

func startGroupcache(workspace *groupcache.Workspace) []*groupcache.Group {

	ttl := 60 * time.Second

	log.Printf("groupcache ttl: %v", ttl)

	//
	// create groupcache pool
	//

	groupcachePort := ":5000"

	myURL := "http://127.0.0.1" + groupcachePort

	log.Printf("groupcache my URL: %s", myURL)

	pool := groupcache.NewHTTPPoolOptsWithWorkspace(workspace, myURL,
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

	const purgeExpired = true

	var list []*groupcache.Group

	names := []string{"files1", "files2"}

	for _, name := range names {

		options := groupcache.Options{
			Workspace:                   workspace,
			Name:                        name,
			PurgeExpired:                purgeExpired,
			CacheBytesLimit:             8000,
			ExpiredKeysEvictionInterval: 2 * time.Minute,
			Getter: groupcache.GetterFunc(
				func(_ /*ctx*/ context.Context, key string, dest groupcache.Sink, _ *groupcache.Info) error {

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
				}),
		}

		cache := groupcache.NewGroupWithWorkspace(options)

		list = append(list, cache)
	}

	return list
}
