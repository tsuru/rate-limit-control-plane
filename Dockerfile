ARG alpine_version=3.19
ARG golang_version=1.22
FROM golang:${golang_version}-alpine${alpine_version} as builder
ARG TARGETARCH
ENV GOARCH=$TARGETARCH
RUN apk update && apk add make

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . /app

RUN CGO_ENABLED=0 make build

FROM alpine:${alpine_version}
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app .

CMD ["./rate-limit-control-plane"]

EXPOSE 3000