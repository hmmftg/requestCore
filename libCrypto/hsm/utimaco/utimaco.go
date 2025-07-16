package utimaco

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hmmftg/requestCore/libCrypto"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

type Command struct {
	Key     string
	Command string
	Resp    string
	Channel chan string
}

type Utimaco struct {
	Socket        net.Conn
	RequestPool   sync.Map
	QueueMutex    *sync.Mutex
	SendQueue     chan Command
	EnableLogging bool         `json:"log"`
	SendStan      bool         `json:"sendStan"`
	Stan          atomic.Int64 `json:"stan"`
	Pvk           string       `json:"pvk"`
	Cvk           string       `json:"cvk"`
	Tpk           string       `json:"tpk"`
	IsSenderUp    bool
}

const (
	Info         = 1
	InfoGood     = 2
	Error        = 3
	ErrorWarning = 4
	ErrorBad     = 5
)

func (config *Utimaco) InitSenderThread() {
	config.QueueMutex.Lock()
	defer config.QueueMutex.Unlock()

	if config.SendQueue == nil {
		config.SendQueue = make(chan Command)
		go config.WaitForMessage()
	}
}

func (config *Utimaco) PurgeRequestsUntilReconnection() {
	log.Println("purging all crypto requests to hsm")
	for {
		req := <-config.SendQueue

		req.Channel <- "err: disconnected from hsm"

		if req.Command == "socket-ready" {
			return
		}
	}
}

func (config *Utimaco) Reconnect() {
	oldIPPort := config.Socket.RemoteAddr()
	var err, lastError error
	waitInterval := time.Second
	retryCount := 0
	maxRetyrInterval := time.Minute
	log.Println("reconnecting to hsm at", oldIPPort.String())
	go config.PurgeRequestsUntilReconnection()
	for {
		config.Socket, err = net.Dial(oldIPPort.Network(), oldIPPort.String())
		retryCount++
		if err != nil {
			if retryCount%10 == 0 && waitInterval < maxRetyrInterval && len(config.SendQueue) == 0 {
				waitInterval *= 5
			}
			if lastError == nil || lastError.Error() != err.Error() {
				log.Println("error reconnection to hsm at", oldIPPort.String(), err)
				lastError = err
			}
			time.Sleep(waitInterval)
		} else {
			log.Println("reconnection to hsm at", oldIPPort.String(), "success")
			config.SendQueue <- Command{
				Command: "socket-ready",
			}
			// wait for purge routine
			time.Sleep(time.Second)
			return
		}
	}
}

func (config *Utimaco) WaitForMessage() {
	for {
		req := <-config.SendQueue

		commandHex := req.Command
		commandByte, err := hex.DecodeString(commandHex)
		if err != nil {
			req.Channel <- fmt.Sprintf("err:Hex decode failed: %s,%+v", commandHex, err)
			continue
		}
		ln, err := config.Socket.Write(commandByte)
		if err != nil || ln != len(commandByte) {
			req.Channel <- fmt.Sprintf("err:Send failed: %+v", err)
			config.Reconnect()
			continue
		}
		log.Println(Info, fmt.Sprintf("Sent(stan: %v): %s", config.SendStan, strings.ToUpper(commandHex)))

		replyLen := make([]byte, 4)
		_, err = config.Socket.Read(replyLen)
		if err != nil {
			req.Channel <- fmt.Sprintf("err:Read error1: %+v", err)
			config.Reconnect()
			continue
		}
		length := (int(replyLen[1]) * 256 * 256) + (int(replyLen[2]) * 256) + int(replyLen[3])
		reply := make([]byte, length)
		_, err = config.Socket.Read(reply)
		if err != nil {
			req.Channel <- fmt.Sprintf("err:Read error3: %+v", err)
			config.Reconnect()
			continue
		}

		fullResp := strings.ToUpper(hex.EncodeToString(replyLen) + hex.EncodeToString(reply))

		req.Channel <- fullResp
	}
}

func (config *Utimaco) SendCommand(stan, command string) string {
	config.RequestPool.Store(stan, Command{
		Key:     stan,
		Command: command,
		Channel: make(chan string),
	})

	if !config.IsSenderUp {
		config.InitSenderThread()
	}

	resp := ""
	if value, ok := config.RequestPool.Load(stan); ok {
		cmd, ok := value.(Command)
		if !ok {
			log.Println("convert value to command failed")

			return ""
		}

		config.SendQueue <- cmd
		resp = <-cmd.Channel
		close(cmd.Channel)
		config.RequestPool.Delete(stan)
	} else {
		return ""
	}

	return resp
}

func (config *Utimaco) processCommand(commands []string) (string, error) {
	lenHex := fmt.Sprintf("%06X", (len(commands[1])+8)/2)

	var stanHex, commandHex string

	if config.SendStan {
		stanHex = fmt.Sprintf("%016X", config.Stan.Load())
		config.Stan.Add(1)
		lenHex = fmt.Sprintf("%06X", (len(commands[1])+8+16)/2)
		commandHex = commands[0] + lenHex + commands[1] + stanHex
	} else {
		commandHex = commands[0] + lenHex + commands[1]
	}

	fullResp := config.SendCommand(stanHex, commandHex)
	if strings.HasPrefix(fullResp, "err:") {
		return fullResp, response.ToErrorState(fmt.Errorf("error in SendCommand(%s)=>%s", commandHex, fullResp))
	}

	resp := fullResp

	if len(resp) > 8 {
		resp = resp[8:]
	}
	if config.SendStan && len(resp) > 24 {
		resp = resp[:len(resp)-24]
	}

	if !strings.HasPrefix(fullResp, "9A") {
		return "Failed" + fullResp, libError.New(
			http.StatusInternalServerError,
			"ERROR_IN_UTIMACO_RESP",
			errors.New(resp),
		)
	}
	log.Println(InfoGood, "Approved:"+fullResp)
	return resp, nil
}

func Init(ip, port, pvk, cvk, tpk string) (*Utimaco, error) {
	var err error
	socket, err := net.Dial("tcp4", ip+":"+port)
	return &Utimaco{
		Socket:        socket,
		Pvk:           pvk,
		Cvk:           cvk,
		Tpk:           tpk,
		SendStan:      true,
		EnableLogging: true,
		Stan:          atomic.Int64{},
		QueueMutex:    &sync.Mutex{},
	}, err
}

func (config *Utimaco) Pvv(pan, pinBlock string) (string, error) {
	commands := [2]string{"9C", ""}
	pvk := config.Pvk
	tpk := config.Tpk
	pvkLen := fmt.Sprintf("%04X", len(pvk)/2)
	tpkLen := fmt.Sprintf("%04X", len(tpk)/2)
	//////////////////////////////////////////////////////////
	commands[1] = "0195" + "1600" + pvkLen + pvk + pan[4:15] + "1" + tpkLen + tpk + pinBlock + pan[3:15]

	return config.processCommand(commands[:])
}

func (config *Utimaco) Cvv(pan, exp, cvvType string) (string, error) {
	commands := [2]string{"9C", ""}
	cvk := config.Cvk
	cvkLen := fmt.Sprintf("%04X", len(cvk)/2)
	service := "506"
	if cvvType == libCrypto.Cvv2 {
		service = "000"
		exp = exp[2:] + exp[:2]
	}
	//////////////////////////////////////////////////////////
	commands[1] = "0195" + "1500" + cvkLen + cvk + exp + service + "010" + pan

	resp, err := config.processCommand(commands[:])
	return resp[:3], err
}

func (config *Utimaco) Offset(pan, pinBlock string) (string, error) {
	commands := [2]string{"9C", ""}
	pvk := config.Pvk
	tpk := config.Tpk
	pvkLen := fmt.Sprintf("%04X", len(pvk)/2)
	tpkLen := fmt.Sprintf("%04X", len(tpk)/2)
	//////////////////////////////////////////////////////////
	commands[1] = "0195" + "1C00" + pvkLen + pvk + "08" + pan + "30313233343536373839303132333435" + pinBlock + "00" + pan[3:15] + tpkLen + tpk + "06"

	resp, err := config.processCommand(commands[:])
	resp = strings.Replace(resp, "F", "", -1)
	return resp, err
}

func (config *Utimaco) SetKey(id, value string) {
	switch id {
	case "Cvk":
		config.Cvk = value
	case "Pvk":
		config.Pvk = value
	case "Tpk":
		config.Tpk = value
	}
}
func (config *Utimaco) GetKey(id string) string {
	switch id {
	case "Cvk":
		return config.Cvk
	case "Pvk":
		return config.Pvk
	case "Tpk":
		return config.Tpk
	}
	return ""
}

func (config *Utimaco) Mac(data string) (string, error) {
	return "0000000000000000", nil
}
func (config *Utimaco) Cvv2Padding(data string) (string, error) {
	return "0000000000000000", nil
}
func (config *Utimaco) Translate(pan, pinBlock, tpk2nd string) (string, error) {
	return pinBlock, nil
}
func (config *Utimaco) Crypt(data, mode string) (string, error) {
	return data, nil
}
