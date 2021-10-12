FROM golang:latest
ENV GO111MODULE=auto
WORKDIR /go/src/
COPY . .
RUN go get github.com/gorilla/mux
RUN go get github.com/joho/godotenv
CMD ["go", "run", "esr_riders.go"]
EXPOSE 5421