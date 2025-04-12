package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hmmftg/requestCore/libCrypto/ssm"
	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
)

func validateHex(input string) error {
	_, err := hex.DecodeString(input)
	if err != nil {
		return fmt.Errorf("invalid Hex")
	}
	return nil
}

func validatBase64(input string) error {
	_, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return fmt.Errorf("invalid Base64")
	}
	return nil
}

func validateNumber(input string) error {
	_, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("invalid Number")
	}
	return nil
}

func Pvv(config *Config) error {
	prompt := promptui.Prompt{
		Label:    "Pvk Value",
		Validate: validatBase64,
		Default:  config.Pvk,
	}
	pvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
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
		Label:    "Pin 1 Value",
		Validate: validateNumber,
		Default:  config.Pin1,
	}
	pin1, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pin 1 %v\n", err)
		return err
	}

	pvv, err := ssm.GeneratePvv(pan, pvk, "1", pin1)

	if err != nil {
		pterm.Error.Printf("Error: %v\n", err)
		return err
	}

	pterm.Success.Printf("%s PVV %v\n", promptui.IconGood, pvv)

	return nil
}

func PinBlock(config *Config) error {
	prompt := promptui.Prompt{
		Label:    "Pvk Value",
		Validate: validatBase64,
		Default:  config.Pvk,
	}
	pvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
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
		Label:    "Pin Value",
		Validate: validateNumber,
		Default:  "1234",
	}
	pin, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pin %v\n", err)
		return err
	}

	pinBlock, err := ssm.GeneratePinBlock(pan, pvk, pin)

	if err != nil {
		pterm.Error.Printf("Error: %v\n", err)
		return err
	}

	pterm.Success.Printf("%s pinBlock %v\n", promptui.IconGood, pinBlock)

	return nil
}

func Offset(config *Config) error {
	prompt := promptui.Prompt{
		Label:    "Pvk Value",
		Validate: validatBase64,
		Default:  config.Pvk,
	}
	pvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
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
		Label:    "Pin 2 Value",
		Validate: validateNumber,
		Default:  config.Pin2,
	}
	pin2, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Pin 2 %v\n", err)
		return err
	}

	offset, err := ssm.GenerateOffset(pan, pvk, pin2, len(pin2))

	if err != nil {
		pterm.Error.Printf("Error: %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Offset %v\n", promptui.IconGood, offset)

	return nil
}

func Cvv(config *Config) error {
	prompt := promptui.Prompt{
		Label:    "Cvk Value",
		Validate: validatBase64,
		Default:  config.Cvk,
	}
	cvk, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong CVK Key %v\n", err)
		return err
	}
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

	cvv, err := ssm.GenCvv(pan, exp, service, cvk)

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s %s %v\n", promptui.IconGood, cvvType, cvv)

	return nil
}

func Encrypt(config *Config) error {
	//////////////////////////////////////////////////////////
	opTypeSelect := promptui.Select{
		Label: "Select Operation Type",
		Items: []string{"Encrypt", "Decrypt"},
		Size:  3,
	}
	_, opType, err := opTypeSelect.Run()
	if err != nil {
		pterm.Error.Printf("Select Error %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt := promptui.Prompt{
		Label:    "Key",
		Validate: validatBase64,
		Default:  config.Keyplain,
	}
	key, err := prompt.Run()

	if err != nil {
		pterm.Error.Printf("Wrong Key %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	prompt = promptui.Prompt{
		Label:    "Data",
		Validate: validateHex,
		Default:  "0000000000000000",
	}
	data, err := prompt.Run()
	if err != nil {
		pterm.Error.Printf("Wrong Data %v\n", err)
		return err
	}
	//////////////////////////////////////////////////////////
	var resp string
	if opType == "Encrypt" {
		resp, err = ssm.EncryptDesHex(data, key)
	} else {
		resp, err = ssm.DecryptDesHex(data, key)
	}

	if err != nil {
		pterm.Error.Printf("Error Command %v\n", err)
		return err
	}

	pterm.Success.Printf("%s Result %v\n", promptui.IconGood, resp)

	return nil
}

type Config struct {
	EnableLogging bool   `json:"log"`
	Keyplain      string `json:"keyplain"`
	Pan           string `json:"pan"`
	Expiry        string `json:"expiry"`
	Pin1          string `json:"pin1"`
	Pin2          string `json:"pin2"`
	Pvk           string `json:"pvk"`
	Cvk           string `json:"cvk"`
}

func main() {
	configFile := flag.String("c", "crypto.json", "Config File")
	convert := flag.String("v", "", "Convert Hex To Base64")
	flag.Parse()

	if len(*convert) > 0 {
		b, err := hex.DecodeString(*convert)
		if err != nil {
			log.Fatal(err.Error())
		}
		b64 := base64.StdEncoding.EncodeToString(b)
		log.Println(b64)
		return
	}

	var config Config

	pterm.Info.Println("Reading", os.Getenv("CONFIG")+"/"+*configFile)

	fileData, err := os.ReadFile(os.Getenv("CONFIG") + "/" + *configFile)
	if err != nil {
		pterm.Error.Printf("Read config file:\n {"+os.Getenv("CONFIG")+"/"+*configFile+"}\n failed: %v\n", err)
		return
	}
	err = json.Unmarshal(fileData, &config)
	if err != nil {
		pterm.Error.Printf("Config parse error: %v\n", err)
		return
	}

	listmenu := []string{"Crypto", "Pvv", "Cvv", "Offset", "Pinblock", "Quit"}

	prompt := promptui.Select{
		Label: "Select Operation",
		Items: listmenu,
		Searcher: func(input string, idx int) bool {
			member := strings.ToUpper(listmenu[idx])
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
			return
		case "Crypto":
			err = Encrypt(&config)
		case "Pvv":
			err = Pvv(&config)
		case "Cvv":
			err = Cvv(&config)
		case "Offset":
			err = Offset(&config)
		case "Pinblock":
			err = PinBlock(&config)
		}
		if err != nil {
			pterm.Error.Printf("Prompt failed %v\n", err)
			return
		}
	}
}
