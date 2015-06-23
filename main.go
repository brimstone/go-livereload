package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"code.google.com/p/go.net/websocket"
	"github.com/brimstone/go-livereload/static"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/grafov/bcast"
	"gopkg.in/fsnotify.v1"
)

var group *bcast.Group

var address = flag.String("a", "0.0.0.0:8080", "listening address")

type lr_plugin struct {
	Disabled bool   `json:"disabled"`
	Version  string `json:"version"`
}

type lr_command struct {
	Command   string               `json:"command"`
	Protocols []string             `json:"protocols"`
	Ver       string               `json:"ver"`
	Snipver   int                  `json:"snipver"`
	Url       string               `json:"url"`
	Plugins   map[string]lr_plugin `json:"plugins"`
}

func readWebsocket(ws *websocket.Conn, out_chan chan lr_command, err_chan chan error) {
	var message lr_command
	var err error
	for {
		// read in a message
		err = websocket.JSON.Receive(ws, &message)
		if err != nil {
			err_chan <- err
			return
		}
		// send it out of our channel
		out_chan <- message
	}
}

// http://livereload.com/api/protocol/
func watchEvents(ws *websocket.Conn) {
	// when we exit the function, close the socket
	defer ws.Close()
	member := group.Join()
	// start our reader in the background
	clientmsg := make(chan lr_command)
	clienterr := make(chan error)
	go readWebsocket(ws, clientmsg, clienterr)
	for {
		select {
		case val := <-member.In:
			if str, ok := val.(string); ok {
				websocket.Message.Send(ws, "{\"command\":\"reload\", \"path\":\"/"+str+"\"}")
			}
		case data := <-clientmsg:
			if data.Command == "hello" {
				err := websocket.Message.Send(ws, "{\"command\":\"hello\",\"protocols\":[\"http://livereload.com/protocols/official-7\"],\"serverName\":\"go-livereload\"}")
				if err != nil {
					log.Println("Error writing hello: ", err.Error())
				}
			}
		case e := <-clienterr:
			if e.Error() != "EOF" {
				log.Println("Error from websocket: ", e.Error())
			}
			return
		}
	}
}

func watchdirs(watcher *fsnotify.Watcher) {
	defer watcher.Close()
	for {
		select {
		case event := <-watcher.Events:
			// Don't update when the file is hidden
			if filepath.Base(event.Name)[0:1] == "." {
				continue
			}
			// Don't update when the operation isn't a Write
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println(event.Name, "changed")
				group.Send("/" + filepath.Clean(event.Name))
			}
		case err := <-watcher.Errors:
			log.Println("error:", err)
		}
	}
}

func main() {
	flag.Parse()
	group = bcast.NewGroup()
	go group.Broadcasting(0)

	// setup asset handler
	http.Handle("/livereload.js",
		http.FileServer(
			&assetfs.AssetFS{Asset: static.Asset, AssetDir: static.AssetDir, Prefix: "www"}))
	http.Handle("/livereload", websocket.Handler(watchEvents))
	http.Handle("/", http.FileServer(http.Dir(".")))

	// setup filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Unable to start watcher:", err.Error())
		return
	}
	go watchdirs(watcher)
	// add all of the directories under our CWD
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		err = watcher.Add(path)
		if err != nil {
			log.Println("Unable to watch current directory:", err.Error())
			return nil
		}
		return nil
	})

	log.Println("Starting http server on " + *address)
	http.ListenAndServe(*address, nil)
}
