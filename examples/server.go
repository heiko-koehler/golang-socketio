package main

import (
	"flag"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/heiko-koehler/golang-socketio"
	"github.com/heiko-koehler/golang-socketio/transport"
)

type Channel struct {
	Channel string `json:"channel"`
}

type Message struct {
	Id      int    `json:"id"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func main() {
	flag.Parse()
	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		glog.Infoln("Connected")

		c.Emit("/message", Message{10, "main", "using emit"})

		c.Join("test")
		c.BroadcastTo("test", "/message", Message{10, "main", "using broadcast"})
	})
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		glog.Infoln("Disconnected")
	})

	server.On("/join", func(c *gosocketio.Channel, channel Channel) string {
		time.Sleep(2 * time.Second)
		glog.Infoln("Client joined to ", channel.Channel)
		return "joined to " + channel.Channel
	})

	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", server)

	glog.Infoln("Starting server...")
	glog.Fatal(http.ListenAndServe(":3811", serveMux))
}
