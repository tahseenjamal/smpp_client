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

func init() {

	pattern := `id:(\w+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) [Tt]ext:(.+)`

	re = regexp.MustCompile(pattern)
}

func extract(message string) (map[string]string, error) {

	matches := re.FindStringSubmatch(message)

	var resultMap = make(map[string]string)

	if len(matches) > 0 {
		keys := []string{"id", "sub", "dlvrd", "submit_date", "done_date", "stat", "err", "text", "Text"}

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
	go sendingAndReceiveSMS(&wg)

	wg.Wait()
}

func sendingAndReceiveSMS(wg *sync.WaitGroup) {
	defer wg.Done()

	auth := gosmpp.Auth{
		SMSC:       "smscsim.smpp.org:2775",
		SystemID:   "SYSTEMID",
		Password:   "PASSWORD",
		SystemType: "",
	}

	trans, err := gosmpp.NewSession(
		gosmpp.TRXConnector(gosmpp.NonTLSDialer, auth),
		//gosmpp.TRXConnector(TLSDialer, auth),
		gosmpp.Settings{
			EnquireLink: 5 * time.Second,

			ReadTimeout: 10 * time.Second,

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

	// sending SMS(s)
	for i := 0; i < 1; i++ {
		if err = trans.Transceiver().Submit(newSubmitSM()); err != nil {
			fmt.Println(err)
		}
		time.Sleep(5 * time.Second)
	}
}

func handlePDU() func(pdu.PDU, bool) {
	//concatenated := map[uint8][]string{}
	return func(p pdu.PDU, _ bool) {
		switch pd := p.(type) {
		case *pdu.SubmitSMResp:
			fmt.Printf("SubmitSMResp:%+v\n", pd.MessageID)
			fmt.Println(pd.CommandStatus)

		case *pdu.GenericNack:
			fmt.Println("GenericNack Received")

		case *pdu.EnquireLinkResp:
			fmt.Println("EnquireLinkResp Received")

		case *pdu.DataSM:
			fmt.Printf("DataSM:%+v\n", pd)

		case *pdu.DeliverSM:
			fmt.Println("Printing PDU...")
			message, _ := pd.Message.GetMessage()
			fmt.Println("Print tag parameters")
			fmt.Println(pd.OptionalParameters)
			m, _ := extract(message)
			fmt.Println(m["stat"])
		}
	}
}

func newSubmitSM() *pdu.SubmitSM {
	// build up submitSM
	srcAddr := pdu.NewAddress()
	srcAddr.SetTon(5)
	srcAddr.SetNpi(0)
	_ = srcAddr.SetAddress("MelroseLabs")

	destAddr := pdu.NewAddress()
	destAddr.SetTon(1)
	destAddr.SetNpi(1)
	_ = destAddr.SetAddress("447712345678")

	submitSM := pdu.NewSubmitSM().(*pdu.SubmitSM)
	submitSM.SourceAddr = srcAddr
	submitSM.DestAddr = destAddr
	_ = submitSM.Message.SetMessageWithEncoding("Hello World ", data.UCS2)
	submitSM.ProtocolID = 0
	submitSM.RegisteredDelivery = 1
	submitSM.ReplaceIfPresentFlag = 0
	submitSM.EsmClass = 0

	fmt.Println("Just after submitting")
	fmt.Println(submitSM.CommandStatus)

	return submitSM
}
