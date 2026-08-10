# Dev container for the SSR admin + backend. The frontend build is Go-only
# (esgun, github.com/oddship/esgun), so no node/npm/bun is needed.
FROM golang:1.26

ENV GOPATH /go
ENV CGO_ENABLED=0
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /app
CMD [ "sleep infinity" ]
