package main

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

func readTemplate(path string) *template.Template {
	dat, _ := ioutil.ReadFile(path)
	return template.Must(template.New(path).Parse(string(dat)))
}

func isConnectionLost(c *gin.Context) bool {
	done := false

	select {
	case <-c.Writer.CloseNotify():
		done = true
	default:
	}

	return done
}

func handle(r *gin.Engine) {

	indexTmplPath := "index.html"
	r.SetHTMLTemplate(readTemplate(indexTmplPath))

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, indexTmplPath, map[string]interface{}{})
	})

	r.GET("/ping", func(c *gin.Context) {

		h := c.Writer.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("Content-Type", "text/event-stream")
		h.Set("X-Accel-Buffering", "no")

		// data can be a primitive like a string, an integer or a float
		ticker := time.NewTicker(1 * time.Second)

		for range ticker.C {
			if isConnectionLost(c) {
				fmt.Println("Event-connection is lost")
				return
			}

			error := sse.Encode(c.Writer, sse.Event{
				Event: "message",
				Data:  time.Now().Unix(),
			})

			if error != nil {
				log.Println(error)
				return
			}

			c.Writer.Flush()
		}
	})
}

func main() {
	router := gin.Default()
	router.Use(cors.Default())
	handle(router)

	upg, err := tableflip.New(tableflip.Options{})
	if err != nil {
		panic(err)
	}
	defer upg.Stop()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			err := upg.Upgrade()
			if err != nil {
				log.Println("Upgrade failed:", err)
				continue
			}

			log.Println("Upgrade succeeded")
		}
	}()

	ln, err := upg.Fds.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatalln("Can't listen:", err)
	}

	server := &http.Server{
		Addr:           ":8080",
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go server.Serve(ln)

	if err := upg.Ready(); err != nil {
		panic(err)
	}
	<-upg.Exit()

	time.AfterFunc(30*time.Second, func() {
		os.Exit(1)
	})

	_ = server.Shutdown(context.Background())
}
