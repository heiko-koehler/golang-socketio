package gosocketio

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/heiko-koehler/golang-socketio/protocol"
	"github.com/heiko-koehler/golang-socketio/transport"
)

const (
	queueBufferSize = 500
)

var (
	ErrorWrongHeader = errors.New("Wrong header")
)

/**
engine.io header to send or receive
*/
type Header struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"`
	PingTimeout  int      `json:"pingTimeout"`
}

/**
socket.io connection handler

use IsAlive to check that handler is still working
use Dial to connect to websocket
use In and Out channels for message exchange
Close message means channel is closed
ping is automatic
*/
type Channel struct {
	conn transport.Connection

	in     chan *protocol.Message
	out    chan string
	header Header

	alive     bool
	aliveLock sync.Mutex

	ack ackProcessor

	server           *Server
	ip               string
	requestHeader    http.Header
	sequentialInLoop bool
}

/**
create channel, map, and set active
*/
func (c *Channel) initChannel() {
	//TODO: queueBufferSize from constant to server or client variable
	c.in = make(chan *protocol.Message, queueBufferSize)
	c.out = make(chan string, queueBufferSize)
	c.ack.resultWaiters = make(map[int](chan string))
	c.alive = true
}

/**
Get id of current socket connection
*/
func (c *Channel) Id() string {
	return c.header.Sid
}

/**
Checks that Channel is still alive
*/
func (c *Channel) IsAlive() bool {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()

	return c.alive
}

/**
Close channel
*/
func closeChannel(c *Channel, m *methods, args ...interface{}) error {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()

	if !c.alive {
		//already closed
		return nil
	}

	c.conn.Close()
	c.alive = false

	// close message in-channel
	close(c.in)

	//clean outloop
	for len(c.out) > 0 {
		<-c.out
	}
	c.out <- protocol.CloseMessage

	m.callLoopEvent(c, OnDisconnection)

	overfloodedLock.Lock()
	delete(overflooded, c)
	overfloodedLock.Unlock()

	return nil
}

//incoming messages loop, puts incoming messages to In channel
func inLoop(c *Channel, m *methods) error {
	glog.Infoln("Start in loop for channel", c)
	defer func() {
		glog.Infoln("Exit in loop for channel", c)
	}()
	for {
		pkg, err := c.conn.GetMessage()
		if err != nil {
			glog.Errorf("Failed to get message: %s", err)
			return closeChannel(c, m, err)
		}
		msg, err := protocol.Decode(pkg)
		if err != nil {
			glog.Errorf("Failed to decode message: %s", err)
			closeChannel(c, m, protocol.ErrorWrongPacket)
			return err
		}

		glog.V(3).Infof("Received message %q", msg.Method)
		switch msg.Type {
		case protocol.MessageTypeOpen:
			if err := json.Unmarshal([]byte(msg.Source[1:]), &c.header); err != nil {
				glog.Errorf("Failed to decode message source: %s", err)
				closeChannel(c, m, ErrorWrongHeader)
			}
			m.callLoopEvent(c, OnConnection)
		case protocol.MessageTypePing:
			c.out <- protocol.PongMessage
		case protocol.MessageTypePong:
		default:
			if c.sequentialInLoop {
				glog.V(5).Infof("Process %q sequentially", msg.Method)
				c.in <- msg
			} else {
				glog.V(5).Infof("Process %q asynchronously", msg.Method)
				go m.processIncomingMessage(c, msg)
			}
		}
	}
	return nil
}

// worker for processing messages
func workerLoop(c *Channel, m *methods) error {
	glog.Infoln("Start worker loop for channel", c)
	defer func() {
		glog.Infoln("Exit worker loop for channel", c)
	}()
	for {
		select {
		case msg := <-c.in:
			if msg == nil {
				return nil
			}
			m.processIncomingMessage(c, msg)
		}
	}
}

var overflooded map[*Channel]struct{} = make(map[*Channel]struct{})
var overfloodedLock sync.Mutex

func AmountOfOverflooded() int64 {
	overfloodedLock.Lock()
	defer overfloodedLock.Unlock()

	return int64(len(overflooded))
}

/**
outgoing messages loop, sends messages from channel to socket
*/
func outLoop(c *Channel, m *methods) error {
	glog.Infoln("Start out loop for channel", c)
	defer func() {
		glog.Infoln("Exit out loop for channel", c)
	}()
	for {
		outBufferLen := len(c.out)
		if outBufferLen >= queueBufferSize-1 {
			glog.Errorf("Output buffer to small")
			return closeChannel(c, m, ErrorSocketOverflood)
		} else if outBufferLen > int(queueBufferSize/2) {
			overfloodedLock.Lock()
			overflooded[c] = struct{}{}
			overfloodedLock.Unlock()
		} else {
			overfloodedLock.Lock()
			delete(overflooded, c)
			overfloodedLock.Unlock()
		}

		msg := <-c.out
		if msg == protocol.CloseMessage {
			return nil
		}

		err := c.conn.WriteMessage(msg)
		if err != nil {
			glog.Errorf("Failed to write message: %s", err)
			return closeChannel(c, m, err)
		}
	}
	return nil
}

/**
Pinger sends ping messages for keeping connection alive
*/
func pinger(c *Channel) {
	for {
		interval, _ := c.conn.PingParams()
		time.Sleep(interval)
		if !c.IsAlive() {
			return
		}

		c.out <- protocol.PingMessage
	}
}
