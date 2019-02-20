package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/fvbock/endless"
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
				Data:  strconv.Itoa(int(time.Now().Unix())) + " v1",
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

	if err := endless.ListenAndServe("localhost:8080", router); err != nil {
		log.Fatalln(err)
	}
}
