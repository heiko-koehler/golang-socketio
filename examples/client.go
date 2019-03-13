package main

import (
	"flag"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/nutanix/golang-socketio"
	"github.com/nutanix/golang-socketio/transport"
)

type Channel struct {
	Channel string `json:"channel"`
}

type Message struct {
	Id      int    `json:"id"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func sendJoin(c *gosocketio.Client) {
	glog.Infoln("Acking /join")
	result, err := c.Ack("/join", Channel{"main"}, time.Second*5)
	if err != nil {
		glog.Fatal(err)
	} else {
		glog.Infoln("Ack result to /join: ", result)
	}
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	c, err := gosocketio.Dial2(
		gosocketio.GetUrl("localhost", 3811, false),
		transport.GetDefaultWebsocketTransport())
	if err != nil {
		glog.Fatal(err)
	}

	err = c.On("/message", func(h *gosocketio.Channel, args Message) {
		glog.Infoln("--- Got chat message: ", args)
	})
	if err != nil {
		glog.Fatal(err)
	}

	err = c.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		glog.Infoln("Disconnected")
	})
	if err != nil {
		glog.Fatal(err)
	}

	err = c.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		glog.Infoln("Connected")
	})
	if err != nil {
		glog.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	go sendJoin(c)
	go sendJoin(c)
	go sendJoin(c)
	go sendJoin(c)
	go sendJoin(c)

	time.Sleep(60 * time.Second)
	c.Close()

	glog.Infoln(" [x] Complete")
}
