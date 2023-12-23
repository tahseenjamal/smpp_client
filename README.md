With just main.go run below command

    go mod init 
>
    go mod tidy
>
    go get 
>
    go build main.go


As this program uses a low level library, it needs to implement a mutex to create blocking submission to ensure it can create relationship between submitted message and the message id received in acknowledgement

