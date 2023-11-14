FROM golang:1.20-bullseye as builder

WORKDIR /app

COPY go.mod .
COPY Makefile .
COPY go.sum .
RUN go mod download
RUN make prepare

COPY . .
RUN make build

FROM debian:bullseye as final

WORKDIR /app
COPY --from=builder /app/bin/axiom-ledger .

ENTRYPOINT ["./axiom-ledger"]