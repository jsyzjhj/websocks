package mux

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
)

type Conn struct {
	ID uint32

	group *Group

	mutex sync.Mutex
	buf   []byte
	wait  chan int

	receiveMessageID uint32
	sendMessageID    *uint32
}

func (conn *Conn) Write(p []byte) (n int, err error) {
	mh := &MessageHead{
		Method:    MessageMethodData,
		ConnID:    conn.ID,
		MessageID: conn.SendMessageID(),
		Length:    uint32(len(p)),
	}

	err = conn.group.Send(mh, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (conn *Conn) Read(p []byte) (n int, err error) {
	if len(conn.buf) == 0 {
		//log.Printf("%d buf is 0, waiting", conn.ID)
		<-conn.wait
	}

	conn.mutex.Lock()
	//log.Printf("%d buf: %v", conn.buf)
	n = copy(p, conn.buf)
	conn.buf = conn.buf[n:]
	conn.mutex.Unlock()
	return
}

func (conn *Conn) HandleMessage(mh *MessageHead, data []byte) (err error) {
	//log.Printf("handle message %d %d", mh.ConnID, mh.MessageID)
	for {
		if conn.receiveMessageID == mh.MessageID {
			conn.mutex.Lock()
			conn.buf = append(conn.buf, data...)
			conn.receiveMessageID++
			close(conn.wait)
			conn.wait = make(chan int)
			conn.mutex.Unlock()
			//log.Printf("handled message %d %d", mh.ConnID, mh.MessageID)
			return
		}
		<-conn.wait
	}
	return
}

func (conn *Conn) SendMessageID() (id uint32) {
	id = atomic.LoadUint32(conn.sendMessageID)
	atomic.AddUint32(conn.sendMessageID, 1)
	return
}

func (conn *Conn) Run(c *net.TCPConn) {
	go func() {
		_, err := io.Copy(c, conn)
		if err != nil {
			//log.Printf(err.Error())
		}
	}()

	_, err := io.Copy(conn, c)
	if err != nil {
		//log.Printf(err.Error())
	}

	return
}
