package alpacapi

import (
	"errors"
	"net"
	"time"

	tea "github.com/fumiama/gofastTEA"
)

var (
	ErrTEANilResult  = errors.New("tea encrypt got nil result")
	ErrDataTooBig    = errors.New("data too big")
	ErrWorkerTimeout = errors.New("worker response timeout")
)

// GetReply ...
func (r *WorkerRequest) GetReply(worker *net.UDPAddr, buffersize int, timeout time.Duration, teakey tea.TEA, sumtable [16]uint32) (rep WorkerReply, err error) {
	conn, err := net.DialUDP(worker.Network(), nil, worker)
	if err != nil {
		return
	}
	data := teakey.EncryptLittleEndian(r.Pack(), sumtable)
	if data == nil {
		err = ErrTEANilResult
		return
	}
	if len(data) > buffersize {
		err = ErrDataTooBig
		return
	}
	_, err = conn.Write(data)
	if err != nil {
		return
	}
	ch := make(chan struct{}, 1)
	defer close(ch)
	go func() {
		defer conn.Close()
		n := 0
		buf := make([]byte, buffersize)
		for i := 0; i < 16; i++ {
			n, _, err = conn.ReadFromUDP(buf)
			if err != nil {
				ch <- struct{}{}
				return
			}
			rep, err = ParseWorkerReply(teakey.DecryptLittleEndian(buf[:n], sumtable))
			if err != nil || (rep.ID == r.ID && !rep.IsPending) {
				ch <- struct{}{}
				return
			}
		}
	}()
	select {
	case <-time.After(timeout):
		err = ErrWorkerTimeout
		return
	case <-ch:
		return
	}
}
