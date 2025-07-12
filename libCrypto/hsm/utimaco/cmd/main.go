package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
)

func validateHex(input string) error {
	_, err := hex.DecodeString(input)
	if err != nil {
		return errors.New("invalid Hex")
	}
	return nil
}

func validateNumber(input string) error {
	_, err := strconv.Atoi(input)
	if err != nil {
		return errors.New("invalid Number")
	}
	return nil
}

func ProcessCommand(commands []string, config Config) (string, error) {
	lenHex := fmt.Sprintf("%06X", (len(commands[1])+8)/2)

	var commandHex string

	if config.Sendstan {
		stanHex := fmt.Sprintf("%016X", config.Stan)
		config.Stan++
		lenHex = fmt.Sprintf("%06X", (len(commands[1])+8+16)/2)
		commandHex = commands[0] + lenHex + commands[1] + stanHex
	} else {
		commandHex = commands[0] + lenHex + commands[1]
	}

	commandByte, err := hex.DecodeString(commandHex)
	if err != nil {
		pterm.Error.Printf("Hex decode failed %v: %s\n", err, commandHex)
		return "", err
	}
	ln, err := config.Socket.Write(commandByte)
	if err != nil || ln != len(commandByte) {
		pterm.Error.Printf("Send failed %v\n", err)
		return "", err
	}
	if config.EnableLogging {
		pterm.Info.Printf("Sent(stan: %v): %s\n", config.Sendstan, strings.ToUpper(commandHex))
	}

	replyLen := make([]byte, 4)
	_, err = config.Socket.Read(replyLen)
	if err != nil {
		pterm.Error.Printf("Read error1: %s\n", err)
		return "", err
	}
	length := (int(replyLen[1]) * 256 * 256) + (int(replyLen[2]) * 256) + int(replyLen[3])
	if err != nil {
		pterm.Error.Printf("Read error2: %s\n", err)
		return "", err
	}
	reply := make([]byte, length)
	_, err = config.Socket.Read(reply)
	if err != nil {
		pterm.Error.Printf("Read error3: %s\n", err)
		return "", err
	}
	//log.Printf("Read %q (%d bytes)\n", string(reply[:n]), n)

	resp := strings.ToUpper(hex.EncodeToString(replyLen[:4]) + hex.EncodeToString(reply))
	fullResp := strings.ToUpper(hex.EncodeToString(replyLen) + hex.EncodeToString(reply))

	if len(resp) > 8 {
		resp = resp[8:]
	}
	if config.Sendstan && len(resp) > 24 {
		resp = resp[:len(resp)-24]
	}

	if replyLen[0] == 0x9A {
		if config.EnableLogging {
			pterm.Info.Println(promptui.IconGood+" Approved:", fullResp)
		}
		return resp, nil
	} else {
		if config.EnableLogging {
			pterm.Error.Println(promptui.IconWarn+" Failed: ", fullResp)
		}
		return promptui.IconBad, errors.New(promptui.IconBad + "  " + resp)
	}
}

func ImportKey(config Config) error {
	commands := [2]string{"9C", ""}
	//////////////////////////////////////////////
	prompt := promptui.Prompt{
		Label:    "Key Value",
		Validate: validateHex,
		Default:  config.Keyplain,
	}
	key, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	keyLen := fmt.Sprintf("%02X", len(key)/2)
	//////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:   "Key Name",
		Default: "Key1C1C0",
	}
	keyName, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Name %v\n", err)
		return err
	}
	keyNameHex := hex.EncodeToString([]byte(keyName))
	//////////////////////////////////////////////

	commands[1] = "0195" + "1100" + "31" + keyNameHex + "06" + keyLen + key + "00000000000000000000000000000000" + "0000"

	_, err = ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	token, err := Tokenize(config, keyNameHex)

	if err != nil {
		pterm.Error.Printf("Error Tokenize %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Token %s\n", promptui.IconGood, token)

	return nil
}

func ImportCsd(config Config) error {
	commands := [2]string{"9C", ""}
	//////////////////////////////////////////////
	prompt := promptui.Prompt{
		Label:    "Key Value",
		Validate: validateHex,
		Default:  config.Keyplain,
	}
	key, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	keyLen := fmt.Sprintf("%02X", len(key)/2)
	//////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:   "Key Name",
		Default: "CSD",
	}
	keyName, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Name %v\n", err)
		return err
	}
	//////////////////////////////////////////////

	keyNameHex := hex.EncodeToString([]byte(keyName + "-ENCR"))
	commands[1] = "0195" + "1100" + "31" + keyNameHex + "06" + keyLen + key + "00000000000000000000000000000000" + "0000" + "60"

	_, err = ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	token, err := Tokenize(config, keyNameHex)

	if err != nil {
		pterm.Error.Printf("Error Tokenize %v\n", err)
		return err
	}

	pterm.Success.Printf("Token Encryption %s\n", token)
	keyNameHex = hex.EncodeToString([]byte(keyName + "-DECR"))
	commands[1] = "0195" + "1100" + "31" + keyNameHex + "06" + keyLen + key + "00000000000000000000000000000000" + "0000" + "61"

	_, err = ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	token, err = Tokenize(config, keyNameHex)

	if err != nil {
		pterm.Error.Printf("Error Tokenize %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Token Decryption %s\n", promptui.IconGood, token)

	return nil
}

func Pvv(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Pvk Value",
		Validate: validateHex,
		Default:  config.Pvk,
	}
	pvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	pvkLen := fmt.Sprintf("%04X", len(pvk)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Tpk Value",
		Validate: validateHex,
		Default:  config.Tpk,
	}
	tpk, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	tpkLen := fmt.Sprintf("%04X", len(tpk)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Pan Value",
		Validate: validateNumber,
		Default:  config.Pan,
	}
	pan, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pan %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "PinBlock",
		Validate: validateHex,
		Default:  config.PinBlock,
	}
	pinBlock, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong PinBlock %v\n", err)
		return err
	}

	commands[1] = "0195" + "1600" + pvkLen + pvk + pan[4:15] + "1" + tpkLen + tpk + pinBlock + pan[3:15]

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s PVV %v\n", promptui.IconGood, resp)

	return nil
}

func Offset(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Pvk Value",
		Validate: validateHex,
		Default:  config.Pvk,
	}
	pvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	pvkLen := fmt.Sprintf("%04X", len(pvk)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Tpk Value",
		Validate: validateHex,
		Default:  config.Tpk,
	}
	tpk, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	tpkLen := fmt.Sprintf("%04X", len(tpk)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Pan Value",
		Validate: validateNumber,
		Default:  config.Pan,
	}
	pan, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pan %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "PinBlock",
		Validate: validateHex,
		Default:  config.Pin2Block,
	}
	pinBlock, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong PinBlock %v\n", err)
		return err
	}

	commands[1] = "0195" + "1C00" + pvkLen + pvk + "08" + pan + "30313233343536373839303132333435" + pinBlock + "00" + pan[3:15] + tpkLen + tpk + "06"

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Offset %v\n", promptui.IconGood, resp)

	return nil
}

func Translate(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Source Key",
		Validate: validateHex,
		Default:  config.Tpk,
	}
	sourceKey, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	sourceKeyLen := fmt.Sprintf("%04X", len(sourceKey)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Dest Key",
		Validate: validateHex,
		Default:  config.Tpk,
	}
	destKey, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	destKeyLen := fmt.Sprintf("%04X", len(destKey)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Pan Value",
		Validate: validateNumber,
		Default:  config.Pan,
	}
	pan, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pan %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "PinBlock",
		Validate: validateHex,
		Default:  config.PinBlock,
	}
	pinBlock, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong PinBlock %v\n", err)
		return err
	}

	commands[1] = "0195" + "1F00" + sourceKeyLen + sourceKey + "00" + "00" + pinBlock + "00" + pan[3:15] + destKeyLen + destKey + "00" + "00" + "00" + pan[3:15]

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Translated PinBlock %v\n", promptui.IconGood, resp)

	return nil
}

func Mac(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Tak Value",
		Validate: validateHex,
		Default:  config.Tak,
	}
	tak, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	takLen := fmt.Sprintf("%04X", len(tak)/2)
	//////////////////////////////////////////////////////////
	macSelect := promptui.Select{
		Label: "Select Mac Type",
		Items: []string{"X9.19", "X9.9", "CBC"},
		Size:  3,
	}
	_, mac, err := macSelect.Run()
	if err != nil {
		pterm.Error.Printf("Select Error %v\n", err)
		return err
	}
	macMode := ""
	switch mac {
	case "X9.19":
		macMode = "07"
	case "X9.9":
		macMode = "07"
	case "CBC":
		macMode = "03"
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Data",
		Validate: validateNumber,
		Default:  "0000000000000000",
	}
	data, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Data %v\n", err)
		return err
	}
	dataLen := fmt.Sprintf("%04X", len(data)/2)
	//////////////////////////////////////////////////////////

	commands[1] = "0195" + "1900" + takLen + tak + "0000" + "01" + macMode + dataLen + data

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Mac %v\n", promptui.IconGood, resp)

	return nil
}

func Decrypt(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Csd",
		Validate: validateHex,
		Default:  config.CsdDecrypt,
	}
	csd, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	csdLen := fmt.Sprintf("%04X", len(csd)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Cipher",
		Validate: validateHex,
		Default:  "0000000000000000",
	}
	data, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Cipher %v\n", err)
		return err
	}
	dataLen := fmt.Sprintf("%04X", len(data)/2)
	//////////////////////////////////////////////////////////

	commands[1] = "0195" + "2800" + csdLen + csd + "00000" + "000" + "00000000" + dataLen + data + "00"

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Data %v\n", promptui.IconGood, resp[16:])

	return nil
}

func Encrypt(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Csd",
		Validate: validateHex,
		Default:  config.CsdEncrypt,
	}
	csd, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	csdLen := fmt.Sprintf("%04X", len(csd)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Data",
		Validate: validateNumber,
		Default:  "0000000000000000",
	}
	data, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Data %v\n", err)
		return err
	}
	dataLen := fmt.Sprintf("%04X", len(data)/2)
	//////////////////////////////////////////////////////////

	commands[1] = "0195" + "2800" + csdLen + csd + "00000" + "001" + "00000000" + dataLen + data + "00"

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Cipher %v\n", promptui.IconGood, resp[16:])

	return nil
}

func Cvv(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Cvk Value",
		Validate: validateHex,
		Default:  config.Cvk,
	}
	cvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	cvkLen := fmt.Sprintf("%04X", len(cvk)/2)
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Pan Value",
		Validate: validateNumber,
		Default:  config.Pan,
	}
	pan, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pan %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Expiry Date",
		Validate: validateNumber,
		Default:  config.Expiry,
	}
	exp, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Expiry Date %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	cvvSelect := promptui.Select{
		Label: "Select CVV Type",
		Items: []string{"Cvv1", "Cvv2"},
		Size:  3,
	}
	_, cvvType, err := cvvSelect.Run()
	if err != nil {
		pterm.Error.Printf("Select Error %v\n", err)
		return err
	}
	service := "506"
	if cvvType == "Cvv2" {
		service = "000"
		exp = exp[2:] + exp[:2]
	}
	//////////////////////////////////////////////////////////

	commands[1] = "0195" + "1500" + cvkLen + cvk + exp + service + "010" + pan

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s %s %v\n", promptui.IconGood, cvvType, resp[:3])

	return nil
}

func Tokenize(config Config, keyNameHex string) (string, error) {
	commands := [2]string{"9C", ""}
	commands[1] = "0195" + "0200" + keyNameHex

	resp, err := ProcessCommand(commands[:], config)
	if err != nil {
		return "", err
	}

	return resp, nil
}

func Kcv(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Token",
		Validate: validateHex,
		Default:  config.Token,
	}
	token, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	keyLen := fmt.Sprintf("%04X", len(token)/2)

	commands[1] = "0195" + "2300" + keyLen + token

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("KCV Full %v\n", resp)

	pterm.Success.Printf("%s KCV %v\n", promptui.IconGood, resp[32:38])

	return nil
}

func ImportUnderMaster(config Config) error {
	commands := [2]string{"9C", ""}
	prompt := promptui.Prompt{
		Label:    "Token",
		Validate: validateHex,
		Default:  config.Token,
	}
	token, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	keyLen := fmt.Sprintf("%04X", len(token)/2)
	prompt = promptui.Prompt{
		Label:    "KUK",
		Validate: validateHex,
		Default:  config.Kuk,
	}
	kuk, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	kuklen := fmt.Sprintf("%02X", len(kuk)/2)

	commands[1] = "0195" + "0400" + keyLen + token + "19" + kuklen + kuk + "00"

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}
	pterm.Success.Printf("%s Token %v\n", promptui.IconGood, resp[16:len(resp)-16])

	return nil
}

func Custom(config Config) error {
	commands := [2]string{"9C", ""}

	prompt := promptui.Prompt{
		Label:    "Command",
		Validate: validateHex,
	}
	cmd, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Hex %v\n", err)
		return err
	}

	commands[1] = "0195" + cmd

	resp, err := ProcessCommand(commands[:], config)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Resp Hex %v\n", promptui.IconGood, resp)

	return nil
}

type Config struct {
	Ip            string `json:"ip"`
	Port          string `json:"port"`
	Socket        net.Conn
	EnableLogging bool   `json:"log"`
	Sendstan      bool   `json:"sendstan"`
	Stan          int    `json:"stan"`
	Keyplain      string `json:"keyplain"`
	Pan           string `json:"pan"`
	PinBlock      string `json:"pinblock"`
	Pin2Block     string `json:"pin2block"`
	Token         string `json:"token"`
	Expiry        string `json:"expiry"`
	Pvk           string `json:"pvk"`
	Cvk           string `json:"cvk"`
	Tpk           string `json:"tpk"`
	Tak           string `json:"tak"`
	CsdEncrypt    string `json:"csd1"`
	CsdDecrypt    string `json:"csd2"`
	Kuk           string `json:"kuk"`
}

func main() {
	configFile := flag.String("c", "utimaco.json", "Config File")
	flag.Parse()

	var config Config

	pterm.Info.Println("Reading", os.Getenv("CONFIG")+"/"+*configFile)

	fileData, err := os.ReadFile(os.Getenv("CONFIG") + "/" + *configFile)
	if err != nil {
		pterm.Error.Printf("Config File[%s] Read Error %v\n", fileData, err)
		return
	}
	err = json.Unmarshal(fileData, &config)
	if err != nil {
		pterm.Error.Printf("Config File[%s] Format Error %v\n", fileData, err)
		return
	}

	if config.EnableLogging {
		pterm.Info.Println("IP", config.Ip, "Port", config.Port, "Stan", config.Sendstan)
	}

	config.Socket, err = net.Dial("tcp4", config.Ip+":"+config.Port)
	if err != nil {
		pterm.Error.Printf("Connect failed %v\n", err)
		return
	}

	listMenu := []string{"Import", "ImportCsd", "Kcv", "ImportUnderMaster", "Pvv", "Mac", "Cvv", "Offset", "Translate", "Encrypt", "Decrypt", "Custom", "Quit"}

	prompt := promptui.Select{
		Label: "Select Operation",
		Items: listMenu,
		Searcher: func(input string, idx int) bool {
			member := strings.ToUpper(listMenu[idx])
			return strings.Contains(member, strings.ToUpper(input))
		},
		Size:              5,
		StartInSearchMode: true,
	}

	for i := 0; ; i++ {
		_, choice, err := prompt.Run()
		if err != nil {
			pterm.Error.Printf("Prompt failed %v\n", err)
			return
		}

		//pterm.Error.Printf("You choose %q\n", choice)

		switch choice {
		case "Quit":
			config.Socket.Close()
			return
		case "Import":
			err = ImportKey(config)
		case "ImportCsd":
			err = ImportCsd(config)
		case "Kcv":
			err = Kcv(config)
		case "ImportUnderMaster":
			err = ImportUnderMaster(config)
		case "Pvv":
			err = Pvv(config)
		case "Mac":
			err = Mac(config)
		case "Cvv":
			err = Cvv(config)
		case "Offset":
			err = Offset(config)
		case "Translate":
			err = Translate(config)
		case "Encrypt":
			err = Encrypt(config)
		case "Decrypt":
			err = Decrypt(config)
		case "Custom":
			err = Custom(config)
		}
		if err != nil {
			pterm.Error.Printf("error: %v\n", err)
			return
		}
	}
}
