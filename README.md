With just main.go run below command

go mod init
go get
go build main.go


For **long messages** below approach can be taken 


    c := make(chan []*pdu.SubmitSM)

somewhere later in the program where you are processing 

    c <- submitSM.split()


    for {
        for _, p := range <-c {
            trans.Transceiver().Submit(p)
        }
    }



