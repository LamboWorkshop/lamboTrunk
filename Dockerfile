FROM golang:latest

WORKDIR /lamboTrunk

ADD . .

RUN make dep
RUN make

CMD ./build/lamboTrunk