FROM golang:latest

WORKDIR /lamboTrunk

ADD . .

CMD ./build/lamboTrunk
