package main

import (
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/linxGnu/gosmpp"
	"github.com/linxGnu/gosmpp/data"
	"github.com/linxGnu/gosmpp/pdu"

	"crypto/tls"
	"net"
)

var (
	// TLSDialer is tls connection dialer.
	TLSDialer = func(addr string) (net.Conn, error) {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		return tls.Dial("tcp", addr, conf)
	}

	re *regexp.Regexp
)

type SMPPConfig struct {
	host               string
	systemID           string
	password           string
	systemType         string
	enquirelink        time.Duration
	enquireLinkTimeout time.Duration
	ReadTimeout        time.Duration
}

type MessageConfig struct {
	sourceTON  byte
	sourceNPI  byte
	sourceAddr string
	destTON    byte
	destNPI    byte
	destAddr   string
	message    string
	esmclass   int
	encoding   int
}

var smppConfig SMPPConfig

func init() {

	pattern := `id:(\w+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) [Tt]ext:(.+)`

	re = regexp.MustCompile(pattern)

	smppConfig = SMPPConfig{"localhost:2775", "admin", "admin", "", 15 * time.Second, 10 * time.Second, 30 * time.Second}
}

func extract(message string) (map[string]string, error) {

	matches := re.FindStringSubmatch(message)

	var resultMap = make(map[string]string)

	if len(matches) > 0 {
		keys := []string{"id", "sub", "dlvrd", "submit_date", "done_date", "stat", "err", "text"}

		for i, key := range keys {
			resultMap[key] = matches[i+1]
		}

		return resultMap, nil
	}

	return nil, fmt.Errorf("invalid data length")

}

func main() {
	var wg sync.WaitGroup

	wg.Add(1)

	go sendingAndReceiveSMS(&wg, smppConfig)

	wg.Wait()
}

func sendingAndReceiveSMS(wg *sync.WaitGroup, smppConfig SMPPConfig) {
	defer wg.Done()

	auth := gosmpp.Auth{
		SMSC:       smppConfig.host,
		SystemID:   smppConfig.systemID,
		Password:   smppConfig.password,
		SystemType: smppConfig.systemType,
	}

	trans, err := gosmpp.NewSession(
		gosmpp.TRXConnector(gosmpp.NonTLSDialer, auth),
		//gosmpp.TRXConnector(TLSDialer, auth),
		gosmpp.Settings{
			EnquireLink: smppConfig.enquirelink,

			ReadTimeout: smppConfig.ReadTimeout,

			OnSubmitError: func(_ pdu.PDU, err error) {
				log.Fatal("SubmitPDU error:", err)
			},

			OnReceivingError: func(err error) {
				fmt.Println("Receiving PDU/Network error:", err)
			},

			OnRebindingError: func(err error) {
				fmt.Println("Rebinding but error:", err)
			},

			OnPDU: handlePDU(),

			OnClosed: func(state gosmpp.State) {
				fmt.Println(state)
			},
		}, 5*time.Second)

	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = trans.Close()
	}()

	c := make(chan []*pdu.SubmitSM, 1)

	messageConfig := MessageConfig{0, 0, "Zoro", 1, 1, "343434343434", "Hello", 0, 0}

	submitSM := newSubmitSM(messageConfig)

	// Split message if it's too long
	if submitSM.ShouldSplit() {

		// Array of SubmitSMs using Split() call
		submitSMs, _ := submitSM.Split()
		fmt.Println(submitSMs)
		for _, p := range submitSMs {
			fmt.Println(p.SequenceNumber)
		}

		c <- submitSMs

	} else {
		fmt.Println(submitSM.SequenceNumber)
		c <- []*pdu.SubmitSM{submitSM}
	}

	for _, p := range <-c {
		if err = trans.Transceiver().Submit(p); err != nil {
			fmt.Println(err)
		}
	}

	// Wait infinitely as we have receive handler
	// for delivery receipt
	select {}

}

func handlePDU() func(pdu.PDU, bool) {

	return func(p pdu.PDU, _ bool) {
		switch pd := p.(type) {
		case *pdu.SubmitSMResp:
			fmt.Printf("SubmitSMResp: %+v, %+v\n", pd.SequenceNumber, pd.MessageID)

		case *pdu.GenericNack:
			fmt.Println("GenericNack Received")

		case *pdu.EnquireLinkResp:
			fmt.Println("EnquireLinkResp Received")

		case *pdu.DataSM:
			fmt.Printf("DataSM:%+v\n", pd)

		case *pdu.DeliverSM:
			//fmt.Println("Printing PDU...", pd)
			message, _ := pd.Message.GetMessage()
			//fmt.Println("DLR", message)
			//fmt.Println(pd.OptionalParameters)
			m, _ := extract(message)
			fmt.Println("DLR", m["stat"])
		}
	}
}

func newSubmitSM(messageConfig MessageConfig) *pdu.SubmitSM {

	// build up submitSM

	srcAddr := pdu.NewAddress()
	srcAddr.SetTon(messageConfig.sourceTON)
	srcAddr.SetNpi(messageConfig.sourceNPI)
	_ = srcAddr.SetAddress(messageConfig.sourceAddr)

	destAddr := pdu.NewAddress()
	destAddr.SetTon(messageConfig.destTON)
	destAddr.SetNpi(messageConfig.destNPI)
	_ = destAddr.SetAddress(messageConfig.destAddr)

	submitSM := pdu.NewSubmitSM().(*pdu.SubmitSM)
	fmt.Println("SequenceNumber", submitSM.SequenceNumber)
	submitSM.SourceAddr = srcAddr
	submitSM.DestAddr = destAddr
	_ = submitSM.Message.SetMessageWithEncoding(messageConfig.message, data.UCS2)
	submitSM.ProtocolID = 0
	submitSM.RegisteredDelivery = 1
	submitSM.ReplaceIfPresentFlag = 0
	submitSM.EsmClass = 0

	return submitSM
}
