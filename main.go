package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/grafov/bcast"
	"golang.org/x/net/websocket"
	"gopkg.in/fsnotify.v1"
)

var group *bcast.Group

var address = flag.String("a", "127.0.0.1:8080", "listening address")

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
	lastChange := time.Now()
	for {
		select {
		case event := <-watcher.Events:
			// Don't update when the time changes
			if time.Since(lastChange) < time.Second {
				continue
			}
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

// stolen from github.com/smartystreets/goconvey/goconvey.go
func browserCmd() (string, bool) {
	browser := map[string]string{
		"darwin": "open",
		"linux":  "xdg-open",
		"win32":  "start",
	}
	cmd, ok := browser[runtime.GOOS]
	return cmd, ok
}

func launchBrowser(host string) {
	browser, ok := browserCmd()
	if !ok {
		log.Printf("Skipped launching browser for this OS: %s", runtime.GOOS)
		return
	}

	log.Printf("Launching browser on %s", host)
	url := fmt.Sprintf("http://%s", host)
	cmd := exec.Command(browser, url)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
		log.Println(string(output))
	}
}

func main() {
	flag.Parse()
	group = bcast.NewGroup()
	go group.Broadcasting(0)

	// setup asset handler
	http.HandleFunc("/livereload.js", livereloadjs)
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
	go launchBrowser(*address)
	http.ListenAndServe(*address, nil)
}
