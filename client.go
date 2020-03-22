package gosocketio

import (
	"strconv"

	"github.com/heiko-koehler/golang-socketio/transport"
)

const (
	webSocketProtocol       = "ws://"
	webSocketSecureProtocol = "wss://"
	socketioUrl             = "/socket.io/?EIO=3&transport=websocket"
)

/**
Socket.io client representation
*/
type Client struct {
	methods
	Channel
}

/**
Get ws/wss url by host and port
*/
func GetUrl(host string, port int, secure bool) string {
	var prefix string
	if secure {
		prefix = webSocketSecureProtocol
	} else {
		prefix = webSocketProtocol
	}
	return prefix + host + ":" + strconv.Itoa(port) + socketioUrl
}

/**
connect to host and initialise socket.io protocol

The correct ws protocol url example:
ws://myserver.com/socket.io/?EIO=3&transport=websocket

You can use GetUrlByHost for generating correct url
*/
func Dial(url string, tr transport.Transport) (*Client, error) {
	c := &Client{}
	c.initChannel()
	c.initMethods()

	var err error
	c.conn, err = tr.Connect(url)
	if err != nil {
		return nil, err
	}

	go workerLoop(&c.Channel, &c.methods)
	go inLoop(&c.Channel, &c.methods)
	go outLoop(&c.Channel, &c.methods)
	go pinger(&c.Channel)

	return c, nil
}

// Dial2 - Similar to Dial, but set sequentialInLoop to true in Channel
// this will cause incoming message handling to be serialized.
func Dial2(url string, tr transport.Transport) (*Client, error) {
	c, err := Dial(url, tr)
	if err == nil {
		c.Channel.sequentialInLoop = true
	}
	return c, err
}

/**
Close client connection
*/
func (c *Client) Close() {
	closeChannel(&c.Channel, &c.methods)
}
