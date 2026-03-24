FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo docker)" -o /factory-pilot .

FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    curl \
    git \
    nodejs \
    npm

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && mv kubectl /usr/local/bin/

# Install Claude CLI
RUN npm install -g @anthropic-ai/claude-code

COPY --from=builder /factory-pilot /usr/local/bin/factory-pilot

RUN adduser -D -h /home/pilot pilot
USER pilot
WORKDIR /home/pilot

ENTRYPOINT ["factory-pilot"]
CMD ["run"]
