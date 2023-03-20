package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/tarm/serial"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/platforms/raspi"
)

const (
	cmdCfgReg      byte = 0xC0
	cmdReadReg          = 0xC1
	cmdCfgTmpReg        = 0xC2
	cmdWirelessCfg      = 0xCF
)

type command struct {
	head          byte
	beginRegister byte
	length        byte
	data          []byte
}

func (cmd command) bytes() []byte {
	b := make([]byte, 0, len(cmd.data)+3)
	b = append(b, cmd.head, cmd.beginRegister, cmd.length)
	b = append(b, cmd.data...)
	return b
}

func (cmd command) exec(rw io.ReadWriter) (*answer, error) {
	var ans answer
	var buf [3+9]byte
	var length *byte = &buf[2]
	var sofar, stop int

	_, err := rw.Write(cmd.bytes())
	if err != nil {
		return nil, err
	}

	for {
		_, err = rw.Read(buf[sofar:sofar+1])
		if err != nil {
			return nil, err
		}
		if *length == 0xFF {
			return &answerError, errors.New("Answer error")
		}
		if *length > 9 {
			return nil, fmt.Errorf("Bad length 0x%02X", length)
		}
		if *length > 0 && stop == 0 {
			stop = sofar + int(*length)
		}
		if stop > 0 && stop == sofar {
			break
		}
		sofar++
	}

	err = ans.from(buf[:sofar+1])

	return &ans, err
}

type answer struct {
	head          byte
	beginRegister byte
	length        byte
	data          []byte
}

func (ans *answer) bytes() []byte {
	b := make([]byte, 0, len(ans.data)+3)
	b = append(b, ans.head, ans.beginRegister, ans.length)
	b = append(b, ans.data...)
	return b
}

func (ans *answer) from(data []byte) error {
	if len(data) < 3 {
		return errors.New("insufficient data for answer header")
	}
	ans.head = data[0]
	ans.beginRegister = data[1]
	ans.length = data[2]
	if len(data) < 3+int(ans.length) {
		return errors.New("insufficient data for answer payload")
	}
	ans.data = make([]byte, ans.length)
	copy(ans.data, data[3:3+ans.length])
	return nil
}

func (ans *answer) dump() string {
	var s string
	fmt.Println(hex.Dump(ans.bytes()))
	for i := 0; i < ans.length; i++ {
		switch i {
		case 0:
			s = append(s, ans.data[i])
		}
	}
}

var answerError = answer{0xFF, 0xFF, 0xFF, nil}

const (
	regADDH byte = iota
	regADDL
	regNETID
	regREG0
	regREG1
	regREG2
	regREG3
	regCRYPT_H
	regCRYPT_L
	regPID = 0x80
)

func main() {
	adaptor := raspi.NewAdaptor()
	adaptor.Connect()

	M0 := gpio.NewDirectPinDriver(adaptor, "15") // GPIO 22
	M1 := gpio.NewDirectPinDriver(adaptor, "13") // GPIO 27

	println("opening")
	c := &serial.Config{Name: "/dev/ttyS0", Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	println("open OK")

	M0.Off() // low
	M1.On()  // high
	time.Sleep(100 * time.Millisecond)

	cmd := command{cmdReadReg, regADDH, 0x09, nil}
	fmt.Println(cmd.dump())

	ans, err := cmd.exec(s)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ans.dump())

	s.Close()
	M1.Off() // low
}
