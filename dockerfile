# Use Golang to build the high-speed ingester
FROM golang:1.21-alpine AS builder
WORKDIR /app
# In a real setup, you'd COPY and 'go build' here. 
# For our demo, we'll use a pre-compiled binary or a script simulation.
RUN apk add --no-cache git
# Let's assume we've built our Go app called 'ingester'
ENTRYPOINT ["/bin/sh", "-c", "echo 'Ingester listening on UDP 5005...' && sleep 3600"]