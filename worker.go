package alpacapi

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"strconv"
)

var (
	ErrInvalidMd5 = errors.New("invalid md5 chksum")
)

// WorkerRequest ...
type WorkerRequest struct {
	ID      uint32
	Config  Config
	Message string
}

// ParseWorkerRequest ...
func ParseWorkerRequest(body []byte) (req WorkerRequest, err error) {
	m := md5.Sum(body[16:])
	if !bytes.Equal(body[:16], m[:]) {
		err = ErrInvalidMd5
		return
	}
	err = json.Unmarshal(body[16:], &req)
	return
}

func (r *WorkerRequest) String() string {
	return "假装 " + r.Config.Role + " 回答 " + r.Message + ", 默认: " + r.Config.Default
}

func (r *WorkerRequest) Pack() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	m := md5.Sum(data)
	ret := make([]byte, len(data)+md5.Size)
	copy(ret[:md5.Size], m[:])
	copy(ret[md5.Size:], data)
	return ret
}

// WorkerReply ...
type WorkerReply struct {
	ID        uint32
	IsPending bool
	Msg       string
}

// ParseWorkerReply...
func ParseWorkerReply(body []byte) (rep WorkerReply, err error) {
	m := md5.Sum(body[16:])
	if !bytes.Equal(body[:16], m[:]) {
		err = ErrInvalidMd5
		return
	}
	err = json.Unmarshal(body[16:], &rep)
	return
}

func (r *WorkerReply) String() string {
	return "ID " + strconv.Itoa(int(r.ID)) + " 回答 " + r.Msg
}

func (r *WorkerReply) Pack() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	m := md5.Sum(data)
	ret := make([]byte, len(data)+md5.Size)
	copy(ret[:md5.Size], m[:])
	copy(ret[md5.Size:], data)
	return ret
}
