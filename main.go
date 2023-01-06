package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
	"math/rand"
	"strconv"

	"github.com/urfave/cli"
)

var (
	mux           = http.NewServeMux()
	sessionCookie = "session"
	waitGroup     = sync.WaitGroup{}
	started       = time.Now()
	requests      = 0
)

type (
	Content struct {
		Title           string
		Version         string
		Hostname        string
		RefreshInterval string
		ExpireInterval  string
		Metadata        string
		SkipErrors      bool
		ShowVersion     bool
		ContColor       string
		Pets            string
		RemoveInterval  string
	}

	Ping struct {
		Instance  string `json:"instance"`
		Version   string `json:"version"`
		Metadata  string `json:"metadata,omitempty"`
		RequestID string `json:"request_id,omitempty"`
		ContColor string `json:"contColor"`
		Pets      string `json:"pets"`
		RandomNumber string `json:"randomNumber"`
	}
)

// Function to get the hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return hostname
}

// Function to get the version
func getVersion() string {
	ver := os.Getenv("VERSION")
	if ver == "" {
		ver = "0.1"
	}

	return ver
}

func randomNumber() int {
	rand.Seed(time.Now().UnixNano())
    return rand.Intn(10) + 1
}


func loadTemplate(filename string) (*template.Template, error) {
	return template.ParseFiles(filename)
}

func getMetadata() string {
	return os.Getenv("METADATA")
}

// Function to start the index page
func index(w http.ResponseWriter, r *http.Request) {
	waitGroup.Add(1)
	defer waitGroup.Done()
	remote := r.RemoteAddr

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		remote = forwarded
	}

	log.Printf("request from %s\n", remote)

	t, err := loadTemplate("templates/index.html.tmpl")
	if err != nil {
		fmt.Printf("error loading template: %s\n", err)
		return
	}

	title := os.Getenv("TITLE")
	if title == "" {
		title = "Single Penguin"
	}

	hostname := getHostname()
	refreshInterval := os.Getenv("REFRESH_INTERVAL")
	if refreshInterval == "" {
		refreshInterval = "1000"
	}

	expireInterval := os.Getenv("EXPIRE_INTERVAL")
	if expireInterval == "" {
		expireInterval = "10"
	}

	removeInterval := os.Getenv("REMOVE_INTERVAL")
	if removeInterval == "" {
		removeInterval = "20"
	}

	cnt := &Content{
		Title:           title,
		Version:         getVersion(),
		Hostname:        hostname,
		RefreshInterval: refreshInterval,
		ExpireInterval:  expireInterval,
		RemoveInterval:  removeInterval,
		Metadata:        getMetadata(),
		SkipErrors:      os.Getenv("SKIP_ERRORS") != "",
		ShowVersion:     os.Getenv("SHOW_VERSION") != "",
	}

	t.Execute(w, cnt)
}


func fail(w http.ResponseWriter, r *http.Request) {
	waitGroup.Add(1)
	defer waitGroup.Done()

	// add a false delay
	time.Sleep(2 * time.Second)

	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusInternalServerError)
}

func missing(w http.ResponseWriter, r *http.Request) {
	waitGroup.Add(1)
	defer waitGroup.Done()

	// add a false delay
	time.Sleep(2 * time.Second)

	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusNotFound)
}

func ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	waitGroup.Add(1)
	defer waitGroup.Done()

	w.Header().Set("Connection", "close")

	hostname := getHostname()

	colors := [10]string{"red","orange","yellow","olive","green","teal","blue","violet","purple","pink"}

	var contColor string = colors[globalrandom-1]

	pets := os.Getenv("PETS")
	if pets == "" {
		pets = "penguin"
	}

	//myRandomNum := os.Getenv("RANDOMPENGNUMBER")

	p := Ping{
		Instance:  hostname,
		Version:   getVersion(),
		Metadata:  getMetadata(),
		ContColor: contColor,
		Pets:      pets,
		RandomNumber:	strconv.Itoa(globalrandom),
	}

	requestID := r.Header.Get("X-Request-Id")
	if requestID != "" {
		p.RequestID = requestID
	}

	current, _ := r.Cookie(sessionCookie)
	if current == nil {
		current = &http.Cookie{
			Name:    sessionCookie,
			Value:   fmt.Sprintf("%d", time.Now().UnixNano()),
			Path:    "/",
			Expires: time.Now().AddDate(0, 0, 1),
			MaxAge:  86400,
		}
	}

	http.SetCookie(w, current)

	if err := json.NewEncoder(w).Encode(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func waitForDone(ctx context.Context) {
	waitGroup.Wait()
	ctx.Done()
}

func counter(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		h.ServeHTTP(w, r)
	})
}

// Global Variable for the random number
var globalrandom int = randomNumber()

func main() {
	app := cli.NewApp()
	app.Name = "single-penguin"
	app.Usage = "single-penguin application"
	app.Version = "1.4.1"
	app.Author = "@oskapt"
	app.Email = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "listen-addr, l",
			Usage: "listen address",
			Value: ":8080",
		},
		cli.StringFlag{
			Name:  "tls-cert, c",
			Usage: "tls certificate",
			Value: "",
		},
		cli.StringFlag{
			Name:  "tls-key, k",
			Usage: "tls certificate key",
			Value: "",
		},
	}
	app.Action = func(c *cli.Context) error {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
		mux.Handle("/demo", counter(http.HandlerFunc(ping)))
		mux.Handle("/fail", counter(http.HandlerFunc(fail)))
		mux.Handle("/404", counter(http.HandlerFunc(missing)))
		mux.Handle("/", counter(http.HandlerFunc(index)))

		hostname := getHostname()
		listenAddr := c.String("listen-addr")
		tlsCert := c.String("tls-cert")
		tlsKey := c.String("tls-key")

		srv := &http.Server{
			Handler:      mux,
			Addr:         listenAddr,
			WriteTimeout: time.Second * 10,
			ReadTimeout:  time.Second * 10,
		}

		fmt.Printf("instance: %s\n", hostname)
		fmt.Printf("listening on %s\n", listenAddr)
		fmt.Printf("Our Random Penguin Number:  %s\n", strconv.Itoa(globalrandom))

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)

		go func() {
			select {
			case <-ch:
				log.Println("stopping")
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()

				waitForDone(ctx)

				if err := srv.Shutdown(ctx); err != nil {
					log.Fatal(err)
				}
			}
		}()

		var err error
		if tlsCert != "" && tlsKey != "" {
			err = srv.ListenAndServeTLS(tlsCert, tlsKey)
		} else {
			err = srv.ListenAndServe()
		}

		return err
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
