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
	"time"

	"github.com/sirupsen/logrus"

	para "github.com/fumiama/go-hide-param"
	tea "github.com/fumiama/gofastTEA"

	"github.com/fumiama/alpacapi"
)

const endmark = `

### Response:

`

// prompt: role, default, message
const prompt = `You are a Chinese %s and got messages with foramt "User(Name): Content". Reply with one sentence in Chinese. If you don't know how to reply, just say "%s".

### Instruction:

%s` + endmark

func main() {
	addr := flag.String("l", "0.0.0.0:31471", "listening endpoint")
	mpth := flag.String("m", "/dataset/Alpaca/ggml/13B-ggml-model-q4_1.bin", "alpaca ggml model path")
	threadcnt := flag.Uint("t", 24, "use threads count")
	llamapath := flag.String("p", "./src/llama.cpp/main", "llama.cpp main path")
	bufsz := flag.Uint("b", 4096, "udp buffer size")
	sumtablepath := flag.String("s", "sumtable.bin", "tea sumtable file")
	flag.Parse()
	if len(flag.Args()) < 1 {
		panic("must give tea key (16 bytes hex string)")
	}
	kstr := flag.Args()[0]
	k, err := hex.DecodeString(kstr)
	if err != nil {
		panic(err)
	}
	for i, a := range os.Args {
		if a == kstr {
			para.Hide(i)
		}
	}
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
			ID:        uint32(req.ID),
			IsPending: true,
			Msg:       "pending...",
		}
		go func() {
			data := tk.EncryptLittleEndian(repl.Pack(), sumtable)
			for i := 0; i < 8; i++ {
				_, _ = listener.WriteToUDP(data, from)
				time.Sleep(time.Second * 4)
			}
		}()
		cmd := exec.Command(
			*llamapath, "-m", *mpth, "-p",
			fmt.Sprintf(prompt, req.Config.Role, req.Config.Default, req.Message),
			"--ctx_size", "2048", "-b", "512", "--top_k", "10000",
			"-t", strconv.Itoa(int(*threadcnt)),
		)
		buffer := bytes.NewBuffer(bufd[:0])
		cmd.Stdout = buffer
		// cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			logrus.Infoln("get reply err:", err)
			continue
		}
		reply := buffer.String()
		i := strings.LastIndex(reply, endmark)
		if i <= 0 {
			logrus.Infoln("get reply err: invalid prompt")
			continue
		}
		reply = strings.TrimSpace(reply[i+len(endmark):])
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
		data = tk.EncryptLittleEndian(repl.Pack(), sumtable)
		go func() {
			for i := 0; i < 8; i++ {
				_, _ = listener.WriteToUDP(data, from)
				time.Sleep(time.Second)
			}
		}()
	}
	logrus.Fatal(err)
}
