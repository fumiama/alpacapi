package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	para "github.com/fumiama/go-hide-param"
	tea "github.com/fumiama/gofastTEA"

	"github.com/fumiama/alpacapi"
)

// prompt: role, default, message
const prompt = `Do following task in Chinese, no interaction, no conversation. Pretend to be a Chinese %s. You got a message. Reply with just one sentence. No imaging User's reply, no explain why. If you don't know how to reply, just say "%s".
User: %s
You(last line):`

var globalid uint32

func main() {
	addr := flag.String("l", "0.0.0.0:31471", "listening endpoint")
	mpth := flag.String("m", "/dataset/Alpaca-ggml/13B-ggml-model-q4_1.bin", "alpaca ggml model path")
	threadcnt := flag.Uint("t", 24, "use threads count")
	llamapath := flag.String("p", "./src/llama.cpp/build/bin/main", "llama.cpp main path")
	bufsz := flag.Uint("b", 4096, "udp buffer size")
	sumtablepath := flag.String("s", "sumtable.bin", "tea sumtable file")
	flag.Parse()
	if len(os.Args) <= 1 {
		panic("must give tea key (16 bytes hex string)")
	}
	k, err := hex.DecodeString(os.Args[1])
	if err != nil {
		panic(err)
	}
	para.Hide(1)
	tk := tea.NewTeaCipherLittleEndian(k)
	data, err := os.ReadFile(*sumtablepath)
	if err != nil {
		panic(err)
	}
	var sumtable [16]uint32
	for i := range sumtable {
		sumtable[i] = binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
	}
	listener, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort(*addr)))
	if err != nil {
		panic(err)
	}
	logrus.Infoln("listening on", listener.LocalAddr())
	buf := make([]byte, *bufsz)
	bufd := make([]byte, *bufsz+16)
	for {
		n, from, err := listener.ReadFromUDP(buf)
		if err != nil {
			break
		}
		logrus.Infoln("recv from", from)
		if n == 0 {
			logrus.Infoln("skip empty body")
			continue
		}
		a, b := tk.DecryptLittleEndianTo(buf[:n], sumtable, bufd)
		if a >= b || b == 0 || b-a <= 16+2 {
			logrus.Infoln("skip dirty udp body")
			continue
		}
		body := bytes.TrimSuffix(bufd[a:b], []byte{' '})
		req, err := alpacapi.ParseWorkerRequest(body)
		if err != nil {
			logrus.Infoln("parse body err:", err)
			continue
		}
		logrus.Infoln("get request:", &req)
		repl := alpacapi.WorkerReply{
			ID:        atomic.AddUint32(&globalid, 1),
			IsPending: true,
			Msg:       "pending...",
		}
		_, err = listener.WriteToUDP(tk.EncryptLittleEndian(repl.Pack(), sumtable), from)
		if err != nil {
			logrus.Infoln("send pending err:", err)
			continue
		}
		cmd := exec.Command(
			*llamapath, "-m", *mpth, "-p",
			fmt.Sprintf(prompt, req.Config.Role, req.Config.Default, req.Message),
			"--ctx_size", "2048", "-b", "256", "--top_k", "10000",
			"--repeat_penalty", "1", "-t", strconv.Itoa(int(*threadcnt)),
		)
		buffer := bytes.NewBuffer(bufd[:0])
		cmd.Stdout = buffer
		err = cmd.Run()
		if err != nil {
			logrus.Infoln("get reply err:", err)
			continue
		}
		reply := buffer.String()
		i := strings.LastIndex(reply, "You(last line):")
		if i <= 0 {
			logrus.Infoln("get reply err: invalid prompt")
			continue
		}
		reply = strings.TrimSpace(reply[i+len("You(last line):"):])
		b = strings.Index(reply, "\n")
		if b > 0 {
			reply = strings.TrimSpace(reply[:b])
		}
		b = strings.Index(reply, `\n`)
		if b > 0 {
			reply = strings.TrimSpace(reply[:b])
		}
		logrus.Infoln("get reply:", reply)
		repl.IsPending = false
		repl.Msg = reply
		_, err = listener.WriteToUDP(tk.EncryptLittleEndian(repl.Pack(), sumtable), from)
		if err != nil {
			logrus.Infoln("send reply err:", err)
			continue
		}
	}
	logrus.Fatal(err)
}
