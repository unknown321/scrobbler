package device

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

var NVPModelNode = "/dev/icx_nvp/033"
var Capabilities = "/system/vendor/sony/etc/default-capability_w_internal.xml"
var USBFile = "/sys/devices/virtual/android_usb/android0/f_mass_storage/lun/file"

func GetModelNVP() (string, error) {
	res, err := os.ReadFile(NVPModelNode)
	if err != nil {
		return "", err
	}

	res = bytes.Trim(res, "\x00")

	return string(res), nil
}

type Devices struct {
	XMLName                   xml.Name `xml:"devices"`
	Text                      string   `xml:",chardata"`
	Xsi                       string   `xml:"xsi,attr"`
	NoNamespaceSchemaLocation string   `xml:"noNamespaceSchemaLocation,attr"`
	Version                   string   `xml:"version"`
	Device                    struct {
		Text           string `xml:",chardata"`
		Identification struct {
			Text            string `xml:",chardata"`
			Class           string `xml:"class"`
			Model           string `xml:"model"`
			Marketingname   string `xml:"marketingname"`
			Vendor          string `xml:"vendor"`
			Firmwareversion string `xml:"firmwareversion"`
		} `xml:"identification"`
	} `xml:"device"`
}

func GetModel() (*Devices, error) {
	var xmlFile *os.File
	var err error
	d := &Devices{}

	if xmlFile, err = os.Open(Capabilities); err != nil {
		return d, err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = makeCharsetReader

	if err = decoder.Decode(d); err != nil {
		return nil, fmt.Errorf("cannot unmarshal: %w", err)
	}

	return d, nil
}

func IsWalkmanOne() bool {
	s, err := os.Stat("/etc/.mod")
	if err != nil {
		return false
	}

	return s.IsDir()
}

func GetModelID() (string, error) {
	cmd := exec.Command("nvpflag", "-x", "mid")

	var out strings.Builder
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	r := strings.TrimRight(out.String(), "\n")

	return r, nil
}

func makeCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	if charset == "ISO-8859-1" {
		// Windows-1252 is a superset of ISO-8859-1, so should do here
		return charmap.Windows1252.NewDecoder().Reader(input), nil
	}
	return nil, fmt.Errorf("unknown charset: %s", charset)
}
