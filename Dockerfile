############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder

# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/deelfietsdashboard-importer
COPY . .

# Fetch dependencies.
# Using go get.
RUN go get -d -v
# Build the binary.
RUN CGO_ENABLED=0 go build -o /go/bin/deelfietsdashboard-importer

############################
# STEP 2 build a small image
############################
FROM alpine
# Copy our static executable.
COPY --from=builder /go/bin/deelfietsdashboard-importer /go/bin/deelfietsdashboard-importer

CMD ["/go/bin/deelfietsdashboard-importer"]